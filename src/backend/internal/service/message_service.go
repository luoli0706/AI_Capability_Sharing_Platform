package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

const (
	maxMessageRunes  = 4000
	maxCardNoteRunes = 500
)

type MessageService interface {
	SetAssetAccessRepos(
		assetRepo repository.AssetRepository,
		kbRepo repository.KBRepository,
		domainRepo repository.DomainRepository,
		orgRepo repository.OrgRepository,
		govRepo repository.GovernanceRepository,
		crossRefRepo repository.CrossRefRepository,
	)
	GetUnreadCount(userID uint64) (int64, error)
	ListConversations(userID uint64) ([]model.ConversationItem, error)
	ListContacts(userID uint64) ([]model.ContactUserItem, error)
	CreateOrGetConversation(userID, targetUserID uint64) (*model.ConversationItem, error)
	ListMessages(userID, convID uint64, beforeID uint64, limit int) ([]model.UserMessage, bool, error)
	SendMessage(userID, convID uint64, content string) (*model.UserMessage, error)
	SendMessageReq(userID uint64, isSuperAdmin bool, convID uint64, req model.SendMessageRequest) (*model.UserMessage, error)
	ShareAssetCard(senderID uint64, isSuperAdmin bool, req model.ShareAssetCardRequest) (int, error)
	MarkRead(userID, convID uint64) error
}

type messageService struct {
	msgRepo      repository.MessageRepository
	userRepo     repository.UserRepository
	assetRepo    repository.AssetRepository
	kbRepo       repository.KBRepository
	domainRepo   repository.DomainRepository
	orgRepo      repository.OrgRepository
	govRepo      repository.GovernanceRepository
	crossRefRepo repository.CrossRefRepository
}

func (s *messageService) SetAssetAccessRepos(
	assetRepo repository.AssetRepository,
	kbRepo repository.KBRepository,
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	govRepo repository.GovernanceRepository,
	crossRefRepo repository.CrossRefRepository,
) {
	s.assetRepo = assetRepo
	s.kbRepo = kbRepo
	s.domainRepo = domainRepo
	s.orgRepo = orgRepo
	s.govRepo = govRepo
	s.crossRefRepo = crossRefRepo
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

func (s *messageService) ListContacts(userID uint64) ([]model.ContactUserItem, error) {
	return s.msgRepo.ListContacts(userID)
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
	return s.SendMessageReq(userID, false, convID, model.SendMessageRequest{
		MsgType: model.MessageTypeText,
		Content: content,
	})
}

func (s *messageService) SendMessageReq(userID uint64, isSuperAdmin bool, convID uint64, req model.SendMessageRequest) (*model.UserMessage, error) {
	msgType := strings.TrimSpace(req.MsgType)
	if msgType == "" {
		msgType = model.MessageTypeText
	}

	var content, payloadJSON, preview string
	switch msgType {
	case model.MessageTypeText:
		trimmed := strings.TrimSpace(req.Content)
		if trimmed == "" {
			return nil, NewBadRequest("message content cannot be empty")
		}
		if utf8.RuneCountInString(trimmed) > maxMessageRunes {
			return nil, NewBadRequest("message content exceeds 4000 characters")
		}
		content = trimmed
		preview = truncateRunes(trimmed, 120)
	case model.MessageTypeAssetCard:
		var cardReq model.AssetCardPayload
		if err := json.Unmarshal([]byte(req.Payload), &cardReq); err != nil || cardReq.AssetID == 0 {
			return nil, NewBadRequest("invalid asset card payload")
		}
		note := strings.TrimSpace(cardReq.Note)
		var err error
		payloadJSON, preview, content, err = s.buildSanitizedAssetCard(userID, isSuperAdmin, cardReq.AssetID, cardReq.DocID, cardReq.RefDomainID, cardReq.CrossRefID, note)
		if err != nil {
			return nil, err
		}
	default:
		return nil, NewBadRequest("unsupported message type")
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
		MsgType:        msgType,
		Content:        content,
		Payload:        payloadJSON,
		CreatedAt:      time.Now(),
	}
	if err := s.msgRepo.CreateMessage(conv, msg, preview); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *messageService) ShareAssetCard(senderID uint64, isSuperAdmin bool, req model.ShareAssetCardRequest) (int, error) {
	seen := make(map[uint64]struct{}, len(req.RecipientIDs))
	recipients := make([]uint64, 0, len(req.RecipientIDs))
	for _, rid := range req.RecipientIDs {
		if rid == 0 || rid == senderID {
			continue
		}
		if _, ok := seen[rid]; !ok {
			seen[rid] = struct{}{}
			recipients = append(recipients, rid)
		}
	}
	if len(recipients) == 0 {
		return 0, NewBadRequest("at least one valid recipient is required")
	}

	payloadJSON, preview, content, err := s.buildSanitizedAssetCard(senderID, isSuperAdmin, req.AssetID, req.DocID, req.RefDomainID, req.CrossRefID, req.Note)
	if err != nil {
		return 0, err
	}

	for _, rid := range recipients {
		target, err := s.userRepo.FindByID(rid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, NewNotFound("user not found")
			}
			return 0, err
		}
		if target.Status != model.UserStatusActive {
			return 0, NewBadRequest("目标用户不可用")
		}
	}

	sentCount := 0
	for _, rid := range recipients {
		u1, u2 := senderID, rid
		if u1 > u2 {
			u1, u2 = u2, u1
		}
		conv, err := s.msgRepo.FindOrCreateConversation(u1, u2)
		if err != nil {
			return sentCount, err
		}
		msg := &model.UserMessage{
			ConversationID: conv.ID,
			SenderID:       senderID,
			ReceiverID:     rid,
			MsgType:        model.MessageTypeAssetCard,
			Content:        content,
			Payload:        payloadJSON,
			CreatedAt:      time.Now(),
		}
		if err := s.msgRepo.CreateMessage(conv, msg, preview); err != nil {
			return sentCount, err
		}
		sentCount++
	}
	return sentCount, nil
}

