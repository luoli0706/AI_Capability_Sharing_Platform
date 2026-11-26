package service

import (
	"errors"
	"strings"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type DomainEventService interface {
	CreateEvent(userID uint64, isSuperAdmin bool, domainID uint64, req model.CreateDomainEventRequest) (*model.DomainEventSummary, error)
	ListEvents(userID uint64, isSuperAdmin bool, domainID uint64, statusFilter, eventTypeFilter string) ([]model.DomainEventSummary, error)
	GetEventDetail(userID uint64, isSuperAdmin bool, domainID, eventID uint64) (*model.DomainEventDetail, error)
	AddCandidate(userID uint64, isSuperAdmin bool, domainID, eventID uint64, req model.AddDomainEventCandidateRequest) (*model.DomainEventDetail, error)
	VoteCandidate(userID uint64, isSuperAdmin bool, domainID, eventID, candidateID uint64) (*model.DomainEventDetail, error)
	UnvoteCandidate(userID uint64, isSuperAdmin bool, domainID, eventID, candidateID uint64) (*model.DomainEventDetail, error)
	CloseEvent(userID uint64, isSuperAdmin bool, domainID, eventID uint64) (*model.DomainEventSummary, error)
	DeleteEvent(userID uint64, isSuperAdmin bool, domainID, eventID uint64) error
}

type domainEventService struct {
	eventRepo  repository.DomainEventRepository
	domainRepo repository.DomainRepository
	orgRepo    repository.OrgRepository
	assetRepo  repository.AssetRepository
	kbRepo     repository.KBRepository
	userRepo   repository.UserRepository
}

func NewDomainEventService(
	eventRepo repository.DomainEventRepository,
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	assetRepo repository.AssetRepository,
	kbRepo repository.KBRepository,
	userRepo repository.UserRepository,
) DomainEventService {
	return &domainEventService{
		eventRepo:  eventRepo,
		domainRepo: domainRepo,
		orgRepo:    orgRepo,
		assetRepo:  assetRepo,
		kbRepo:     kbRepo,
		userRepo:   userRepo,
	}
}

func (s *domainEventService) ensureDomainAccess(userID uint64, isSuperAdmin bool, domainID uint64) error {
	if _, err := s.domainRepo.FindByID(domainID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("domain not found")
		}
		return err
	}
	if isSuperAdmin {
		return nil
	}
	ok, err := hasDomainAccess(s.domainRepo, s.orgRepo, domainID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return NewForbidden("no access to this domain")
	}
	return nil
}

func (s *domainEventService) isDomainAdmin(userID uint64, isSuperAdmin bool, domainID uint64) (bool, error) {
	if isSuperAdmin {
		return true, nil
	}
	role, err := s.domainRepo.GetMemberRole(domainID, userID)
	if err != nil {
		return false, err
	}
	return role == model.DomainRoleOwner || role == model.DomainRoleAdmin, nil
}

func (s *domainEventService) resolveUserNames(userIDs []uint64) map[uint64]string {
	result := make(map[uint64]string)
	if s.userRepo == nil || len(userIDs) == 0 {
		return result
	}
	seen := make(map[uint64]struct{}, len(userIDs))
	unique := make([]uint64, 0, len(userIDs))
	for _, id := range userIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return result
	}
	briefs, err := s.userRepo.FindBriefsByIDs(unique)
	if err != nil {
		return result
	}
	for id, b := range briefs {
		name := strings.TrimSpace(b.Nickname)
		if name == "" {
			name = b.Username
		}
		result[id] = name
	}
	return result
}

func (s *domainEventService) CreateEvent(userID uint64, isSuperAdmin bool, domainID uint64, req model.CreateDomainEventRequest) (*model.DomainEventSummary, error) {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, NewBadRequest("title is required")
	}
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		eventType = model.DomainEventTypeFolderVote
	}
	if eventType != model.DomainEventTypeFolderVote {
		return nil, NewBadRequest("unsupported event type (only folder_vote is currently available)")
	}
	if !req.EndTime.After(req.StartTime) {
		return nil, NewBadRequest("end_time must be after start_time")
	}

	event := &model.DomainEvent{
		DomainID:       domainID,
		Title:          title,
		Description:    strings.TrimSpace(req.Description),
		EventType:      eventType,
		AllowMultiVote: req.AllowMultiVote,
		CreatorID:      userID,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
	}
	if err := s.eventRepo.CreateEvent(event); err != nil {
		return nil, err
	}

	names := s.resolveUserNames([]uint64{userID})
	return &model.DomainEventSummary{
		DomainEvent:    *event,
		Status:         event.ComputeStatus(time.Now()),
		CreatorName:    names[userID],
		CandidateCount: 0,
		TotalVotes:     0,
		CanManage:      true,
	}, nil
}

