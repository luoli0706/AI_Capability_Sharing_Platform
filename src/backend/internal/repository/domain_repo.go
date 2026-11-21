package repository

import (
	"fmt"
	"strings"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DomainRepository interface {
	ListDomains(userID uint64, q model.DomainQuery, userOrgIDs, adminOrgIDs []uint64) ([]model.DomainListItem, int64, error)
	FindByID(id uint64) (*model.BizDomain, error)
	FindByCode(code string) (*model.BizDomain, error)
	FindByUID(uid string) (*model.BizDomain, error)
	CreateWithOwner(domain *model.BizDomain) error
	UpdateDomain(domain *model.BizDomain) error
	DeleteDomain(id uint64) error
	ListMembers(domainID uint64) ([]model.DomainMemberItem, error)
	GetMemberRole(domainID, userID uint64) (string, error)
	UpsertMember(member *model.DomainMember) error
	RemoveMember(domainID, userID uint64) error
	GetUserDomainIDs(userID uint64) ([]uint64, error)
	GetUserAdminDomainIDs(userID uint64) ([]uint64, error)
	GetAccessibleDomainIDs(userID uint64, userOrgIDs, adminOrgIDs []uint64) ([]uint64, error)
}

type domainRepository struct {
	db *gorm.DB
}

func NewDomainRepository(db *gorm.DB) DomainRepository {
	return &domainRepository{db: db}
}

func (r *domainRepository) ListDomains(userID uint64, q model.DomainQuery, userOrgIDs, adminOrgIDs []uint64) ([]model.DomainListItem, int64, error) {
	tx := r.db.Model(&model.BizDomain{})
	if q.OrgID > 0 {
		tx = tx.Where("org_id = ?", q.OrgID)
	}

	kw := strings.TrimSpace(q.Keyword)
	uid := strings.TrimSpace(q.DomainUID)
	if uid == "" && strings.HasPrefix(strings.ToUpper(kw), "DOM-") {
		uid = strings.ToUpper(kw)
	}

	switch q.Tab {
	case "square":
		if uid != "" {
			tx = tx.Where("domain_uid = ?", uid)
		} else {
			tx = tx.Where("visibility = ?", model.DomainVisibilityPublic)
			if kw != "" {
				like := "%" + kw + "%"
				tx = tx.Where("(domain_name LIKE ? OR domain_code LIKE ? OR description LIKE ?)", like, like, like)
			}
		}
	case "mine":
		memberIDs, err := r.GetUserDomainIDs(userID)
		if err != nil {
			return nil, 0, err
		}
		var conds []string
		var args []interface{}
		if len(memberIDs) > 0 {
			conds = append(conds, "id IN ?")
			args = append(args, memberIDs)
		}
		if len(userOrgIDs) > 0 {
			conds = append(conds, "(visibility = ? AND org_id IN ?)")
			args = append(args, model.DomainVisibilityPublic, userOrgIDs)
		}
		if len(adminOrgIDs) > 0 {
			conds = append(conds, "(visibility = ? AND org_id IN ?)")
			args = append(args, model.DomainVisibilityOrgPrivate, adminOrgIDs)
		}
		if len(conds) == 0 {
			return []model.DomainListItem{}, 0, nil
		}
		tx = tx.Where("("+strings.Join(conds, " OR ")+")", args...)
		if kw != "" {
			like := "%" + kw + "%"
			tx = tx.Where("(domain_name LIKE ? OR domain_code LIKE ? OR domain_uid = ?)", like, like, strings.ToUpper(kw))
		}
	default:
		if uid != "" {
			tx = tx.Where("domain_uid = ?", uid)
		} else {
			memberIDs, err := r.GetUserDomainIDs(userID)
			if err != nil {
				return nil, 0, err
			}
			conds := []string{"visibility = ?"}
			args := []interface{}{model.DomainVisibilityPublic}
			if len(memberIDs) > 0 {
				conds = append(conds, "id IN ?")
				args = append(args, memberIDs)
			}
			if len(adminOrgIDs) > 0 {
				conds = append(conds, "(visibility = ? AND org_id IN ?)")
				args = append(args, model.DomainVisibilityOrgPrivate, adminOrgIDs)
			}
			tx = tx.Where("("+strings.Join(conds, " OR ")+")", args...)
			if kw != "" {
				like := "%" + kw + "%"
				tx = tx.Where("(domain_name LIKE ? OR domain_code LIKE ?)", like, like)
			}
		}
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

	var domains []model.BizDomain
	offset := (page - 1) * pageSize
	if err := tx.Order("id DESC").Offset(offset).Limit(pageSize).Find(&domains).Error; err != nil {
		return nil, 0, err
	}

	items, err := hydrateDomainListItems(r.db, domains, userID, userOrgIDs, adminOrgIDs)
	return items, total, err
}

func hydrateDomainListItems(db *gorm.DB, domains []model.BizDomain, userID uint64, userOrgIDs, adminOrgIDs []uint64) ([]model.DomainListItem, error) {
	if len(domains) == 0 {
		return []model.DomainListItem{}, nil
	}

	type countRow struct {
		DomainID uint64
		Cnt      int64
	}
	var counts []countRow
	if err := db.Model(&model.DomainMember{}).
		Select("domain_id, COUNT(*) AS cnt").
		Group("domain_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	countMap := make(map[uint64]int64, len(counts))
	for _, c := range counts {
		countMap[c.DomainID] = c.Cnt
	}

	var myMemberships []model.DomainMember
	if err := db.Where("user_id = ?", userID).Find(&myMemberships).Error; err != nil {
		return nil, err
	}
	roleMap := make(map[uint64]string, len(myMemberships))
	for _, m := range myMemberships {
		roleMap[m.DomainID] = m.DomainRole
	}

	orgSet := make(map[uint64]bool, len(userOrgIDs))
	for _, oid := range userOrgIDs {
		orgSet[oid] = true
	}
	adminOrgSet := make(map[uint64]bool, len(adminOrgIDs))
	for _, oid := range adminOrgIDs {
		adminOrgSet[oid] = true
	}

	items := make([]model.DomainListItem, 0, len(domains))
	for _, d := range domains {
		myRole := roleMap[d.ID]
		var canManage bool
		if d.Visibility == model.DomainVisibilityOrgPrivate && d.OrgID > 0 {
			canManage = adminOrgSet[d.OrgID]
		} else {
			canManage = d.OwnerID == userID || myRole == model.DomainRoleOwner || myRole == model.DomainRoleAdmin
		}
		hasOrgAccess := d.Visibility == model.DomainVisibilityPublic && d.OrgID > 0 && orgSet[d.OrgID]
		items = append(items, model.DomainListItem{
			ID:           d.ID,
			DomainUID:    d.DomainUID,
			DomainCode:   d.DomainCode,
			DomainName:   d.DomainName,
			Description:  d.Description,
			Visibility:   d.Visibility,
			OrgID:        d.OrgID,
			OrgName:      d.OrgName,
			OwnerID:      d.OwnerID,
			Status:       d.Status,
			MyRole:       myRole,
			CanManage:    canManage,
			HasOrgAccess: hasOrgAccess,
			MemberCount:  countMap[d.ID],
			CreatedAt:    d.CreatedAt,
		})
	}
	return items, nil
}

func (r *domainRepository) FindByID(id uint64) (*model.BizDomain, error) {
	var domain model.BizDomain
	if err := r.db.First(&domain, id).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

func (r *domainRepository) FindByCode(code string) (*model.BizDomain, error) {
	var domain model.BizDomain
	if err := r.db.Where("domain_code = ?", code).First(&domain).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

func (r *domainRepository) FindByUID(uid string) (*model.BizDomain, error) {
	var domain model.BizDomain
	if err := r.db.Where("domain_uid = ?", uid).First(&domain).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

func (r *domainRepository) CreateWithOwner(domain *model.BizDomain) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(domain).Error; err != nil {
			return err
		}
		member := model.DomainMember{
			DomainID:   domain.ID,
			UserID:     domain.OwnerID,
			DomainRole: model.DomainRoleOwner,
		}
		return tx.Create(&member).Error
	})
}

func (r *domainRepository) UpdateDomain(domain *model.BizDomain) error {
	return r.db.Save(domain).Error
}

func (r *domainRepository) DeleteDomain(id uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var dom model.BizDomain
		if err := tx.First(&dom, id).Error; err != nil {
			return err
		}
		suffix := fmt.Sprintf("__del_%d", id)
		newCode := dom.DomainCode
		if len(newCode)+len(suffix) > 64 {
			newCode = newCode[:64-len(suffix)]
		}
		newCode += suffix
		if err := tx.Model(&model.BizDomain{}).Where("id = ?", id).Update("domain_code", newCode).Error; err != nil {
			return err
		}
		if err := tx.Where("domain_id = ?", id).Delete(&model.DomainMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.BizDomain{}, id).Error
	})
}

func (r *domainRepository) ListMembers(domainID uint64) ([]model.DomainMemberItem, error) {
	var items []model.DomainMemberItem
	err := r.db.Table("domain_member").
		Select("domain_member.user_id, sys_user.username, sys_user.nickname, sys_user.email, domain_member.domain_role, domain_member.joined_at").
		Joins("INNER JOIN sys_user ON sys_user.id = domain_member.user_id AND sys_user.deleted_at IS NULL").
		Where("domain_member.domain_id = ?", domainID).
		Order("domain_member.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.DomainMemberItem{}
	}
	return items, nil
}

func (r *domainRepository) GetMemberRole(domainID, userID uint64) (string, error) {
	var member model.DomainMember
	err := r.db.Where("domain_id = ? AND user_id = ?", domainID, userID).First(&member).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return member.DomainRole, nil
}

func (r *domainRepository) UpsertMember(member *model.DomainMember) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "domain_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"domain_role", "updated_at"}),
	}).Create(member).Error
}