func (s *messageService) buildSanitizedAssetCard(senderID uint64, isSuperAdmin bool, assetID, docID, refDomainID, crossRefID uint64, rawNote string) (string, string, string, error) {
	if s.assetRepo == nil || s.domainRepo == nil {
		return "", "", "", NewBadRequest("asset sharing is not configured")
	}
	res, err := checkAssetAndDocAccess(
		s.assetRepo, s.kbRepo, s.domainRepo, s.orgRepo, s.govRepo, s.crossRefRepo,
		senderID, isSuperAdmin, assetID, docID, refDomainID,
	)
	if err != nil {
		return "", "", "", err
	}
	if !res.Result.Accessible {
		if res.Asset == nil {
			return "", "", "", NewNotFound(res.Result.Reason)
		}
		return "", "", "", NewForbidden(res.Result.Reason)
	}

	note := strings.TrimSpace(rawNote)
	if utf8.RuneCountInString(note) > maxCardNoteRunes {
		return "", "", "", NewBadRequest("note exceeds 500 characters")
	}

	card := model.AssetCardPayload{
		AssetID:     res.Asset.ID,
		AssetName:   res.Asset.Name,
		AssetType:   res.Asset.AssetType,
		DomainID:    res.Asset.DomainID,
		DomainName:  res.Asset.DomainName,
		RefDomainID: refDomainID,
		CrossRefID:  crossRefID,
		Note:        note,
	}
	if docID > 0 && res.Doc != nil {
		card.DocID = res.Doc.ID
		card.DocName = res.Doc.FileName
		card.IsFolder = res.Doc.IsFolder
	}

	rawJSON, err := json.Marshal(card)
	if err != nil {
		return "", "", "", err
	}

	var preview string
	if card.DocID > 0 && card.DocName != "" {
		if card.IsFolder {
			preview = truncateRunes("[知识库文件夹] "+card.DocName, 120)
		} else {
			preview = truncateRunes("[知识库文档] "+card.DocName, 120)
		}
	} else {
		preview = truncateRunes("[资产卡片] "+card.AssetName, 120)
	}
	content := note
	if content == "" {
		content = preview
	}
	return string(rawJSON), preview, content, nil
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