func (s *domainEventService) ListEvents(userID uint64, isSuperAdmin bool, domainID uint64, statusFilter, eventTypeFilter string) ([]model.DomainEventSummary, error) {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return nil, err
	}

	events, err := s.eventRepo.ListEventsByDomain(domainID)
	if err != nil {
		return nil, err
	}

	isAdmin, err := s.isDomainAdmin(userID, isSuperAdmin, domainID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	statusFilter = strings.TrimSpace(statusFilter)
	eventTypeFilter = strings.TrimSpace(eventTypeFilter)

	filtered := make([]model.DomainEvent, 0, len(events))
	eventIDs := make([]uint64, 0, len(events))
	creatorIDs := make([]uint64, 0, len(events))
	for _, ev := range events {
		if eventTypeFilter != "" && ev.EventType != eventTypeFilter {
			continue
		}
		st := ev.ComputeStatus(now)
		if statusFilter != "" && st != statusFilter {
			continue
		}
		filtered = append(filtered, ev)
		eventIDs = append(eventIDs, ev.ID)
		creatorIDs = append(creatorIDs, ev.CreatorID)
	}

	candCountMap, voteSumMap, err := s.eventRepo.GetEventCandidateAndVoteStats(eventIDs)
	if err != nil {
		return nil, err
	}
	names := s.resolveUserNames(creatorIDs)

	out := make([]model.DomainEventSummary, 0, len(filtered))
	for _, ev := range filtered {
		out = append(out, model.DomainEventSummary{
			DomainEvent:    ev,
			Status:         ev.ComputeStatus(now),
			CreatorName:    names[ev.CreatorID],
			CandidateCount: candCountMap[ev.ID],
			TotalVotes:     voteSumMap[ev.ID],
			CanManage:      isAdmin || ev.CreatorID == userID,
		})
	}
	return out, nil
}

func (s *domainEventService) GetEventDetail(userID uint64, isSuperAdmin bool, domainID, eventID uint64) (*model.DomainEventDetail, error) {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return nil, err
	}

	event, err := s.eventRepo.FindEventByID(domainID, eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("event not found")
		}
		return nil, err
	}

	candidates, err := s.eventRepo.ListCandidatesByEvent(eventID)
	if err != nil {
		return nil, err
	}
	votedIDs, err := s.eventRepo.ListUserVotedCandidateIDs(eventID, userID)
	if err != nil {
		return nil, err
	}
	isAdmin, err := s.isDomainAdmin(userID, isSuperAdmin, domainID)
	if err != nil {
		return nil, err
	}

	votedSet := make(map[uint64]bool, len(votedIDs))
	for _, id := range votedIDs {
		votedSet[id] = true
	}

	userIDs := make([]uint64, 0, 1+len(candidates))
	userIDs = append(userIDs, event.CreatorID)
	var totalVotes int64
	for _, c := range candidates {
		userIDs = append(userIDs, c.AddedByUserID)
		totalVotes += c.VoteCount
	}
	names := s.resolveUserNames(userIDs)

	candViews := make([]model.DomainEventCandidateView, 0, len(candidates))
	for _, c := range candidates {
		candViews = append(candViews, model.DomainEventCandidateView{
			DomainEventCandidate: c,
			AddedByName:          names[c.AddedByUserID],
			HasVoted:             votedSet[c.ID],
		})
	}

	return &model.DomainEventDetail{
		DomainEventSummary: model.DomainEventSummary{
			DomainEvent:    *event,
			Status:         event.ComputeStatus(time.Now()),
			CreatorName:    names[event.CreatorID],
			CandidateCount: int64(len(candidates)),
			TotalVotes:     totalVotes,
			CanManage:      isAdmin || event.CreatorID == userID,
		},
		Candidates:          candViews,
		MyVotedCandidateIDs: votedIDs,
	}, nil
}

func (s *domainEventService) loadActiveFolderVoteEvent(domainID, eventID uint64) (*model.DomainEvent, error) {
	event, err := s.eventRepo.FindEventByID(domainID, eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("event not found")
		}
		return nil, err
	}
	if event.EventType != model.DomainEventTypeFolderVote {
		return nil, NewBadRequest("unsupported event type for folder voting")
	}
	if event.ComputeStatus(time.Now()) != model.DomainEventStatusActive {
		return nil, NewBadRequest("event is not active")
	}
	return event, nil
}

