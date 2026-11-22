package repository

import (
	"errors"
	"sort"
	"strings"
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MessageRepository interface {
	GetTotalUnreadCount(userID uint64) (int64, error)
	ListConversations(userID uint64) ([]model.UserConversation, error)
	FindOrCreateConversation(user1ID, user2ID uint64) (*model.UserConversation, error)
	FindConversationByID(convID uint64) (*model.UserConversation, error)
	ListMessages(convID uint64, beforeID uint64, limit int) ([]model.UserMessage, bool, error)
	CreateMessage(conv *model.UserConversation, msg *model.UserMessage, preview string) error
	MarkConversationRead(convID, readerID uint64, readAt time.Time) error
	ListContacts(userID uint64) ([]model.ContactUserItem, error)
}

type messageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) GetTotalUnreadCount(userID uint64) (int64, error) {
	var total1, total2 int64
	if err := r.db.Model(&model.UserConversation{}).
		Where("user1_id = ?", userID).
		Select("COALESCE(SUM(user1_unread_count), 0)").
		Scan(&total1).Error; err != nil {
		return 0, err
	}
	if err := r.db.Model(&model.UserConversation{}).
		Where("user2_id = ?", userID).
		Select("COALESCE(SUM(user2_unread_count), 0)").
		Scan(&total2).Error; err != nil {
		return 0, err
	}
	return total1 + total2, nil
}

func (r *messageRepository) ListConversations(userID uint64) ([]model.UserConversation, error) {
	var convs []model.UserConversation
	if err := r.db.Where("user1_id = ? OR user2_id = ?", userID, userID).
		Order("last_message_at DESC, updated_at DESC").
		Find(&convs).Error; err != nil {
		return nil, err
	}
	if convs == nil {
		convs = []model.UserConversation{}
	}
	return convs, nil
}

func (r *messageRepository) FindOrCreateConversation(user1ID, user2ID uint64) (*model.UserConversation, error) {
	var conv model.UserConversation
	err := r.db.Where("user1_id = ? AND user2_id = ?", user1ID, user2ID).First(&conv).Error
	if err == nil {
		return &conv, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	conv = model.UserConversation{
		User1ID: user1ID,
		User2ID: user2ID,
	}
	if err := r.db.Create(&conv).Error; err != nil {
		// Handle concurrent creation race condition by re-querying.
		var existing model.UserConversation
		if qErr := r.db.Where("user1_id = ? AND user2_id = ?", user1ID, user2ID).First(&existing).Error; qErr == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &conv, nil
}

func (r *messageRepository) FindConversationByID(convID uint64) (*model.UserConversation, error) {
	var conv model.UserConversation
	if err := r.db.First(&conv, convID).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *messageRepository) ListMessages(convID uint64, beforeID uint64, limit int) ([]model.UserMessage, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tx := r.db.Where("conversation_id = ?", convID)
	if beforeID > 0 {
		tx = tx.Where("id < ?", beforeID)
	}
	var msgs []model.UserMessage
	if err := tx.Order("id DESC").Limit(limit + 1).Find(&msgs).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	// Reverse to chronological ascending order for display.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	if msgs == nil {
		msgs = []model.UserMessage{}
	}
	return msgs, hasMore, nil
}

func (r *messageRepository) CreateMessage(conv *model.UserConversation, msg *model.UserMessage, preview string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"last_message_id":      msg.ID,
			"last_message_preview": preview,
			"last_message_at":      msg.CreatedAt,
		}
		if msg.ReceiverID == conv.User1ID {
			updates["user1_unread_count"] = gorm.Expr("user1_unread_count + 1")
		} else {
			updates["user2_unread_count"] = gorm.Expr("user2_unread_count + 1")
		}
		return tx.Model(&model.UserConversation{}).Where("id = ?", conv.ID).Updates(updates).Error
	})
}

func (r *messageRepository) MarkConversationRead(convID, readerID uint64, readAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var conv model.UserConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&conv, convID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.UserMessage{}).
			Where("conversation_id = ? AND receiver_id = ? AND is_read = ?", convID, readerID, false).
			Updates(map[string]interface{}{
				"is_read": true,
				"read_at": readAt,
			}).Error; err != nil {
			return err
		}
		var remaining int64
		if err := tx.Model(&model.UserMessage{}).
			Where("conversation_id = ? AND receiver_id = ? AND is_read = ?", convID, readerID, false).
			Count(&remaining).Error; err != nil {
			return err
		}
		if conv.User1ID == readerID {
			return tx.Model(&model.UserConversation{}).
				Where("id = ?", convID).
				Update("user1_unread_count", remaining).Error
		}
		if conv.User2ID == readerID {
			return tx.Model(&model.UserConversation{}).
				Where("id = ?", convID).
				Update("user2_unread_count", remaining).Error
		}
		return nil
	})
}

