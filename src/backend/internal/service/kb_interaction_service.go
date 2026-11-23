package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

const (
	maxKBCommentRunes = 4000
	maxKBQuoteRunes   = 500
)

type KBInteractionService interface {
	SetCrossRefRepo(crossRefRepo repository.CrossRefRepository)
	GetInteractions(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBDocumentInteractionsResponse, error)
	ToggleLike(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBDocumentLikeToggleResponse, error)
	ToggleFavorite(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (bool, error)
	ListComments(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) ([]model.KBCommentItem, error)
	CreateComment(userID uint64, isSuperAdmin bool, kbID, docID uint64, req model.CreateKBCommentRequest, refDomainID uint64) (*model.KBCommentItem, error)
	DeleteComment(userID uint64, isSuperAdmin bool, kbID, docID, commentID, refDomainID uint64) error
	GetReadingProgress(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.ReadingProgressResponse, error)
	SaveReadingProgress(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64, req model.UpdateReadingProgressRequest) (*model.ReadingProgressResponse, error)
}

type kbInteractionService struct {
	interactionRepo repository.KBInteractionRepository
	kbRepo          repository.KBRepository
	assetRepo       repository.AssetRepository
	domainRepo      repository.DomainRepository
	govRepo         repository.GovernanceRepository
	orgRepo         repository.OrgRepository
	userRepo        repository.UserRepository
	crossRefRepo    repository.CrossRefRepository
}

func (s *kbInteractionService) SetCrossRefRepo(crossRefRepo repository.CrossRefRepository) {
	s.crossRefRepo = crossRefRepo
}

func NewKBInteractionService(
	interactionRepo repository.KBInteractionRepository,
	kbRepo repository.KBRepository,
	assetRepo repository.AssetRepository,
	domainRepo repository.DomainRepository,
	govRepo repository.GovernanceRepository,
	orgRepo repository.OrgRepository,
	userRepo repository.UserRepository,
) KBInteractionService {
	return &kbInteractionService{
		interactionRepo: interactionRepo,
		kbRepo:          kbRepo,
		assetRepo:       assetRepo,
		domainRepo:      domainRepo,
		govRepo:         govRepo,
		orgRepo:         orgRepo,
		userRepo:        userRepo,
	}
}

func (s *kbInteractionService) GetInteractions(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBDocumentInteractionsResponse, error) {
	if _, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, true); err != nil {
		return nil, err
	}

	likeCount, likedByMe, likeUserIDs, err := s.interactionRepo.GetLikeStats(docID, userID)
	if err != nil {
		return nil, err
	}
	likeUsers, err := s.loadOrderedUserBriefs(likeUserIDs)
	if err != nil {
		return nil, err
	}

	favoritedByMe, err := s.interactionRepo.IsFavorited(docID, userID)
	if err != nil {
		return nil, err
	}

	commentCount, err := s.interactionRepo.CountComments(docID)
	if err != nil {
		return nil, err
	}

	return &model.KBDocumentInteractionsResponse{
		LikeCount:     likeCount,
		LikedByMe:     likedByMe,
		LikeUsers:     likeUsers,
		FavoritedByMe: favoritedByMe,
		CommentCount:  commentCount,
	}, nil
}

func (s *kbInteractionService) ToggleLike(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBDocumentLikeToggleResponse, error) {
	if _, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, false); err != nil {
		return nil, err
	}

	if _, err := s.interactionRepo.ToggleLike(kbID, docID, userID); err != nil {
		return nil, err
	}

	likeCount, likedByMe, likeUserIDs, err := s.interactionRepo.GetLikeStats(docID, userID)
	if err != nil {
		return nil, err
	}
	likeUsers, err := s.loadOrderedUserBriefs(likeUserIDs)
	if err != nil {
		return nil, err
	}

	return &model.KBDocumentLikeToggleResponse{
		LikeCount: likeCount,
		LikedByMe: likedByMe,
		LikeUsers: likeUsers,
	}, nil
}

