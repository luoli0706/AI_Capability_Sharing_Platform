package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ascp-backend/internal/config"
	"ascp-backend/internal/handler"
	ascpMiddleware "ascp-backend/internal/middleware"
	"ascp-backend/internal/pkg/mailer"
	"ascp-backend/internal/pkg/ossstore"
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

	const defaultWeakJWTSecret = "change-me-in-production-ascp-jwt-secret"
	if cfg.Server.Mode != "debug" {
		if cfg.JWT.Secret == "" || cfg.JWT.Secret == defaultWeakJWTSecret {
			logger.Fatal("JWT_SECRET must be set to a strong secret in non-debug mode")
		}
	}

	storageSecret := cfg.Storage.Secret
	if storageSecret == "" {
		storageSecret = os.Getenv("STORAGE_SECRET")
	}
	if storageSecret == "" {
		if cfg.Server.Mode == "debug" {
			storageSecret = cfg.JWT.Secret
		} else {
			logger.Fatal("STORAGE_SECRET must be set in non-debug mode")
		}
	}
	if cfg.Server.Mode != "debug" && storageSecret == defaultWeakJWTSecret {
		logger.Fatal("STORAGE_SECRET must not use the default weak secret in non-debug mode")
	}

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
	emailRepo := repository.NewEmailRepository(db)
	orgRepo := repository.NewOrgRepository(db)
	domainRepo := repository.NewDomainRepository(db)
	assetRepo := repository.NewAssetRepository(db)
	kbRepo := repository.NewKBRepository(db)
	govRepo := repository.NewGovernanceRepository(db)
	keyRepo := repository.NewAPIKeyRepository(db)
	logRepo := repository.NewInvokeLogRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	crossRefRepo := repository.NewCrossRefRepository(db)
	msgRepo := repository.NewMessageRepository(db)
	kbInteractionRepo := repository.NewKBInteractionRepository(db)

	// Services
	var mailSender mailer.Mailer
	if cfg.Email.AliyunAK != "" && cfg.Email.AliyunSK != "" {
		mailSender, err = mailer.NewAliyunDirectMailer(
			cfg.Email.AliyunAK,
			cfg.Email.AliyunSK,
			cfg.Email.Endpoint,
			cfg.Email.AccountName,
		)
		if err != nil {
			logger.Warn("failed to initialize aliyun directmail client", zap.Error(err))
		}
	} else {
		logger.Warn("aliyun directmail AK/SK not configured; email verification sending disabled")
	}
	emailSvc := service.NewEmailService(emailRepo, userRepo, mailSender, cfg.JWT.Secret)

	authSvc := service.NewAuthService(userRepo, cfg.JWT)
	authSvc.SetEmailService(emailSvc)
	userSvc := service.NewUserService(userRepo, kbInteractionRepo, domainRepo, orgRepo, govRepo)
	msgSvc := service.NewMessageService(msgRepo, userRepo)
	orgSvc := service.NewOrgService(orgRepo, userRepo)
	domainSvc := service.NewDomainService(domainRepo, userRepo, orgRepo)
	assetSvc := service.NewAssetService(assetRepo, domainRepo, govRepo, orgRepo, cfg.Storage.UploadDir)
	kbSvc := service.NewKBService(kbRepo, assetRepo, domainRepo, govRepo, orgRepo, cfg.Storage.UploadDir, storageSecret)
	if encryptedCount, err := kbSvc.EncryptExistingPlainFiles(context.Background()); err != nil {
		logger.Warn("failed to encrypt existing plain KB files at rest", zap.Error(err))
	} else if encryptedCount > 0 {
		logger.Info("encrypted existing plain KB files at rest", zap.Int("count", encryptedCount))
	}

	tierCtx, tierCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer tierCancel()
	if cfg.OSS.AccessKeyID != "" && cfg.OSS.AccessKeySecret != "" {
		ossStore, ossErr := ossstore.NewAliyunOSSStore(
			cfg.OSS.Endpoint,
			cfg.OSS.AccessKeyID,
			cfg.OSS.AccessKeySecret,
			cfg.OSS.Bucket,
		)
		if ossErr != nil {
			logger.Warn("failed to initialize aliyun OSS store; tiered storage disabled", zap.Error(ossErr))
		} else {
			tierSvc := service.NewStorageTierService(
				kbRepo,
				ossStore,
				cfg.Storage.UploadDir,
				cfg.OSS.RetentionHours,
				cfg.OSS.MaxLocalMB,
				logger,
			)
			kbSvc.SetStorageTierService(tierSvc)
			tierSvc.Start(tierCtx)
			logger.Info("ECS <-> OSS tiered storage service started",
				zap.String("bucket", cfg.OSS.Bucket),
				zap.String("endpoint", ossstore.NormalizeEndpoint(cfg.OSS.Endpoint, cfg.OSS.Bucket)),
				zap.Int("retention_hours", cfg.OSS.RetentionHours),
				zap.Int64("max_local_mb", cfg.OSS.MaxLocalMB),
			)
		}
	} else {
		logger.Info("OSS AK/SK not configured; tiered storage disabled (local-only mode)")
	}
	govSvc := service.NewGovernanceService(govRepo, assetRepo, domainRepo, orgRepo)
	keySvc := service.NewAPIKeyService(keyRepo)
	invokeSvc := service.NewInvokeService(assetRepo, domainRepo, govRepo, kbRepo, logRepo, orgRepo)
	auditSvc := service.NewAuditService(auditRepo, domainRepo, orgRepo)
	crossRefSvc := service.NewCrossRefService(crossRefRepo, domainRepo, orgRepo, assetRepo, kbRepo, govRepo)
	sevenOSvc := service.NewSevenOService(cfg.SevenO.BinPath, cfg.SevenO.ReleaseDir)
	sevenOSvc.SetLimits(cfg.SevenO.MaxConcurrent, cfg.SevenO.MaxRequestBytes)

	assetSvc.SetAuditRepos(auditRepo, userRepo)
	kbSvc.SetAuditRepos(auditRepo, userRepo)
	govSvc.SetAuditRepos(auditRepo, userRepo)
	domainSvc.SetAuditRepos(auditRepo, userRepo)
	orgSvc.SetAuditRepos(auditRepo, userRepo)
	crossRefSvc.SetAuditRepos(auditRepo, userRepo)

	assetSvc.SetCrossRefRepo(crossRefRepo)
	assetSvc.SetKBRepo(kbRepo)
	msgSvc.SetAssetAccessRepos(assetRepo, kbRepo, domainRepo, orgRepo, govRepo, crossRefRepo)
	kbSvc.SetCrossRefRepo(crossRefRepo)
	kbSvc.SetInteractionRepo(kbInteractionRepo)
	domainSvc.SetCrossRefRepo(crossRefRepo)
	govSvc.SetCrossRefRepo(crossRefRepo)
	kbInteractionSvc := service.NewKBInteractionService(kbInteractionRepo, kbRepo, assetRepo, domainRepo, govRepo, orgRepo, userRepo)
	kbInteractionSvc.SetCrossRefRepo(crossRefRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authSvc)
	authHandler.SetEmailService(emailSvc)
	userHandler := handler.NewUserHandler(userSvc)
	msgHandler := handler.NewMessageHandler(msgSvc)
	orgHandler := handler.NewOrgHandler(orgSvc, govSvc)
	domainHandler := handler.NewDomainHandler(domainSvc)
	assetHandler := handler.NewAssetHandler(assetSvc)
	kbHandler := handler.NewKBHandler(kbSvc)
	kbInteractionHandler := handler.NewKBInteractionHandler(kbInteractionSvc)
	govHandler := handler.NewGovernanceHandler(govSvc)
	apiKeyHandler := handler.NewAPIKeyHandler(keySvc)
	invokeHandler := handler.NewInvokeHandler(invokeSvc)
	auditHandler := handler.NewAuditHandler(auditSvc)
	crossRefHandler := handler.NewCrossRefHandler(crossRefSvc)
	sevenOHandler := handler.NewSevenOHandler(sevenOSvc)

	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()

	// Core middleware chain
	e.Use(middleware.RequestID())
	e.Use(ascpMiddleware.ZapLogger(logger))
	e.Use(middleware.Recover())
	if len(cfg.CORS.AllowedOrigins) > 0 {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: cfg.CORS.AllowedOrigins,
		}))
	} else {
		e.Use(middleware.CORS())
	}

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
	v1.POST("/auth/email/send-code", authHandler.SendRegisterEmailCode)
	v1.POST("/auth/register", authHandler.Register)
	v1.POST("/auth/login", authHandler.Login)

	// Protected routes (JWT)
	protected := v1.Group("", ascpMiddleware.AuthMiddleware(cfg.JWT.Secret, userRepo))

	protected.POST("/auth/logout", authHandler.Logout)

	protected.GET("/user/profile", authHandler.GetProfile)
	protected.PUT("/user/profile", authHandler.UpdateProfile)
	protected.POST("/user/email/send-code", authHandler.SendBindEmailCode)
	protected.PUT("/user/email", authHandler.BindEmail)
	protected.PUT("/user/password", authHandler.UpdatePassword)
	protected.GET("/user/favorites", userHandler.GetMyFavorites)
	protected.GET("/user/assets", assetHandler.ListUserAssets)
	protected.GET("/user/api-keys", apiKeyHandler.ListKeys)
	protected.POST("/user/api-keys", apiKeyHandler.CreateKey)
	protected.DELETE("/user/api-keys/:id", apiKeyHandler.RevokeKey)
	protected.GET("/user/invoke-logs", invokeHandler.ListInvokeLogs)

	protected.GET("/users/search", userHandler.SearchUsers)
	protected.GET("/users/:id", userHandler.GetPublicProfile)
	protected.GET("/users/:id/favorites", userHandler.GetUserFavorites)

	protected.GET("/messages/unread-count", msgHandler.GetUnreadCount)
	protected.GET("/messages/contacts", msgHandler.ListContacts)
	protected.GET("/messages/conversations", msgHandler.ListConversations)
	protected.POST("/messages/conversations", msgHandler.CreateConversation)
	protected.GET("/messages/conversations/:id/messages", msgHandler.ListMessages)
	protected.POST("/messages/conversations/:id/messages", msgHandler.SendMessage)
	protected.POST("/messages/conversations/:id/read", msgHandler.MarkRead)
	protected.POST("/messages/share-card", msgHandler.ShareAssetCard)

	protected.GET("/orgs", orgHandler.ListOrgs)
	protected.POST("/orgs", orgHandler.CreateOrg)
	protected.GET("/orgs/:id", orgHandler.GetOrgDetail)
	protected.PUT("/orgs/:id", orgHandler.UpdateOrg)
	protected.DELETE("/orgs/:id", orgHandler.DeleteOrg)
	protected.GET("/orgs/:id/members", orgHandler.ListMembers)
	protected.POST("/orgs/:id/members", orgHandler.AddMember)
	protected.DELETE("/orgs/:id/members/:userId", orgHandler.RemoveMember)
	protected.POST("/orgs/:id/join", orgHandler.JoinOrg)

	protected.GET("/domains", domainHandler.ListDomains)
	protected.POST("/domains", domainHandler.CreateDomain)
	protected.PUT("/domains/:id", domainHandler.UpdateDomain)
	protected.DELETE("/domains/:id", domainHandler.DeleteDomain)
	protected.GET("/domains/:id/members", domainHandler.ListMembers)
	protected.POST("/domains/:id/members", domainHandler.AddMember)
	protected.PUT("/domains/:id/members/:userId", domainHandler.UpdateMemberRole)
	protected.DELETE("/domains/:id/members/:userId", domainHandler.RemoveMember)
	protected.POST("/domains/:id/cross-refs", crossRefHandler.CreateCrossRef)
	protected.GET("/domains/:id/cross-refs", crossRefHandler.ListCrossRefs)
	protected.DELETE("/domains/:id/cross-refs/:refId", crossRefHandler.DeleteCrossRef)
	protected.GET("/cross-refs/source-domains", crossRefHandler.ListSourceDomains)
	protected.GET("/cross-refs/source-items", crossRefHandler.ListSourceItems)

	protected.GET("/assets", assetHandler.ListAssets)
	protected.GET("/assets/:id", assetHandler.GetAsset)
	protected.GET("/assets/:id/access-check", assetHandler.CheckAssetAccess)
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
	protected.POST("/kb/:id/files", kbHandler.CreateMarkdownFile)
	protected.POST("/kb/:id/documents", kbHandler.UploadDocument)
	protected.GET("/kb/:id/documents", kbHandler.ListDocuments)
	protected.GET("/kb/:id/documents/:docId/meta", kbHandler.GetDocumentMeta)
	protected.GET("/kb/:id/documents/:docId/content", kbHandler.GetDocumentContent)
	protected.GET("/kb/:id/documents/:docId/raw", kbHandler.GetDocumentRaw)
	protected.GET("/kb/:id/documents/:docId/bilibili", kbHandler.GetBilibiliPreview)
	protected.PUT("/kb/:id/documents/:docId/content", kbHandler.UpdateDocumentContent)
	protected.GET("/kb/:id/documents/:docId/chunks", kbHandler.ListChunks)
	protected.DELETE("/kb/:id/documents/:docId", kbHandler.DeleteDocument)
	protected.POST("/kb/:id/documents/:docId/restore", kbHandler.RestoreDocument)
	protected.POST("/kb/:id/documents/:docId/move", kbHandler.MoveDocument)
	protected.POST("/kb/:id/documents/:docId/copy", kbHandler.CopyDocument)
	protected.PUT("/kb/:id/documents/:docId/rename", kbHandler.RenameDocument)
	protected.GET("/kb/:id/documents/:docId/reading-progress", kbInteractionHandler.GetReadingProgress)
	protected.PUT("/kb/:id/documents/:docId/reading-progress", kbInteractionHandler.SaveReadingProgress)
	protected.GET("/kb/:id/documents/:docId/interactions", kbInteractionHandler.GetInteractions)
	protected.POST("/kb/:id/documents/:docId/like", kbInteractionHandler.ToggleLike)
	protected.POST("/kb/:id/documents/:docId/favorite", kbInteractionHandler.ToggleFavorite)
	protected.GET("/kb/:id/documents/:docId/comments", kbInteractionHandler.ListComments)
	protected.POST("/kb/:id/documents/:docId/comments", kbInteractionHandler.CreateComment)
	protected.DELETE("/kb/:id/documents/:docId/comments/:commentId", kbInteractionHandler.DeleteComment)
	protected.POST("/kb/:id/search", kbHandler.Search)
	protected.GET("/kb/:id/download/preview", kbHandler.DownloadPreview)
	protected.GET("/kb/:id/download", kbHandler.Download)
	protected.GET("/audit-logs", auditHandler.ListLogs)

	protected.POST("/approvals", govHandler.CreateApproval)
	protected.GET("/approvals/my-requests", govHandler.ListMyRequests)
	protected.GET("/approvals/pending", govHandler.ListPendingRequests)
	protected.GET("/approvals/handled", govHandler.ListHandledRequests)
	protected.POST("/approvals/:id/handle", govHandler.HandleApproval)

	protected.GET("/tools/7o/releases", sevenOHandler.ListReleases)
	protected.GET("/tools/7o/releases/:filename", sevenOHandler.DownloadRelease)
	protected.POST("/tools/7o/encrypt", sevenOHandler.Encrypt)
	protected.POST("/tools/7o/decrypt", sevenOHandler.Decrypt)
	protected.POST("/tools/7o/inspect", sevenOHandler.Inspect)
	protected.GET("/tools/video-link/resolve", kbHandler.ResolveExternalVideoURL, ascpMiddleware.RateLimitByUserOrIP(20, time.Minute))

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
	go func() {
		<-tierCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = e.Shutdown(shutdownCtx)
	}()
	if err := e.Start(cfg.Server.Port); err != nil && err != http.ErrServerClosed {
		logger.Fatal("server exited with error", zap.Error(err))
	}
}
