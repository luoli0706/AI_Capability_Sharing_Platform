package service

import (
	"errors"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

// hasDomainAccess checks if userID is an explicit member of domainID OR
// has organization-derived access (all org members for public org domains;
// org owner/admin only for org_private domains).
func hasDomainAccess(
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	domainID, userID uint64,
) (bool, error) {
	role, err := domainRepo.GetMemberRole(domainID, userID)
	if err != nil {
		return false, err
	}
	if role != "" {
		return true, nil
	}
	if orgRepo == nil {
		return false, nil
	}
	domain, err := domainRepo.FindByID(domainID)
	if err != nil || domain.OrgID == 0 {
		return false, nil
	}
	orgRole, err := orgRepo.GetMemberRole(domain.OrgID, userID)
	if err != nil {
		return false, err
	}
	switch domain.Visibility {
	case model.DomainVisibilityPublic:
		return orgRole != "", nil
	case model.DomainVisibilityOrgPrivate:
		return orgRole == model.OrgRoleOwner || orgRole == model.OrgRoleAdmin, nil
	default:
		return false, nil
	}
}

func getAccessibleDomainIDs(
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	userID uint64,
) ([]uint64, error) {
	var userOrgIDs, adminOrgIDs []uint64
	if orgRepo != nil {
		var err error
		userOrgIDs, err = orgRepo.GetUserOrgIDs(userID)
		if err != nil {
			return nil, err
		}
		adminOrgIDs, err = orgRepo.GetUserAdminOrgIDs(userID)
		if err != nil {
			return nil, err
		}
	}
	return domainRepo.GetAccessibleDomainIDs(userID, userOrgIDs, adminOrgIDs)
}

// canAccessKBAsset checks whether viewerID can read a KB asset given pre-fetched
// domain and asset permission sets, avoiding N+1 queries.
func canAccessKBAsset(
	viewerID uint64,
	isSuperAdmin bool,
	domainID uint64,
	domainVisibility string,
	assetID uint64,
	assetOwnerID uint64,
	assetVisibility string,
	assetStatus string,
	accessibleDomainSet map[uint64]bool,
	adminDomainSet map[uint64]bool,
	permittedAssetSet map[uint64]bool,
) bool {
	if isSuperAdmin || assetOwnerID == viewerID || adminDomainSet[domainID] {
		return true
	}
	if assetStatus != model.AssetStatusPublished {
		return false
	}
	switch assetVisibility {
	case model.VisibilityPublic:
		return domainVisibility == model.DomainVisibilityPublic || accessibleDomainSet[domainID] || permittedAssetSet[assetID]
	case model.VisibilityDomain:
		return accessibleDomainSet[domainID] || permittedAssetSet[assetID]
	case model.VisibilityPrivate:
		return permittedAssetSet[assetID]
	default:
		return false
	}
}

type assetDocAccessDetail struct {
	Result *model.AssetAccessCheckResult
	Asset  *model.AssetDetail
	Doc    *model.KBDocument
}

// checkAssetAndDocAccess verifies whether userID has read access to assetID (and docID if > 0),
// including cross-reference fallback when refDomainID > 0.
func checkAssetAndDocAccess(
	assetRepo repository.AssetRepository,
	kbRepo repository.KBRepository,
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	govRepo repository.GovernanceRepository,
	crossRefRepo repository.CrossRefRepository,
	userID uint64,
	isSuperAdmin bool,
	assetID, docID, refDomainID uint64,
) (*assetDocAccessDetail, error) {
	detail, err := assetRepo.GetDetailByID(assetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &assetDocAccessDetail{
				Result: &model.AssetAccessCheckResult{
					Accessible: false,
					Reason:     "资产不存在或已被删除",
				},
			}, nil
		}
		return nil, err
	}

	allowed := false
	if isSuperAdmin || detail.OwnerID == userID {
		allowed = true
	} else {
		role, err := domainRepo.GetMemberRole(detail.DomainID, userID)
		if err != nil {
			return nil, err
		}
		if role == model.DomainRoleOwner || role == model.DomainRoleAdmin {
			allowed = true
		} else if detail.Status == model.AssetStatusPublished {
			switch detail.Visibility {
			case model.VisibilityPublic:
				allowed = true
			case model.VisibilityDomain:
				ok, err := hasDomainAccess(domainRepo, orgRepo, detail.DomainID, userID)
				if err != nil {
					return nil, err
				}
				allowed = ok
			}
			if !allowed && govRepo != nil {
				if perm, _ := govRepo.GetActiveUserPermission(assetID, userID); perm != nil {
					allowed = true
				}
			}
		}
	}

	if !allowed && refDomainID > 0 && crossRefRepo != nil {
		hasTargetAccess, err := hasDomainAccess(domainRepo, orgRepo, refDomainID, userID)
		if err != nil {
			return nil, err
		}
		if hasTargetAccess {
			var hasRef bool
			if docID == 0 {
				hasRef, err = crossRefRepo.HasActiveAssetRef(refDomainID, assetID)
			} else {
				hasRef, err = crossRefRepo.HasActiveDocOrAssetRef(refDomainID, assetID, docID)
			}
			if err != nil {
				return nil, err
			}
			if hasRef {
				allowed = true
			}
		}
	}

	if !allowed {
		return &assetDocAccessDetail{
			Result: &model.AssetAccessCheckResult{
				Accessible: false,
				Reason:     "无权访问该资产",
				AssetType:  detail.AssetType,
				DomainID:   detail.DomainID,
			},
			Asset: detail,
		}, nil
	}

	var doc *model.KBDocument
	if docID > 0 {
		if detail.AssetType != model.AssetTypeKB {
			return &assetDocAccessDetail{
				Result: &model.AssetAccessCheckResult{
					Accessible: false,
					Reason:     "该资产不是知识库",
					AssetType:  detail.AssetType,
					DomainID:   detail.DomainID,
				},
				Asset: detail,
			}, nil
		}
		if kbRepo != nil {
			foundDoc, err := kbRepo.FindDocumentByID(assetID, docID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &assetDocAccessDetail{
						Result: &model.AssetAccessCheckResult{
							Accessible: false,
							Reason:     "文档不存在或已被删除",
							AssetType:  detail.AssetType,
							DomainID:   detail.DomainID,
						},
						Asset: detail,
					}, nil
				}
				return nil, err
			}
			doc = foundDoc
		}
	}

	return &assetDocAccessDetail{
		Result: &model.AssetAccessCheckResult{
			Accessible: true,
			Reason:     "",
			AssetType:  detail.AssetType,
			DomainID:   detail.DomainID,
		},
		Asset: detail,
		Doc:   doc,
	}, nil
}
