package repository

import (
	"errors"
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

var (
	ErrCandidateExists       = errors.New("candidate folder already exists in this event")
	ErrAlreadyVotedInEvent   = errors.New("user has already voted in this event")
	ErrAlreadyVotedCandidate = errors.New("user has already voted for this candidate")
)

type DomainEventRepository interface {
	CreateEvent(event *model.DomainEvent) error
	FindEventByID(domainID, eventID uint64) (*model.DomainEvent, error)
	ListEventsByDomain(domainID uint64) ([]model.DomainEvent, error)
	CloseEvent(domainID, eventID uint64, closedAt time.Time) error
	DeleteEvent(domainID, eventID uint64) error
	ListCandidatesByEvent(eventID uint64) ([]model.DomainEventCandidate, error)
	GetEventCandidateAndVoteStats(eventIDs []uint64) (candCountMap map[uint64]int64, voteSumMap map[uint64]int64, err error)
	FindCandidateByID(eventID, candidateID uint64) (*model.DomainEventCandidate, error)
	AddCandidateWithInitialVote(eventID uint64, allowMultiVote bool, candidate *model.DomainEventCandidate, userID uint64) error
	CastVote(eventID, candidateID, userID uint64, allowMultiVote bool) error
	CancelVote(eventID, candidateID, userID uint64) error
	ListUserVotedCandidateIDs(eventID, userID uint64) ([]uint64, error)
}

type domainEventRepository struct {
	db *gorm.DB
}

func NewDomainEventRepository(db *gorm.DB) DomainEventRepository {
	return &domainEventRepository{db: db}
}

func (r *domainEventRepository) CreateEvent(event *model.DomainEvent) error {
	return r.db.Create(event).Error
}

func (r *domainEventRepository) FindEventByID(domainID, eventID uint64) (*model.DomainEvent, error) {
	var event model.DomainEvent
	if err := r.db.Where("id = ? AND domain_id = ?", eventID, domainID).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *domainEventRepository) ListEventsByDomain(domainID uint64) ([]model.DomainEvent, error) {
	var events []model.DomainEvent
	if err := r.db.Where("domain_id = ?", domainID).Order("created_at DESC, id DESC").Find(&events).Error; err != nil {
		return nil, err
	}
	if events == nil {
		events = []model.DomainEvent{}
	}
	return events, nil
}

func (r *domainEventRepository) CloseEvent(domainID, eventID uint64, closedAt time.Time) error {
	res := r.db.Model(&model.DomainEvent{}).
		Where("id = ? AND domain_id = ?", eventID, domainID).
		Update("closed_at", closedAt)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *domainEventRepository) DeleteEvent(domainID, eventID uint64) error {
	res := r.db.Where("id = ? AND domain_id = ?", eventID, domainID).Delete(&model.DomainEvent{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *domainEventRepository) ListCandidatesByEvent(eventID uint64) ([]model.DomainEventCandidate, error) {
	var candidates []model.DomainEventCandidate
	if err := r.db.Where("event_id = ?", eventID).
		Order("vote_count DESC, id ASC").
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	if candidates == nil {
		candidates = []model.DomainEventCandidate{}
	}
	return candidates, nil
}

func (r *domainEventRepository) GetEventCandidateAndVoteStats(eventIDs []uint64) (map[uint64]int64, map[uint64]int64, error) {
	candCountMap := make(map[uint64]int64)
	voteSumMap := make(map[uint64]int64)
	if len(eventIDs) == 0 {
		return candCountMap, voteSumMap, nil
	}

	type statRow struct {
		EventID   uint64
		CandCount int64
		VoteSum   int64
	}
	var rows []statRow
	if err := r.db.Model(&model.DomainEventCandidate{}).
		Select("event_id, COUNT(*) AS cand_count, COALESCE(SUM(vote_count), 0) AS vote_sum").
		Where("event_id IN ?", eventIDs).
		Group("event_id").
		Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		candCountMap[row.EventID] = row.CandCount
		voteSumMap[row.EventID] = row.VoteSum
	}
	return candCountMap, voteSumMap, nil
}

func (r *domainEventRepository) FindCandidateByID(eventID, candidateID uint64) (*model.DomainEventCandidate, error) {
	var cand model.DomainEventCandidate
	if err := r.db.Where("id = ? AND event_id = ?", candidateID, eventID).First(&cand).Error; err != nil {
		return nil, err
	}
	return &cand, nil
}

func (r *domainEventRepository) AddCandidateWithInitialVote(eventID uint64, allowMultiVote bool, candidate *model.DomainEventCandidate, userID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existingCount int64
		if err := tx.Model(&model.DomainEventCandidate{}).
			Where("event_id = ? AND kb_asset_id = ? AND folder_doc_id = ?", eventID, candidate.KBAssetID, candidate.FolderDocID).
			Count(&existingCount).Error; err != nil {
			return err
		}
		if existingCount > 0 {
			return ErrCandidateExists
		}

		if !allowMultiVote {
			var userVoteCount int64
			if err := tx.Model(&model.DomainEventVote{}).
				Where("event_id = ? AND user_id = ?", eventID, userID).
				Count(&userVoteCount).Error; err != nil {
				return err
			}
			if userVoteCount > 0 {
				return ErrAlreadyVotedInEvent
			}
		}

		candidate.EventID = eventID
		candidate.AddedByUserID = userID
		candidate.VoteCount = 1
		if err := tx.Create(candidate).Error; err != nil {
			return err
		}

		vote := model.DomainEventVote{
			EventID:     eventID,
			CandidateID: candidate.ID,
			UserID:      userID,
		}
		return tx.Create(&vote).Error
	})
}

