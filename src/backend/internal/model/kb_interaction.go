package model

import (
	"time"

	"gorm.io/gorm"
)

// KBDocumentLike records a user's like on a KB document.
type KBDocumentLike struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	KBAssetID uint64    `gorm:"index;not null" json:"kb_asset_id"`
	DocID     uint64    `gorm:"uniqueIndex:idx_kb_doc_like_user;not null" json:"doc_id"`
	UserID    uint64    `gorm:"uniqueIndex:idx_kb_doc_like_user;not null" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (KBDocumentLike) TableName() string {
	return "kb_document_like"
}

// KBDocumentFavorite records a user's private favorite on a KB document.
type KBDocumentFavorite struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	KBAssetID uint64    `gorm:"index;not null" json:"kb_asset_id"`
	DocID     uint64    `gorm:"uniqueIndex:idx_kb_doc_fav_user;not null" json:"doc_id"`
	UserID    uint64    `gorm:"uniqueIndex:idx_kb_doc_fav_user;not null" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (KBDocumentFavorite) TableName() string {
	return "kb_document_favorite"
}

// KBDocumentComment records an annotation or comment on a KB document.
type KBDocumentComment struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	KBAssetID uint64         `gorm:"index;not null" json:"kb_asset_id"`
	DocID     uint64         `gorm:"index;not null" json:"doc_id"`
	UserID    uint64         `gorm:"index;not null" json:"user_id"`
	ParentID  *uint64        `gorm:"index" json:"parent_id,omitempty"`
	QuoteText string         `gorm:"type:varchar(500);not null;default:''" json:"quote_text"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (KBDocumentComment) TableName() string {
	return "kb_document_comment"
}

// KBDocumentInteractionsResponse represents GET /kb/:id/documents/:docId/interactions.
// Note: favorite_count and favorite_users are intentionally omitted to keep favorites private.
type KBDocumentInteractionsResponse struct {
	LikeCount     int64       `json:"like_count"`
	LikedByMe     bool        `json:"liked_by_me"`
	LikeUsers     []UserBrief `json:"like_users"`
	FavoritedByMe bool        `json:"favorited_by_me"`
	CommentCount  int64       `json:"comment_count"`
}

// KBDocumentLikeToggleResponse represents POST /kb/:id/documents/:docId/like.
type KBDocumentLikeToggleResponse struct {
	LikeCount int64       `json:"like_count"`
	LikedByMe bool        `json:"liked_by_me"`
	LikeUsers []UserBrief `json:"like_users"`
}

// CreateKBCommentRequest represents POST /kb/:id/documents/:docId/comments payload.
type CreateKBCommentRequest struct {
	Content   string  `json:"content" validate:"required"`
	QuoteText string  `json:"quote_text" validate:"max=500"`
	ParentID  *uint64 `json:"parent_id"`
}

// KBCommentItem represents a comment/annotation item with author details and delete permission.
type KBCommentItem struct {
	ID        uint64    `json:"id"`
	KBAssetID uint64    `json:"kb_asset_id"`
	DocID     uint64    `json:"doc_id"`
	UserID    uint64    `json:"user_id"`
	Author    UserBrief `json:"author"`
	ParentID  *uint64   `json:"parent_id,omitempty"`
	QuoteText string    `json:"quote_text"`
	Content   string    `json:"content"`
	CanDelete bool      `json:"can_delete"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// KBReadingProgress records a user's reading progress on a KB document.
type KBReadingProgress struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	KBAssetID   uint64    `gorm:"index;not null" json:"kb_asset_id"`
	DocID       uint64    `gorm:"uniqueIndex:idx_kb_reading_progress_user_doc;not null" json:"doc_id"`
	UserID      uint64    `gorm:"uniqueIndex:idx_kb_reading_progress_user_doc;not null" json:"user_id"`
	CurrentPage int       `gorm:"not null;default:1" json:"current_page"`
	TotalPages  int       `gorm:"not null;default:1" json:"total_pages"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (KBReadingProgress) TableName() string {
	return "kb_reading_progress"
}

// UpdateReadingProgressRequest represents PUT /kb/:id/documents/:docId/reading-progress payload.
type UpdateReadingProgressRequest struct {
	CurrentPage int `json:"current_page" validate:"required,gte=1"`
	TotalPages  int `json:"total_pages" validate:"required,gte=1"`
}

// ReadingProgressResponse represents GET/PUT /kb/:id/documents/:docId/reading-progress response.
type ReadingProgressResponse struct {
	CurrentPage int        `json:"current_page"`
	TotalPages  int        `json:"total_pages"`
	UpdatedAt   *time.Time `json:"updated_at"`
}