func (s *kbInteractionService) ToggleFavorite(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (bool, error) {
	if _, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, true); err != nil {
		return false, err
	}
	return s.interactionRepo.ToggleFavorite(kbID, docID, userID)
}

func (s *kbInteractionService) ListComments(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) ([]model.KBCommentItem, error) {
	detail, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, false)
	if err != nil {
		return nil, err
	}

	comments, err := s.interactionRepo.ListComments(docID)
	if err != nil {
		return nil, err
	}
	if len(comments) == 0 {
		return []model.KBCommentItem{}, nil
	}

	authorIDs := make([]uint64, 0, len(comments))
	seen := make(map[uint64]struct{}, len(comments))
	for _, cm := range comments {
		if _, ok := seen[cm.UserID]; !ok {
			seen[cm.UserID] = struct{}{}
			authorIDs = append(authorIDs, cm.UserID)
		}
	}

	briefs, err := s.userRepo.FindBriefsByIDs(authorIDs)
	if err != nil {
		return nil, err
	}

	isDomainAdminOrOwner, err := s.isAssetOrDomainAdmin(userID, isSuperAdmin, detail)
	if err != nil {
		return nil, err
	}

	items := make([]model.KBCommentItem, 0, len(comments))
	for _, cm := range comments {
		author := briefs[cm.UserID]
		if author.ID == 0 {
			author.ID = cm.UserID
		}
		items = append(items, model.KBCommentItem{
			ID:        cm.ID,
			KBAssetID: cm.KBAssetID,
			DocID:     cm.DocID,
			UserID:    cm.UserID,
			Author:    author,
			ParentID:  cm.ParentID,
			QuoteText: cm.QuoteText,
			Content:   cm.Content,
			CanDelete: isDomainAdminOrOwner || cm.UserID == userID,
			CreatedAt: cm.CreatedAt,
			UpdatedAt: cm.UpdatedAt,
		})
	}
	return items, nil
}

func (s *kbInteractionService) CreateComment(userID uint64, isSuperAdmin bool, kbID, docID uint64, req model.CreateKBCommentRequest, refDomainID uint64) (*model.KBCommentItem, error) {
	if _, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, false); err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, NewBadRequest("comment content cannot be empty")
	}
	if utf8.RuneCountInString(content) > maxKBCommentRunes {
		return nil, NewBadRequest("comment content exceeds 4000 characters")
	}

	quoteText := strings.TrimSpace(req.QuoteText)
	if utf8.RuneCountInString(quoteText) > maxKBQuoteRunes {
		return nil, NewBadRequest("quote_text exceeds 500 characters")
	}

	if req.ParentID != nil && *req.ParentID > 0 {
		if _, err := s.interactionRepo.FindCommentByID(kbID, docID, *req.ParentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("parent comment not found")
			}
			return nil, err
		}
	}

	comment := &model.KBDocumentComment{
		KBAssetID: kbID,
		DocID:     docID,
		UserID:    userID,
		ParentID:  req.ParentID,
		QuoteText: quoteText,
		Content:   content,
	}
	if err := s.interactionRepo.CreateComment(comment); err != nil {
		return nil, err
	}

	var author model.UserBrief
	if user, err := s.userRepo.FindByID(userID); err == nil {
		author = model.ToUserBrief(user)
	} else {
		author = model.UserBrief{ID: userID}
	}

	return &model.KBCommentItem{
		ID:        comment.ID,
		KBAssetID: comment.KBAssetID,
		DocID:     comment.DocID,
		UserID:    comment.UserID,
		Author:    author,
		ParentID:  comment.ParentID,
		QuoteText: comment.QuoteText,
		Content:   comment.Content,
		CanDelete: true,
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	}, nil
}

