package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	DomainEventTypeFolderVote = "folder_vote"
	DomainEventTypeLottery    = "lottery"
	DomainEventTypeMatching   = "matching"
)

const (
	DomainEventStatusUpcoming = "upcoming"
	DomainEventStatusActive   = "active"
	DomainEventStatusEnded    = "ended"
)

// DomainEvent represents an extensible event in a domain's event library.
type DomainEvent struct {
	ID             uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	DomainID       uint64         `json:"domain_id" gorm:"not null;index"`
	Title          string         `json:"title" gorm:"size:200;not null"`
	Description    string         `json:"description" gorm:"type:text"`
	EventType      string         `json:"event_type" gorm:"size:32;not null;default:'folder_vote';index"`
	AllowMultiVote bool           `json:"allow_multi_vote" gorm:"not null;default:false"`
	ConfigJSON     string         `json:"config_json,omitempty" gorm:"type:text"`
	CreatorID      uint64         `json:"creator_id" gorm:"not null;index"`
	StartTime      time.Time      `json:"start_time" gorm:"not null;index"`
	EndTime        time.Time      `json:"end_time" gorm:"not null;index"`
	ClosedAt       *time.Time     `json:"closed_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (DomainEvent) TableName() string {
	return "domain_events"
}

// ComputeStatus returns upcoming, active, or ended relative to now.
func (e *DomainEvent) ComputeStatus(now time.Time) string {
	if e.ClosedAt != nil {
		return DomainEventStatusEnded
	}
	if now.Before(e.StartTime) {
		return DomainEventStatusUpcoming
	}
	if now.After(e.EndTime) {
		return DomainEventStatusEnded
	}
	return DomainEventStatusActive
}

// DomainEventCandidate represents a candidate folder in a folder_vote event.
type DomainEventCandidate struct {
	ID            uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventID       uint64    `json:"event_id" gorm:"not null;uniqueIndex:uk_event_kb_folder,priority:1;index"`
	KBAssetID     uint64    `json:"kb_asset_id" gorm:"not null;uniqueIndex:uk_event_kb_folder,priority:2"`
	KBName        string    `json:"kb_name" gorm:"size:255;not null"`
	FolderDocID   uint64    `json:"folder_doc_id" gorm:"not null;uniqueIndex:uk_event_kb_folder,priority:3"`
	FolderPath    string    `json:"folder_path" gorm:"size:512;not null"`
	FolderName    string    `json:"folder_name" gorm:"size:255;not null"`
	AddedByUserID uint64    `json:"added_by_user_id" gorm:"not null;index"`
	VoteCount     int64     `json:"vote_count" gorm:"not null;default:0"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (DomainEventCandidate) TableName() string {
	return "domain_event_candidates"
}

// DomainEventVote records a user's vote on a candidate in a domain event.
type DomainEventVote struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventID     uint64    `json:"event_id" gorm:"not null;uniqueIndex:uk_event_cand_user,priority:1;index"`
	CandidateID uint64    `json:"candidate_id" gorm:"not null;uniqueIndex:uk_event_cand_user,priority:2;index"`
	UserID      uint64    `json:"user_id" gorm:"not null;uniqueIndex:uk_event_cand_user,priority:3;index"`
	CreatedAt   time.Time `json:"created_at"`
}

func (DomainEventVote) TableName() string {
	return "domain_event_votes"
}

type DomainEventSummary struct {
	DomainEvent
	Status         string `json:"status"`
	CreatorName    string `json:"creator_name"`
	CandidateCount int64  `json:"candidate_count"`
	TotalVotes     int64  `json:"total_votes"`
	CanManage      bool   `json:"can_manage"`
}

type DomainEventCandidateView struct {
	DomainEventCandidate
	AddedByName string `json:"added_by_name"`
	HasVoted    bool   `json:"has_voted"`
}

type DomainEventDetail struct {
	DomainEventSummary
	Candidates         []DomainEventCandidateView `json:"candidates"`
	MyVotedCandidateIDs []uint64                  `json:"my_voted_candidate_ids"`
}

type CreateDomainEventRequest struct {
	Title          string    `json:"title" validate:"required,max=200"`
	Description    string    `json:"description" validate:"max=5000"`
	EventType      string    `json:"event_type" validate:"max=32"`
	AllowMultiVote bool      `json:"allow_multi_vote"`
	StartTime      time.Time `json:"start_time" validate:"required"`
	EndTime        time.Time `json:"end_time" validate:"required"`
}

type AddDomainEventCandidateRequest struct {
	KBAssetID   uint64 `json:"kb_asset_id" validate:"required"`
	FolderDocID uint64 `json:"folder_doc_id" validate:"required"`
}