func (r *domainRepository) RemoveMember(domainID, userID uint64) error {
	res := r.db.Where("domain_id = ? AND user_id = ?", domainID, userID).Delete(&model.DomainMember{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *domainRepository) GetUserDomainIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.Model(&model.DomainMember{}).
		Where("user_id = ?", userID).
		Pluck("domain_id", &ids).Error
	return ids, err
}

func (r *domainRepository) GetUserAdminDomainIDs(userID uint64) ([]uint64, error) {
	var memberIDs []uint64
	if err := r.db.Model(&model.DomainMember{}).
		Where("user_id = ? AND domain_role IN ?", userID, []string{model.DomainRoleOwner, model.DomainRoleAdmin}).
		Pluck("domain_id", &memberIDs).Error; err != nil {
		return nil, err
	}
	var ownerIDs []uint64
	if err := r.db.Model(&model.BizDomain{}).
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

func (r *domainRepository) GetAccessibleDomainIDs(userID uint64, userOrgIDs, adminOrgIDs []uint64) ([]uint64, error) {
	memberIDs, err := r.GetUserDomainIDs(userID)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint64]bool, len(memberIDs))
	result := make([]uint64, 0, len(memberIDs))
	for _, id := range memberIDs {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	if len(userOrgIDs) > 0 {
		var orgDomainIDs []uint64
		if err := r.db.Model(&model.BizDomain{}).
			Where("visibility = ? AND org_id IN ?", model.DomainVisibilityPublic, userOrgIDs).
			Pluck("id", &orgDomainIDs).Error; err != nil {
			return nil, err
		}
		for _, id := range orgDomainIDs {
			if !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}
	if len(adminOrgIDs) > 0 {
		var orgPrivDomainIDs []uint64
		if err := r.db.Model(&model.BizDomain{}).
			Where("visibility = ? AND org_id IN ?", model.DomainVisibilityOrgPrivate, adminOrgIDs).
			Pluck("id", &orgPrivDomainIDs).Error; err != nil {
			return nil, err
		}
		for _, id := range orgPrivDomainIDs {
			if !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}
	return result, nil
}
