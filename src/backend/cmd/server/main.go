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
	orgRepo := repository.NewOrgRepository(db)
	domainRepo := repository.NewDomainRepository(db)
	assetRepo := repository.NewAssetRepository(db)
	kbRepo := repository.NewKBRepository(db)
	govRepo := repository.NewGovernanceRepository(db)
	keyRepo := repository.NewAPIKeyRepository(db)
	logRepo := repository.NewInvokeLogRepository(db)

	// Services
	authSvc := service.NewAuthService(userRepo, cfg.JWT)
	orgSvc := service.NewOrgService(orgRepo, userRepo)
	domainSvc := service.NewDomainService(domainRepo, userRepo, orgRepo)
	assetSvc := service.NewAssetService(assetRepo, domainRepo, govRepo, orgRepo, cfg.Storage.UploadDir)
	kbSvc := service.NewKBService(kbRepo, assetRepo, domainRepo, govRepo, orgRepo, cfg.Storage.UploadDir)
	govSvc := service.NewGovernanceService(govRepo, assetRepo, domainRepo, orgRepo)
	keySvc := service.NewAPIKeyService(keyRepo)
	invokeSvc := service.NewInvokeService(assetRepo, domainRepo, govRepo, kbRepo, logRepo, orgRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	orgHandler := handler.NewOrgHandler(orgSvc)
	domainHandler := handler.NewDomainHandler(domainSvc)
	assetHandler := handler.NewAssetHandler(assetSvc)
	kbHandler := handler.NewKBHandler(kbSvc)
	govHandler := handler.NewGovernanceHandler(govSvc)
	apiKeyHandler := handler.NewAPIKeyHandler(keySvc)
	invokeHandler := handler.NewInvokeHandler(invokeSvc)

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

	// Protected routes (JWT)
	protected := v1.Group("", ascpMiddleware.AuthMiddleware(cfg.JWT.Secret, userRepo))

	protected.POST("/auth/logout", authHandler.Logout)

	protected.GET("/user/profile", authHandler.GetProfile)
	protected.PUT("/user/profile", authHandler.UpdateProfile)
	protected.PUT("/user/password", authHandler.UpdatePassword)
	protected.GET("/user/assets", assetHandler.ListUserAssets)
	protected.GET("/user/api-keys", apiKeyHandler.ListKeys)
	protected.POST("/user/api-keys", apiKeyHandler.CreateKey)
	protected.DELETE("/user/api-keys/:id", apiKeyHandler.RevokeKey)
	protected.GET("/user/invoke-logs", invokeHandler.ListInvokeLogs)

	protected.GET("/orgs", orgHandler.ListOrgs)
	protected.POST("/orgs", orgHandler.CreateOrg)
	protected.GET("/orgs/:id", orgHandler.GetOrgDetail)
	protected.GET("/orgs/:id/members", orgHandler.ListMembers)
	protected.POST("/orgs/:id/members", orgHandler.AddMember)
	protected.DELETE("/orgs/:id/members/:userId", orgHandler.RemoveMember)

	protected.GET("/domains", domainHandler.ListDomains)
	protected.POST("/domains", domainHandler.CreateDomain)
	protected.PUT("/domains/:id", domainHandler.UpdateDomain)
	protected.GET("/domains/:id/members", domainHandler.ListMembers)
	protected.POST("/domains/:id/members", domainHandler.AddMember)
	protected.PUT("/domains/:id/members/:userId", domainHandler.UpdateMemberRole)
	protected.DELETE("/domains/:id/members/:userId", domainHandler.RemoveMember)

	protected.GET("/assets", assetHandler.ListAssets)
	protected.GET("/assets/:id", assetHandler.GetAsset)
	protected.POST("/assets", assetHandler.CreateAsset)
	protected.PUT("/assets/:id", assetHandler.UpdateAsset)
	protected.DELETE("/assets/:id", assetHandler.DeleteAsset)
	protected.POST("/assets/:id/publish", assetHandler.PublishAsset)
	protected.POST("/assets/:id/offline", assetHandler.OfflineAsset)
	protected.GET("/assets/:id/versions", assetHandler.ListVersions)
	protected.POST("/assets/:id/rollback", assetHandler.RollbackAsset)
	protected.POST("/assets/:id/mcp/health-check", assetHandler.CheckMCPHealth)
	protected.POST("/assets/:id/mcp/sync-tools", assetHandler.SyncMCPTools)
	protected.POST("/assets/:id/subscribe", govHandler.SubscribeAsset)
	protected.POST("/skills/parse-file", assetHandler.ParseSkillFile)
	protected.POST("/skills/import-url", assetHandler.ImportSkillURL)
	protected.POST("/assets/:id/skill/package", assetHandler.UploadSkillPackage)
	protected.GET("/assets/:id/skill/download", assetHandler.DownloadSkillPackage)
	protected.GET("/mcp/registry/search", assetHandler.SearchMCPRegistry)
	protected.POST("/mcp/parse-json", assetHandler.ParseMCPConfig)
	protected.POST("/agents/parse-file", assetHandler.ParseAgentFile)
	protected.POST("/agents/import-url", assetHandler.ImportAgentURL)
	protected.GET("/assets/:id/agent/download", assetHandler.DownloadAgentPackage)

	protected.POST("/kb/:id/folders", kbHandler.CreateFolder)
	protected.POST("/kb/:id/documents", kbHandler.UploadDocument)
	protected.GET("/kb/:id/documents", kbHandler.ListDocuments)
	protected.GET("/kb/:id/documents/:docId/content", kbHandler.GetDocumentContent)
	protected.GET("/kb/:id/documents/:docId/chunks", kbHandler.ListChunks)
	protected.DELETE("/kb/:id/documents/:docId", kbHandler.DeleteDocument)
	protected.POST("/kb/:id/search", kbHandler.Search)
	protected.GET("/kb/:id/download/preview", kbHandler.DownloadPreview)
	protected.GET("/kb/:id/download", kbHandler.Download)

	protected.POST("/approvals", govHandler.CreateApproval)
	protected.GET("/approvals/my-requests", govHandler.ListMyRequests)
	protected.GET("/approvals/pending", govHandler.ListPendingRequests)
	protected.GET("/approvals/handled", govHandler.ListHandledRequests)
	protected.POST("/approvals/:id/handle", govHandler.HandleApproval)

	// Unified invoke routes (JWT or X-API-Key + AssetGuardMiddleware)
	invoke := v1.Group(
		"/invoke",
		ascpMiddleware.UnifiedAuthMiddleware(cfg.JWT.Secret, userRepo, keyRepo),
		ascpMiddleware.AssetGuardMiddleware(invokeSvc),
	)
	invoke.POST("/mcp/:assetCode", invokeHandler.InvokeMCP)
	invoke.POST("/skill/:assetCode", invokeHandler.InvokeSkill)
	invoke.POST("/agent/:assetCode", invokeHandler.InvokeAgent)
	invoke.POST("/kb/:assetCode/query", invokeHandler.InvokeKB)

	logger.Info("starting ASCP backend server", zap.String("port", cfg.Server.Port))
	if err := e.Start(cfg.Server.Port); err != nil && err != http.ErrServerClosed {
		logger.Fatal("server exited with error", zap.Error(err))
	}
}
