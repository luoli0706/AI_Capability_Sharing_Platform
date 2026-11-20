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

// AutoMigrate runs GORM AutoMigrate for all P0 models.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.SysUser{},
		&model.SysRole{},
		&model.SysUserRole{},
		&model.BizDomain{},
		&model.DomainMember{},
		&model.AIAsset{},
		&model.AIAssetMCP{},
		&model.AIAssetSkill{},
		&model.AIAssetAgent{},
		&model.KBKnowledgeBase{},
	)
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
