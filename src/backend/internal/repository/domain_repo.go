package repository

import (
	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DomainRepository interface {
	ListDomains(userID uint64) ([]model.DomainListItem, error)
	FindByID(id uint64) (*model.BizDomain, error)
	FindByCode(code string) (*model.BizDomain, error)
	CreateWithOwner(domain *model.BizDomain) error
	ListMembers(domainID uint64) ([]model.DomainMemberItem, error)
	GetMemberRole(domainID, userID uint64) (string, error)
	UpsertMember(member *model.DomainMember) error
	GetUserDomainIDs(userID uint64) ([]uint64, error)
	GetUserAdminDomainIDs(userID uint64) ([]uint64, error)
}

type domainRepository struct {
	db *gorm.DB
}

func NewDomainRepository(db *gorm.DB) DomainRepository {
	return &domainRepository{db: db}
}

func (r *domainRepository) ListDomains(userID uint64) ([]model.DomainListItem, error) {
	var domains []model.BizDomain
	if err := r.db.Order("id DESC").Find(&domains).Error; err != nil {
		return nil, err
	}
	if len(domains) == 0 {
		return []model.DomainListItem{}, nil
	}

	type countRow struct {
		DomainID uint64
		Cnt      int64
	}
	var counts []countRow
	if err := r.db.Model(&model.DomainMember{}).
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
	if err := r.db.Where("user_id = ?", userID).Find(&myMemberships).Error; err != nil {
		return nil, err
	}
	roleMap := make(map[uint64]string, len(myMemberships))
	for _, m := range myMemberships {
		roleMap[m.DomainID] = m.DomainRole
	}

	items := make([]model.DomainListItem, 0, len(domains))
	for _, d := range domains {
		items = append(items, model.DomainListItem{
			ID:          d.ID,
			DomainCode:  d.DomainCode,
			DomainName:  d.DomainName,
			Description: d.Description,
			OwnerID:     d.OwnerID,
			Status:      d.Status,
			MyRole:      roleMap[d.ID],
			MemberCount: countMap[d.ID],
			CreatedAt:   d.CreatedAt,
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

func (r *domainRepository) GetUserDomainIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.Model(&model.DomainMember{}).
		Where("user_id = ?", userID).
		Pluck("domain_id", &ids).Error
	return ids, err
}

func (r *domainRepository) GetUserAdminDomainIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.Model(&model.DomainMember{}).
		Where("user_id = ? AND domain_role IN ?", userID, []string{model.DomainRoleOwner, model.DomainRoleAdmin}).
		Pluck("domain_id", &ids).Error
	return ids, err
}
