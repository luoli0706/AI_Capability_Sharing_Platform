package repository

import (
	"strings"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrgRepository interface {
	ListOrgs(userID uint64, q model.OrgQuery) ([]model.OrgListItem, int64, error)
	FindByID(id uint64) (*model.BizOrganization, error)
	FindByCode(code string) (*model.BizOrganization, error)
	CreateWithOwner(org *model.BizOrganization) error
	UpdateOrg(org *model.BizOrganization) error
	DeleteOrg(id uint64) error
	ListMembers(orgID uint64) ([]model.OrgMemberItem, error)
	GetMemberRole(orgID, userID uint64) (string, error)
	UpsertMember(member *model.OrganizationMember) error
	RemoveMember(orgID, userID uint64) error
	GetUserOrgIDs(userID uint64) ([]uint64, error)
	GetUserAdminOrgIDs(userID uint64) ([]uint64, error)
	ListPublicDomainsByOrg(orgID, userID uint64) ([]model.DomainListItem, error)
}

type orgRepository struct {
	db *gorm.DB
}

func NewOrgRepository(db *gorm.DB) OrgRepository {
	return &orgRepository{db: db}
}

func (r *orgRepository) ListOrgs(userID uint64, q model.OrgQuery) ([]model.OrgListItem, int64, error) {
	tx := r.db.Model(&model.BizOrganization{})
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("(org_name LIKE ? OR org_code LIKE ?)", like, like)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var orgs []model.BizOrganization
	offset := (page - 1) * pageSize
	if err := tx.Order("id DESC").Offset(offset).Limit(pageSize).Find(&orgs).Error; err != nil {
		return nil, 0, err
	}
	if len(orgs) == 0 {
		return []model.OrgListItem{}, total, nil
	}

	type countRow struct {
		ID  uint64
		Cnt int64
	}
	var memberCounts []countRow
	if err := r.db.Model(&model.OrganizationMember{}).
		Select("org_id AS id, COUNT(*) AS cnt").
		Group("org_id").
		Scan(&memberCounts).Error; err != nil {
		return nil, 0, err
	}
	memberMap := make(map[uint64]int64, len(memberCounts))
	for _, c := range memberCounts {
		memberMap[c.ID] = c.Cnt
	}

	var domainCounts []countRow
	if err := r.db.Model(&model.BizDomain{}).
		Select("org_id AS id, COUNT(*) AS cnt").
		Where("org_id > 0 AND visibility = ?", model.DomainVisibilityPublic).
		Group("org_id").
		Scan(&domainCounts).Error; err != nil {
		return nil, 0, err
	}
	domainMap := make(map[uint64]int64, len(domainCounts))
	for _, c := range domainCounts {
		domainMap[c.ID] = c.Cnt
	}

	var myMemberships []model.OrganizationMember
	if err := r.db.Where("user_id = ?", userID).Find(&myMemberships).Error; err != nil {
		return nil, 0, err
	}
	roleMap := make(map[uint64]string, len(myMemberships))
	for _, m := range myMemberships {
		roleMap[m.OrgID] = m.OrgRole
	}

	items := make([]model.OrgListItem, 0, len(orgs))
	for _, o := range orgs {
		items = append(items, model.OrgListItem{
			ID:          o.ID,
			OrgCode:     o.OrgCode,
			OrgName:     o.OrgName,
			Description: o.Description,
			OwnerID:     o.OwnerID,
			Status:      o.Status,
			MyRole:      roleMap[o.ID],
			MemberCount: memberMap[o.ID],
			DomainCount: domainMap[o.ID],
			CreatedAt:   o.CreatedAt,
		})
	}
	return items, total, nil
}

func (r *orgRepository) FindByID(id uint64) (*model.BizOrganization, error) {
	var org model.BizOrganization
	if err := r.db.First(&org, id).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *orgRepository) FindByCode(code string) (*model.BizOrganization, error) {
	var org model.BizOrganization
	if err := r.db.Where("org_code = ?", code).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *orgRepository) CreateWithOwner(org *model.BizOrganization) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(org).Error; err != nil {
			return err
		}
		member := model.OrganizationMember{
			OrgID:   org.ID,
			UserID:  org.OwnerID,
			OrgRole: model.OrgRoleOwner,
		}
		return tx.Create(&member).Error
	})
}

func (r *orgRepository) UpdateOrg(org *model.BizOrganization) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(org).Error; err != nil {
			return err
		}
		return tx.Model(&model.BizDomain{}).
			Where("org_id = ?", org.ID).
			Update("org_name", org.OrgName).Error
	})
}

func (r *orgRepository) DeleteOrg(id uint64) error {
	return r.db.Delete(&model.BizOrganization{}, id).Error
}

func (r *orgRepository) ListMembers(orgID uint64) ([]model.OrgMemberItem, error) {
	var items []model.OrgMemberItem
	err := r.db.Table("organization_member").
		Select("organization_member.user_id, sys_user.username, sys_user.nickname, sys_user.email, organization_member.org_role, organization_member.joined_at").
		Joins("INNER JOIN sys_user ON sys_user.id = organization_member.user_id AND sys_user.deleted_at IS NULL").
		Where("organization_member.org_id = ?", orgID).
		Order("organization_member.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.OrgMemberItem{}
	}
	return items, nil
}

func (r *orgRepository) GetMemberRole(orgID, userID uint64) (string, error) {
	var member model.OrganizationMember
	err := r.db.Where("org_id = ? AND user_id = ?", orgID, userID).First(&member).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return member.OrgRole, nil
}

func (r *orgRepository) UpsertMember(member *model.OrganizationMember) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"org_role", "updated_at"}),
	}).Create(member).Error
}

func (r *orgRepository) RemoveMember(orgID, userID uint64) error {
	res := r.db.Where("org_id = ? AND user_id = ?", orgID, userID).Delete(&model.OrganizationMember{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *orgRepository) GetUserOrgIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.Model(&model.OrganizationMember{}).
		Where("user_id = ?", userID).
		Pluck("org_id", &ids).Error
	return ids, err
}

func (r *orgRepository) GetUserAdminOrgIDs(userID uint64) ([]uint64, error) {
	var memberIDs []uint64
	if err := r.db.Model(&model.OrganizationMember{}).
		Where("user_id = ? AND org_role IN ?", userID, []string{model.OrgRoleOwner, model.OrgRoleAdmin}).
		Pluck("org_id", &memberIDs).Error; err != nil {
		return nil, err
	}
	var ownerIDs []uint64
	if err := r.db.Model(&model.BizOrganization{}).
		Where("owner_id = ?", userID).
		Pluck("id", &ownerIDs).Error; err != nil {
		return nil, err
	}
	seen := make(map[uint64]bool, len(memberIDs)+len(ownerIDs))
	ids := make([]uint64, 0, len(memberIDs)+len(ownerIDs))
	for _, id := range append(memberIDs, ownerIDs...) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *orgRepository) ListPublicDomainsByOrg(orgID, userID uint64) ([]model.DomainListItem, error) {
	var domains []model.BizDomain
	if err := r.db.Where("org_id = ? AND visibility = ?", orgID, model.DomainVisibilityPublic).
		Order("id DESC").Find(&domains).Error; err != nil {
		return nil, err
	}
	orgIDs, err := r.GetUserOrgIDs(userID)
	if err != nil {
		return nil, err
	}
	adminOrgIDs, err := r.GetUserAdminOrgIDs(userID)
	if err != nil {
		return nil, err
	}
	return hydrateDomainListItems(r.db, domains, userID, orgIDs, adminOrgIDs)
}
