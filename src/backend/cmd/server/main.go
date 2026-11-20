package main

import (
	"log"
	"net/http"

	"ascp-backend/internal/config"
	"ascp-backend/internal/handler"
	ascpMiddleware "ascp-backend/internal/middleware"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/pkg/validator"
	"ascp-backend/internal/repository"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	db, err := repository.InitDB(cfg.Database, cfg.Server.Mode)
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err))
	}

	if err := repository.AutoMigrate(db); err != nil {
		logger.Fatal("failed to auto migrate database", zap.Error(err))
	}
	if err := repository.SeedRoles(db); err != nil {
		logger.Fatal("failed to seed default roles", zap.Error(err))
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	domainRepo := repository.NewDomainRepository(db)
	assetRepo := repository.NewAssetRepository(db)

	// Services
	authSvc := service.NewAuthService(userRepo, cfg.JWT)
	domainSvc := service.NewDomainService(domainRepo, userRepo)
	assetSvc := service.NewAssetService(assetRepo, domainRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	domainHandler := handler.NewDomainHandler(domainSvc)
	assetHandler := handler.NewAssetHandler(assetSvc)

	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()

	// Core middleware chain
	e.Use(middleware.RequestID())
	e.Use(ascpMiddleware.ZapLogger(logger))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	v1 := e.Group("/api/v1")

	// Health check endpoint
	v1.GET("/healthz", func(c echo.Context) error {
		dbStatus := "ok"
		if sqlDB, err := db.DB(); err != nil || sqlDB.Ping() != nil {
			dbStatus = "unavailable"
		}
		return response.Success(c, map[string]string{
			"status":   "ok",
			"service":  "ascp-backend",
			"database": dbStatus,
		})
	})

	// Public auth routes
	v1.POST("/auth/register", authHandler.Register)
	v1.POST("/auth/login", authHandler.Login)

	// Protected routes
	protected := v1.Group("", ascpMiddleware.AuthMiddleware(cfg.JWT.Secret, userRepo))

	protected.POST("/auth/logout", authHandler.Logout)

	protected.GET("/user/profile", authHandler.GetProfile)
	protected.PUT("/user/profile", authHandler.UpdateProfile)
	protected.PUT("/user/password", authHandler.UpdatePassword)
	protected.GET("/user/assets", assetHandler.ListUserAssets)

	protected.GET("/domains", domainHandler.ListDomains)
	protected.POST("/domains", domainHandler.CreateDomain)
	protected.GET("/domains/:id/members", domainHandler.ListMembers)
	protected.POST("/domains/:id/members", domainHandler.AddMember)

	protected.GET("/assets", assetHandler.ListAssets)
	protected.GET("/assets/:id", assetHandler.GetAsset)
	protected.POST("/assets", assetHandler.CreateAsset)
	protected.PUT("/assets/:id", assetHandler.UpdateAsset)
	protected.DELETE("/assets/:id", assetHandler.DeleteAsset)
	protected.POST("/assets/:id/publish", assetHandler.PublishAsset)
	protected.POST("/assets/:id/offline", assetHandler.OfflineAsset)

	logger.Info("starting ASCP backend server", zap.String("port", cfg.Server.Port))
	if err := e.Start(cfg.Server.Port); err != nil && err != http.ErrServerClosed {
		logger.Fatal("server exited with error", zap.Error(err))
	}
}