func (s *domainEventService) AddCandidate(userID uint64, isSuperAdmin bool, domainID, eventID uint64, req model.AddDomainEventCandidateRequest) (*model.DomainEventDetail, error) {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return nil, err
	}

	event, err := s.loadActiveFolderVoteEvent(domainID, eventID)
	if err != nil {
		return nil, err
	}

	kbAsset, err := s.assetRepo.GetDetailByID(req.KBAssetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("knowledge base not found")
		}
		return nil, err
	}
	if kbAsset.AssetType != model.AssetTypeKB {
		return nil, NewBadRequest("asset is not a knowledge base")
	}
	if kbAsset.DomainID != domainID {
		return nil, NewForbidden("knowledge base does not belong to this domain")
	}

	folderDoc, err := s.kbRepo.FindDocumentByID(req.KBAssetID, req.FolderDocID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("folder not found")
		}
		return nil, err
	}
	if !folderDoc.IsFolder {
		return nil, NewBadRequest("target document is not a folder")
	}

	folderName := strings.TrimSpace(folderDoc.FileName)
	if folderName == "" {
		folderName = folderDoc.FolderPath
	}

	candidate := &model.DomainEventCandidate{
		EventID:       eventID,
		KBAssetID:     req.KBAssetID,
		KBName:        kbAsset.Name,
		FolderDocID:   req.FolderDocID,
		FolderPath:    folderDoc.FolderPath,
		FolderName:    folderName,
		AddedByUserID: userID,
		VoteCount:     1,
	}
	if err := s.eventRepo.AddCandidateWithInitialVote(eventID, event.AllowMultiVote, candidate, userID); err != nil {
		switch {
		case errors.Is(err, repository.ErrCandidateExists):
			return nil, NewConflict("folder candidate already exists in this event")
		case errors.Is(err, repository.ErrAlreadyVotedInEvent):
			return nil, NewConflict("you have already voted in this single-vote event")
		default:
			return nil, err
		}
	}

	return s.GetEventDetail(userID, isSuperAdmin, domainID, eventID)
}

func (s *domainEventService) VoteCandidate(userID uint64, isSuperAdmin bool, domainID, eventID, candidateID uint64) (*model.DomainEventDetail, error) {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return nil, err
	}

	event, err := s.loadActiveFolderVoteEvent(domainID, eventID)
	if err != nil {
		return nil, err
	}

	if err := s.eventRepo.CastVote(eventID, candidateID, userID, event.AllowMultiVote); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, NewNotFound("candidate not found")
		case errors.Is(err, repository.ErrAlreadyVotedInEvent):
			return nil, NewConflict("you have already voted in this single-vote event")
		case errors.Is(err, repository.ErrAlreadyVotedCandidate):
			return nil, NewConflict("you have already voted for this candidate")
		default:
			return nil, err
		}
	}

	return s.GetEventDetail(userID, isSuperAdmin, domainID, eventID)
}

func (s *domainEventService) UnvoteCandidate(userID uint64, isSuperAdmin bool, domainID, eventID, candidateID uint64) (*model.DomainEventDetail, error) {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return nil, err
	}

	if _, err := s.loadActiveFolderVoteEvent(domainID, eventID); err != nil {
		return nil, err
	}

	if err := s.eventRepo.CancelVote(eventID, candidateID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("vote not found")
		}
		return nil, err
	}

	return s.GetEventDetail(userID, isSuperAdmin, domainID, eventID)
}

func (s *domainEventService) CloseEvent(userID uint64, isSuperAdmin bool, domainID, eventID uint64) (*model.DomainEventSummary, error) {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return nil, err
	}

	event, err := s.eventRepo.FindEventByID(domainID, eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("event not found")
		}
		return nil, err
	}

	isAdmin, err := s.isDomainAdmin(userID, isSuperAdmin, domainID)
	if err != nil {
		return nil, err
	}
	if !isAdmin && event.CreatorID != userID {
		return nil, NewForbidden("only event creator or domain admin can close this event")
	}

	if event.ClosedAt == nil {
		now := time.Now()
		if err := s.eventRepo.CloseEvent(domainID, eventID, now); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("event not found")
			}
			return nil, err
		}
		event.ClosedAt = &now
	}

	candCountMap, voteSumMap, err := s.eventRepo.GetEventCandidateAndVoteStats([]uint64{eventID})
	if err != nil {
		return nil, err
	}
	names := s.resolveUserNames([]uint64{event.CreatorID})

	return &model.DomainEventSummary{
		DomainEvent:    *event,
		Status:         event.ComputeStatus(time.Now()),
		CreatorName:    names[event.CreatorID],
		CandidateCount: candCountMap[eventID],
		TotalVotes:     voteSumMap[eventID],
		CanManage:      true,
	}, nil
}

func (s *domainEventService) DeleteEvent(userID uint64, isSuperAdmin bool, domainID, eventID uint64) error {
	if err := s.ensureDomainAccess(userID, isSuperAdmin, domainID); err != nil {
		return err
	}

	event, err := s.eventRepo.FindEventByID(domainID, eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("event not found")
		}
		return err
	}

	isAdmin, err := s.isDomainAdmin(userID, isSuperAdmin, domainID)
	if err != nil {
		return err
	}
	if !isAdmin && event.CreatorID != userID {
		return NewForbidden("only event creator or domain admin can delete this event")
	}

	if err := s.eventRepo.DeleteEvent(domainID, eventID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("event not found")
		}
		return err
	}
	return nil
}
