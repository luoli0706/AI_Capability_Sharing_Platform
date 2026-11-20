package service

import (
	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"
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
