package repository

import (
	"errors"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KBInteractionRepository interface {
	GetLikeStats(docID, userID uint64) (likeCount int64, likedByMe bool, likeUserIDs []uint64, err error)
	ToggleLike(kbAssetID, docID, userID uint64) (liked bool, err error)
	IsFavorited(docID, userID uint64) (bool, error)
	ToggleFavorite(kbAssetID, docID, userID uint64) (favorited bool, err error)
	ListUserFavorites(userID uint64) ([]model.UserFavoriteItem, error)
	CountComments(docID uint64) (int64, error)
	ListComments(docID uint64) ([]model.KBDocumentComment, error)
	CreateComment(comment *model.KBDocumentComment) error
	FindCommentByID(kbAssetID, docID, commentID uint64) (*model.KBDocumentComment, error)
	DeleteComment(commentID uint64) error
	DeleteByDocIDs(docIDs []uint64) error
}

type kbInteractionRepository struct {
	db *gorm.DB
}

func NewKBInteractionRepository(db *gorm.DB) KBInteractionRepository {
	return &kbInteractionRepository{db: db}
}

func (r *kbInteractionRepository) GetLikeStats(docID, userID uint64) (int64, bool, []uint64, error) {
	var likes []model.KBDocumentLike
	if err := r.db.Where("doc_id = ?", docID).Order("created_at ASC, id ASC").Find(&likes).Error; err != nil {
		return 0, false, nil, err
	}
	userIDs := make([]uint64, 0, len(likes))
	likedByMe := false
	for _, l := range likes {
		userIDs = append(userIDs, l.UserID)
		if l.UserID == userID {
			likedByMe = true
		}
	}
	return int64(len(likes)), likedByMe, userIDs, nil
}

func (r *kbInteractionRepository) ToggleLike(kbAssetID, docID, userID uint64) (bool, error) {
	var liked bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.KBDocumentLike
		err := tx.Where("doc_id = ? AND user_id = ?", docID, userID).First(&existing).Error
		if err == nil {
			if delErr := tx.Delete(&existing).Error; delErr != nil {
				return delErr
			}
			liked = false
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		rec := model.KBDocumentLike{
			KBAssetID: kbAssetID,
			DocID:     docID,
			UserID:    userID,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rec).Error; err != nil {
			return err
		}
		liked = true
		return nil
	})
	return liked, err
}

func (r *kbInteractionRepository) IsFavorited(docID, userID uint64) (bool, error) {
	var count int64
	if err := r.db.Model(&model.KBDocumentFavorite{}).
		Where("doc_id = ? AND user_id = ?", docID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *kbInteractionRepository) ToggleFavorite(kbAssetID, docID, userID uint64) (bool, error) {
	var favorited bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.KBDocumentFavorite
		err := tx.Where("doc_id = ? AND user_id = ?", docID, userID).First(&existing).Error
		if err == nil {
			if delErr := tx.Delete(&existing).Error; delErr != nil {
				return delErr
			}
			favorited = false
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		rec := model.KBDocumentFavorite{
			KBAssetID: kbAssetID,
			DocID:     docID,
			UserID:    userID,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rec).Error; err != nil {
			return err
		}
		favorited = true
		return nil
	})
	return favorited, err
}

func (r *kbInteractionRepository) ListUserFavorites(userID uint64) ([]model.UserFavoriteItem, error) {
	var items []model.UserFavoriteItem
	err := r.db.Table("kb_document_favorite AS f").
		Select("f.id, f.kb_asset_id, f.doc_id, a.domain_id, dom.domain_name, a.name AS kb_name, d.file_name, d.folder_path, d.file_ext, d.is_folder, f.created_at, dom.visibility AS domain_visibility, a.owner_id AS asset_owner_id, a.visibility AS asset_visibility, a.status AS asset_status").
		Joins("INNER JOIN kb_document d ON d.id = f.doc_id AND d.deleted_at IS NULL").
		Joins("INNER JOIN ai_asset a ON a.id = f.kb_asset_id AND a.deleted_at IS NULL").
		Joins("INNER JOIN biz_domain dom ON dom.id = a.domain_id AND dom.deleted_at IS NULL").
		Where("f.user_id = ?", userID).
		Order("f.created_at DESC").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.UserFavoriteItem{}
	}
	return items, nil
}

func (r *kbInteractionRepository) CountComments(docID uint64) (int64, error) {
	var count int64
	if err := r.db.Model(&model.KBDocumentComment{}).
		Where("doc_id = ?", docID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *kbInteractionRepository) ListComments(docID uint64) ([]model.KBDocumentComment, error) {
	var comments []model.KBDocumentComment
	if err := r.db.Where("doc_id = ?", docID).
		Order("created_at ASC, id ASC").
		Find(&comments).Error; err != nil {
		return nil, err
	}
	if comments == nil {
		comments = []model.KBDocumentComment{}
	}
	return comments, nil
}

func (r *kbInteractionRepository) CreateComment(comment *model.KBDocumentComment) error {
	return r.db.Create(comment).Error
}

func (r *kbInteractionRepository) FindCommentByID(kbAssetID, docID, commentID uint64) (*model.KBDocumentComment, error) {
	var comment model.KBDocumentComment
	if err := r.db.Where("id = ? AND kb_asset_id = ? AND doc_id = ?", commentID, kbAssetID, docID).
		First(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *kbInteractionRepository) DeleteComment(commentID uint64) error {
	return r.db.Delete(&model.KBDocumentComment{}, commentID).Error
}

func (r *kbInteractionRepository) DeleteByDocIDs(docIDs []uint64) error {
	if len(docIDs) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("doc_id IN ?", docIDs).Delete(&model.KBDocumentLike{}).Error; err != nil {
			return err
		}
		if err := tx.Where("doc_id IN ?", docIDs).Delete(&model.KBDocumentFavorite{}).Error; err != nil {
			return err
		}
		return tx.Where("doc_id IN ?", docIDs).Delete(&model.KBDocumentComment{}).Error
	})
}
