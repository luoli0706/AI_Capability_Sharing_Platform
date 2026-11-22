package repository

import (
	"errors"
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