type contactRow struct {
	UserID     uint64
	Username   string
	Nickname   string
	Avatar     string
	SourceID   uint64
	SourceName string
}

func (r *messageRepository) ListContacts(userID uint64) ([]model.ContactUserItem, error) {
	var domainRows []contactRow
	if err := r.db.Table("domain_member AS my_dm").
		Select("DISTINCT u.id AS user_id, u.username, u.nickname, u.avatar, d.id AS source_id, d.domain_name AS source_name").
		Joins("INNER JOIN biz_domain d ON d.id = my_dm.domain_id AND d.deleted_at IS NULL").
		Joins("INNER JOIN domain_member peer_dm ON peer_dm.domain_id = my_dm.domain_id AND peer_dm.user_id != ?", userID).
		Joins("INNER JOIN sys_user u ON u.id = peer_dm.user_id AND u.deleted_at IS NULL AND u.status = ?", model.UserStatusActive).
		Where("my_dm.user_id = ?", userID).
		Scan(&domainRows).Error; err != nil {
		return nil, err
	}

	var orgRows []contactRow
	if err := r.db.Table("organization_member AS my_om").
		Select("DISTINCT u.id AS user_id, u.username, u.nickname, u.avatar, o.id AS source_id, o.org_name AS source_name").
		Joins("INNER JOIN biz_organization o ON o.id = my_om.org_id AND o.deleted_at IS NULL").
		Joins("INNER JOIN organization_member peer_om ON peer_om.org_id = my_om.org_id AND peer_om.user_id != ?", userID).
		Joins("INNER JOIN sys_user u ON u.id = peer_om.user_id AND u.deleted_at IS NULL AND u.status = ?", model.UserStatusActive).
		Where("my_om.user_id = ?", userID).
		Scan(&orgRows).Error; err != nil {
		return nil, err
	}

	return mergeContactRows(userID, domainRows, orgRows), nil
}

func mergeContactRows(userID uint64, domainRows, orgRows []contactRow) []model.ContactUserItem {

	contacts := make(map[uint64]*model.ContactUserItem)
	type sourceKey struct {
		typ string
		id  uint64
	}
	seenSources := make(map[uint64]map[sourceKey]struct{})

	appendRows := func(sourceType string, rows []contactRow) {
		for _, row := range rows {
			if row.UserID == 0 || row.UserID == userID {
				continue
			}
			item, ok := contacts[row.UserID]
			if !ok {
				item = &model.ContactUserItem{
					ID:       row.UserID,
					Username: row.Username,
					Nickname: row.Nickname,
					Avatar:   row.Avatar,
					Sources:  []model.ContactSource{},
				}
				contacts[row.UserID] = item
				seenSources[row.UserID] = make(map[sourceKey]struct{})
			}
			key := sourceKey{typ: sourceType, id: row.SourceID}
			if _, exists := seenSources[row.UserID][key]; !exists {
				seenSources[row.UserID][key] = struct{}{}
				item.Sources = append(item.Sources, model.ContactSource{
					Type: sourceType,
					ID:   row.SourceID,
					Name: row.SourceName,
				})
			}
		}
	}

	appendRows("domain", domainRows)
	appendRows("org", orgRows)

	out := make([]model.ContactUserItem, 0, len(contacts))
	for _, item := range contacts {
		sort.Slice(item.Sources, func(i, j int) bool {
			if item.Sources[i].Type != item.Sources[j].Type {
				return item.Sources[i].Type < item.Sources[j].Type
			}
			return item.Sources[i].ID < item.Sources[j].ID
		})
		out = append(out, *item)
	}

	sort.Slice(out, func(i, j int) bool {
		nameI := strings.ToLower(strings.TrimSpace(out[i].Nickname))
		if nameI == "" {
			nameI = strings.ToLower(out[i].Username)
		}
		nameJ := strings.ToLower(strings.TrimSpace(out[j].Nickname))
		if nameJ == "" {
			nameJ = strings.ToLower(out[j].Username)
		}
		if nameI != nameJ {
			return nameI < nameJ
		}
		return out[i].ID < out[j].ID
	})
	return out
}