func (s *kbInteractionService) DeleteComment(userID uint64, isSuperAdmin bool, kbID, docID, commentID, refDomainID uint64) error {
	detail, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, false)
	if err != nil {
		return err
	}

	comment, err := s.interactionRepo.FindCommentByID(kbID, docID, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("comment not found")
		}
		return err
	}

	if comment.UserID != userID {
		isAdmin, err := s.isAssetOrDomainAdmin(userID, isSuperAdmin, detail)
		if err != nil {
			return err
		}
		if !isAdmin {
			return NewForbidden("only comment author or domain admin can delete this comment")
		}
	}

	return s.interactionRepo.DeleteComment(comment.ID)
}

func (s *kbInteractionService) GetReadingProgress(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.ReadingProgressResponse, error) {
	if _, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, false); err != nil {
		return nil, err
	}
	rec, err := s.interactionRepo.GetReadingProgress(docID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.ReadingProgressResponse{
				CurrentPage: 1,
				TotalPages:  0,
				UpdatedAt:   nil,
			}, nil
		}
		return nil, err
	}
	updatedAt := rec.UpdatedAt
	return &model.ReadingProgressResponse{
		CurrentPage: rec.CurrentPage,
		TotalPages:  rec.TotalPages,
		UpdatedAt:   &updatedAt,
	}, nil
}

func (s *kbInteractionService) SaveReadingProgress(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64, req model.UpdateReadingProgressRequest) (*model.ReadingProgressResponse, error) {
	if _, _, err := s.ensureKBDocViewAccess(userID, isSuperAdmin, kbID, docID, refDomainID, false); err != nil {
		return nil, err
	}
	totalPages := req.TotalPages
	if totalPages < 1 {
		totalPages = 1
	}
	currentPage := req.CurrentPage
	if currentPage < 1 {
		currentPage = 1
	}
	if currentPage > totalPages {
		currentPage = totalPages
	}
	rec, err := s.interactionRepo.UpsertReadingProgress(kbID, docID, userID, currentPage, totalPages)
	if err != nil {
		return nil, err
	}
	updatedAt := rec.UpdatedAt
	return &model.ReadingProgressResponse{
		CurrentPage: rec.CurrentPage,
		TotalPages:  rec.TotalPages,
		UpdatedAt:   &updatedAt,
	}, nil
}

func (s *kbInteractionService) ensureKBDocViewAccess(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64, allowFolder bool) (*model.AssetDetail, *model.KBDocument, error) {
	detail, err := ensureKBViewAccessWithCrossRef(
		s.assetRepo, s.domainRepo, s.orgRepo, s.govRepo, s.crossRefRepo,
		userID, isSuperAdmin, kbID, refDomainID, docID,
	)
	if err != nil {
		return nil, nil, err
	}

	doc, err := s.kbRepo.FindDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, NewNotFound("document not found")
		}
		return nil, nil, err
	}
	if doc.IsFolder && !allowFolder {
		return nil, nil, NewBadRequest("interactions are only supported on documents")
	}
	return detail, doc, nil
}

func (s *kbInteractionService) isAssetOrDomainAdmin(userID uint64, isSuperAdmin bool, detail *model.AssetDetail) (bool, error) {
	if isSuperAdmin || detail.OwnerID == userID {
		return true, nil
	}
	role, err := s.domainRepo.GetMemberRole(detail.DomainID, userID)
	if err != nil {
		return false, err
	}
	return role == model.DomainRoleOwner || role == model.DomainRoleAdmin, nil
}

func (s *kbInteractionService) loadOrderedUserBriefs(userIDs []uint64) ([]model.UserBrief, error) {
	if len(userIDs) == 0 {
		return []model.UserBrief{}, nil
	}
	briefs, err := s.userRepo.FindBriefsByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	out := make([]model.UserBrief, 0, len(userIDs))
	for _, uid := range userIDs {
		if b, ok := briefs[uid]; ok {
			out = append(out, b)
		} else {
			out = append(out, model.UserBrief{ID: uid})
		}
	}
	return out, nil
}
