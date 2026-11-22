package repository

import (
	"time"

	"ascp-backend/internal/config"
	"ascp-backend/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// InitDB opens a MySQL 8.0 connection via GORM and configures the connection pool.
func InitDB(cfg config.DatabaseConfig, mode string) (*gorm.DB, error) {
	logLevel := gormlogger.Warn
	if mode == "debug" {
		logLevel = gormlogger.Info
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 100
	}

	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// AutoMigrate runs GORM AutoMigrate for all P0, P1, and Organization models.
func AutoMigrate(db *gorm.DB) error {
	// Ensure legacy biz_domain rows have unique domain_uid values before GORM creates uk_domain_uid.
	if db.Migrator().HasTable(&model.BizDomain{}) {
		if !db.Migrator().HasColumn(&model.BizDomain{}, "domain_uid") {
			if err := db.Migrator().AddColumn(&model.BizDomain{}, "DomainUID"); err != nil {
				return err
			}
		}
		if err := db.Model(&model.BizDomain{}).
			Where("domain_uid = '' OR domain_uid IS NULL").
			Update("domain_uid", gorm.Expr("CONCAT('DOM-', id)")).Error; err != nil {
			return err
		}
	}

	if err := db.AutoMigrate(
		&model.SysUser{},
		&model.SysRole{},
		&model.SysUserRole{},
		&model.UserAPIKey{},
		&model.BizOrganization{},
		&model.OrganizationMember{},
		&model.BizDomain{},
		&model.DomainMember{},
		&model.AIAsset{},
		&model.AIAssetMCP{},
		&model.AIAssetSkill{},
		&model.AIAssetAgent{},
		&model.AIAssetVersion{},
		&model.KBKnowledgeBase{},
		&model.KBDocument{},
		&model.KBChunk{},
		&model.KBDocumentLike{},
		&model.KBDocumentFavorite{},
		&model.KBDocumentComment{},
		&model.AssetPermission{},
		&model.ApprovalRequest{},
		&model.ApprovalRecord{},
		&model.AssetInvokeLog{},
		&model.AssetAuditLog{},
		&model.DomainCrossRef{},
		&model.UserConversation{},
		&model.UserMessage{},
	); err != nil {
		return err
	}
	if err := db.Exec("UPDATE biz_organization SET visibility = ? WHERE visibility IS NULL OR visibility = ''", model.OrgVisibilityPublic).Error; err != nil {
		return err
	}
	return BackfillDomains(db)
}

// BackfillDomains ensures legacy biz_domain rows have valid domain_uid, visibility, and org_name.
func BackfillDomains(db *gorm.DB) error {
	if err := db.Model(&model.BizDomain{}).
		Where("domain_uid = '' OR domain_uid IS NULL").
		Update("domain_uid", gorm.Expr("CONCAT('DOM-', id)")).Error; err != nil {
		return err
	}
	if err := db.Model(&model.BizDomain{}).
		Where("visibility = '' OR visibility IS NULL").
		Update("visibility", model.DomainVisibilityPublic).Error; err != nil {
		return err
	}
	return db.Model(&model.BizDomain{}).
		Where("org_name = '' OR org_name IS NULL").
		Update("org_name", "个人").Error
}

// SeedRoles ensures the default RBAC roles exist in sys_role.
func SeedRoles(db *gorm.DB) error {
	roles := []model.SysRole{
		{
			RoleCode:    model.RoleSuperAdmin,
			RoleName:    "平台超级管理员",
			Description: "拥有全平台域、用户、资产与审批管理权限",
		},
		{
			RoleCode:    model.RoleDomainAdmin,
			RoleName:    "域管理员",
			Description: "负责所属业务域的成员管理、资产上线审核与跨域审批",
		},
		{
			RoleCode:    model.RoleDeveloper,
			RoleName:    "研发人员",
			Description: "可创建与管理个人资产、检索订阅及调用授权能力",
		},
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "role_code"}},
		DoNothing: true,
	}).Create(&roles).Error
}