func (r *domainEventRepository) CastVote(eventID, candidateID, userID uint64, allowMultiVote bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var cand model.DomainEventCandidate
		if err := tx.Where("id = ? AND event_id = ?", candidateID, eventID).First(&cand).Error; err != nil {
			return err
		}

		if !allowMultiVote {
			var userVoteCount int64
			if err := tx.Model(&model.DomainEventVote{}).
				Where("event_id = ? AND user_id = ?", eventID, userID).
				Count(&userVoteCount).Error; err != nil {
				return err
			}
			if userVoteCount > 0 {
				return ErrAlreadyVotedInEvent
			}
		} else {
			var candVoteCount int64
			if err := tx.Model(&model.DomainEventVote{}).
				Where("event_id = ? AND candidate_id = ? AND user_id = ?", eventID, candidateID, userID).
				Count(&candVoteCount).Error; err != nil {
				return err
			}
			if candVoteCount > 0 {
				return ErrAlreadyVotedCandidate
			}
		}

		vote := model.DomainEventVote{
			EventID:     eventID,
			CandidateID: candidateID,
			UserID:      userID,
		}
		if err := tx.Create(&vote).Error; err != nil {
			return err
		}

		return tx.Model(&model.DomainEventCandidate{}).
			Where("id = ? AND event_id = ?", candidateID, eventID).
			UpdateColumn("vote_count", gorm.Expr("vote_count + 1")).Error
	})
}

func (r *domainEventRepository) CancelVote(eventID, candidateID, userID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("event_id = ? AND candidate_id = ? AND user_id = ?", eventID, candidateID, userID).
			Delete(&model.DomainEventVote{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return tx.Model(&model.DomainEventCandidate{}).
			Where("id = ? AND event_id = ?", candidateID, eventID).
			UpdateColumn("vote_count", gorm.Expr("CASE WHEN vote_count > 0 THEN vote_count - 1 ELSE 0 END")).Error
	})
}

func (r *domainEventRepository) ListUserVotedCandidateIDs(eventID, userID uint64) ([]uint64, error) {
	var ids []uint64
	if err := r.db.Model(&model.DomainEventVote{}).
		Where("event_id = ? AND user_id = ?", eventID, userID).
		Pluck("candidate_id", &ids).Error; err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []uint64{}
	}
	return ids, nil
}
