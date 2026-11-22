package service

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

const maxMessageRunes = 4000

type MessageService interface {
	GetUnreadCount(userID uint64) (int64, error)
	ListConversations(userID uint64) ([]model.ConversationItem, error)
	CreateOrGetConversation(userID, targetUserID uint64) (*model.ConversationItem, error)
	ListMessages(userID, convID uint64, beforeID uint64, limit int) ([]model.UserMessage, bool, error)
	SendMessage(userID, convID uint64, content string) (*model.UserMessage, error)
	MarkRead(userID, convID uint64) error
}

type messageService struct {
	msgRepo  repository.MessageRepository
	userRepo repository.UserRepository
}

func NewMessageService(msgRepo repository.MessageRepository, userRepo repository.UserRepository) MessageService {
	return &messageService{
		msgRepo:  msgRepo,
		userRepo: userRepo,
	}
}

func (s *messageService) GetUnreadCount(userID uint64) (int64, error) {
	return s.msgRepo.GetTotalUnreadCount(userID)
}

func (s *messageService) ListConversations(userID uint64) ([]model.ConversationItem, error) {
	convs, err := s.msgRepo.ListConversations(userID)
	if err != nil {
		return nil, err
	}
	if len(convs) == 0 {
		return []model.ConversationItem{}, nil
	}

	peerIDs := make([]uint64, 0, len(convs))
	seen := make(map[uint64]struct{}, len(convs))
	for _, c := range convs {
		peerID := c.User2ID
		if c.User2ID == userID {
			peerID = c.User1ID
		}
		if _, ok := seen[peerID]; !ok {
			seen[peerID] = struct{}{}
			peerIDs = append(peerIDs, peerID)
		}
	}

	briefs, err := s.userRepo.FindBriefsByIDs(peerIDs)
	if err != nil {
		return nil, err
	}

	items := make([]model.ConversationItem, 0, len(convs))
	for _, c := range convs {
		peerID := c.User2ID
		unread := c.User1UnreadCount
		if c.User2ID == userID {
			peerID = c.User1ID
			unread = c.User2UnreadCount
		}
		peer := briefs[peerID]
		if peer.ID == 0 {
			peer.ID = peerID
		}
		items = append(items, model.ConversationItem{
			ID:                 c.ID,
			Peer:               peer,
			LastMessagePreview: c.LastMessagePreview,
			LastMessageAt:      c.LastMessageAt,
			UnreadCount:        unread,
			UpdatedAt:          c.UpdatedAt,
		})
	}
	return items, nil
}

func (s *messageService) CreateOrGetConversation(userID, targetUserID uint64) (*model.ConversationItem, error) {
	if userID == targetUserID {
		return nil, NewBadRequest("cannot start conversation with yourself")
	}
	target, err := s.userRepo.FindByID(targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("user not found")
		}
		return nil, err
	}
	if target.Status != model.UserStatusActive {
		return nil, NewBadRequest("目标用户不可用")
	}

	u1, u2 := userID, targetUserID
	if u1 > u2 {
		u1, u2 = u2, u1
	}
	conv, err := s.msgRepo.FindOrCreateConversation(u1, u2)
	if err != nil {
		return nil, err
	}

	unread := conv.User1UnreadCount
	if conv.User2ID == userID {
		unread = conv.User2UnreadCount
	}
	return &model.ConversationItem{
		ID:                 conv.ID,
		Peer:               model.ToUserBrief(target),
		LastMessagePreview: conv.LastMessagePreview,
		LastMessageAt:      conv.LastMessageAt,
		UnreadCount:        unread,
		UpdatedAt:          conv.UpdatedAt,
	}, nil
}

func (s *messageService) ListMessages(userID, convID uint64, beforeID uint64, limit int) ([]model.UserMessage, bool, error) {
	conv, err := s.requireParticipant(userID, convID)
	if err != nil {
		return nil, false, err
	}
	if err := s.msgRepo.MarkConversationRead(conv.ID, userID, time.Now()); err != nil {
		return nil, false, err
	}
	return s.msgRepo.ListMessages(conv.ID, beforeID, limit)
}

func (s *messageService) SendMessage(userID, convID uint64, content string) (*model.UserMessage, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, NewBadRequest("message content cannot be empty")
	}
	if utf8.RuneCountInString(trimmed) > maxMessageRunes {
		return nil, NewBadRequest("message content exceeds 4000 characters")
	}

	conv, err := s.requireParticipant(userID, convID)
	if err != nil {
		return nil, err
	}

	receiverID := conv.User2ID
	if conv.User2ID == userID {
		receiverID = conv.User1ID
	}

	receiver, err := s.userRepo.FindByID(receiverID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("user not found")
		}
		return nil, err
	}
	if receiver.Status != model.UserStatusActive {
		return nil, NewBadRequest("目标用户不可用")
	}

	msg := &model.UserMessage{
		ConversationID: conv.ID,
		SenderID:       userID,
		ReceiverID:     receiverID,
		Content:        trimmed,
		CreatedAt:      time.Now(),
	}
	preview := truncateRunes(trimmed, 120)
	if err := s.msgRepo.CreateMessage(conv, msg, preview); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *messageService) MarkRead(userID, convID uint64) error {
	conv, err := s.requireParticipant(userID, convID)
	if err != nil {
		return err
	}
	return s.msgRepo.MarkConversationRead(conv.ID, userID, time.Now())
}

func (s *messageService) requireParticipant(userID, convID uint64) (*model.UserConversation, error) {
	conv, err := s.msgRepo.FindConversationByID(convID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("conversation not found")
		}
		return nil, err
	}
	if conv.User1ID != userID && conv.User2ID != userID {
		return nil, NewForbidden("no permission to access this conversation")
	}
	return conv, nil
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
