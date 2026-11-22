package model

import "time"

const (
	MessageTypeText      = "text"
	MessageTypeAssetCard = "asset_card"
)

// UserConversation represents a direct messaging thread between two users.
// Always store User1ID < User2ID.
type UserConversation struct {
	ID                 uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	User1ID            uint64     `gorm:"uniqueIndex:idx_conv_pair;not null" json:"user1_id"`
	User2ID            uint64     `gorm:"uniqueIndex:idx_conv_pair;not null" json:"user2_id"`
	LastMessageID      uint64     `gorm:"not null;default:0" json:"last_message_id"`
	LastMessagePreview string     `gorm:"type:varchar(255);not null;default:''" json:"last_message_preview"`
	LastMessageAt      *time.Time `gorm:"type:datetime" json:"last_message_at,omitempty"`
	User1UnreadCount   int        `gorm:"type:int;not null;default:0" json:"user1_unread_count"`
	User2UnreadCount   int        `gorm:"type:int;not null;default:0" json:"user2_unread_count"`
	CreatedAt          time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserConversation) TableName() string {
	return "biz_user_conversation"
}

// UserMessage represents a single direct message in a conversation.
type UserMessage struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64     `gorm:"index;not null" json:"conversation_id"`
	SenderID       uint64     `gorm:"index;not null" json:"sender_id"`
	ReceiverID     uint64     `gorm:"index;not null" json:"receiver_id"`
	MsgType        string     `gorm:"type:varchar(24);not null;default:'text'" json:"msg_type"`
	Content        string     `gorm:"type:text;not null" json:"content"`
	Payload        string     `gorm:"type:text" json:"payload,omitempty"`
	IsRead         bool       `gorm:"type:tinyint(1);not null;default:0" json:"is_read"`
	ReadAt         *time.Time `gorm:"type:datetime" json:"read_at,omitempty"`
	CreatedAt      time.Time  `gorm:"index;autoCreateTime" json:"created_at"`
}

func (UserMessage) TableName() string {
	return "biz_user_message"
}

// CreateConversationRequest represents POST /api/v1/messages/conversations payload.
type CreateConversationRequest struct {
	TargetUserID uint64 `json:"target_user_id" validate:"required,gt=0"`
}

// AssetCardPayload represents the structured payload for an asset_card message.
type AssetCardPayload struct {
	AssetID     uint64 `json:"asset_id"`
	AssetName   string `json:"asset_name"`
	AssetType   string `json:"asset_type"`
	DomainID    uint64 `json:"domain_id"`
	DomainName  string `json:"domain_name"`
	DocID       uint64 `json:"doc_id,omitempty"`
	DocName     string `json:"doc_name,omitempty"`
	IsFolder    bool   `json:"is_folder,omitempty"`
	RefDomainID uint64 `json:"ref_domain_id,omitempty"`
	Note        string `json:"note,omitempty"`
}

// SendMessageRequest represents POST /api/v1/messages/conversations/:id/messages payload.
type SendMessageRequest struct {
	Content string `json:"content"`
	MsgType string `json:"msg_type"`
	Payload string `json:"payload"`
}

// ShareAssetCardRequest represents POST /api/v1/messages/share-card payload.
type ShareAssetCardRequest struct {
	RecipientIDs []uint64 `json:"recipient_ids" validate:"required,min=1,max=50"`
	AssetID      uint64   `json:"asset_id" validate:"required,gt=0"`
	DocID        uint64   `json:"doc_id"`
	RefDomainID  uint64   `json:"ref_domain_id"`
	Note         string   `json:"note" validate:"max=500"`
}

// ContactSource describes a shared domain or organization with a contact user.
type ContactSource struct {
	Type string `json:"type"` // "domain" | "org"
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// ContactUserItem represents a peer user who shares at least one domain or organization with the current user.
type ContactUserItem struct {
	ID       uint64          `json:"id"`
	Username string          `json:"username"`
	Nickname string          `json:"nickname"`
	Avatar   string          `json:"avatar"`
	Sources  []ContactSource `json:"sources"`
}

// ConversationItem represents a conversation summary for the current user.
type ConversationItem struct {
	ID                 uint64     `json:"id"`
	Peer               UserBrief  `json:"peer"`
	LastMessagePreview string     `json:"last_message_preview"`
	LastMessageAt      *time.Time `json:"last_message_at,omitempty"`
	UnreadCount        int        `json:"unread_count"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// UserMessageItem represents a direct message item returned by message endpoints.
type UserMessageItem = UserMessage
