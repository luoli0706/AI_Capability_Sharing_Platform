package service

import (
	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"
)

// hasDomainAccess checks if userID is an explicit member of domainID OR
// if domainID is a public domain belonging to an organization the user is a member of.
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
	if err != nil {
		return false, nil
	}
	if domain.Visibility == model.DomainVisibilityPublic && domain.OrgID > 0 {
		orgRole, err := orgRepo.GetMemberRole(domain.OrgID, userID)
		if err != nil {
			return false, err
		}
		return orgRole != "", nil
	}
	return false, nil
}

func getAccessibleDomainIDs(
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	userID uint64,
) ([]uint64, error) {
	var orgIDs []uint64
	if orgRepo != nil {
		var err error
		orgIDs, err = orgRepo.GetUserOrgIDs(userID)
		if err != nil {
			return nil, err
		}
	}
	return domainRepo.GetAccessibleDomainIDs(userID, orgIDs)
}
