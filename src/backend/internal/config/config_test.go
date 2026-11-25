package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FallbackAndEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	dotEnvPath := filepath.Join(tmpDir, ".env")
	yamlContent := `server:
  port: ":8080"
  mode: "debug"
database:
  dsn: "yaml_user:yaml_pass@tcp(127.0.0.1:3306)/yaml_db?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conns: 10
  max_open_conns: 100
jwt:
  secret: "test-secret"
  expire_hours: 24
storage:
  upload_dir: "./uploads"
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	// 1. Fallback to config.yaml when no .env or env vars are set
	for _, k := range []string{
		"MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_DATABASE", "JWT_SECRET",
		"SERVER_MODE", "STORAGE_SECRET", "Aliyun_User_AK", "Aliyun_User_SK", "ALIYUN_DM_ENDPOINT", "ALIYUN_DM_ACCOUNT_NAME", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(k, "")
	}
	cfg, err := loadFrom(yamlPath, dotEnvPath)
	if err != nil {
		t.Fatalf("loadFrom fallback failed: %v", err)
	}
	if cfg.Email.Endpoint != "dm.aliyuncs.com" || cfg.Email.AccountName != "verify@email.7thcv.cn" {
		t.Fatalf("unexpected default email config: %+v", cfg.Email)
	}
	wantYAML := "yaml_user:yaml_pass@tcp(127.0.0.1:3306)/yaml_db?charset=utf8mb4&parseTime=True&loc=Local"
	if cfg.Database.DSN != wantYAML {
		t.Fatalf("expected %q, got %q", wantYAML, cfg.Database.DSN)
	}
	if cfg.JWT.Secret != "test-secret" {
		t.Fatalf("expected JWT secret %q, got %q", "test-secret", cfg.JWT.Secret)
	}

	// 2. Override from .env
	dotEnvContent := `
# MySQL settings
MYSQL_USER=dotenv_user
MYSQL_PASSWORD=dotenv_pass
MYSQL_HOST=10.0.0.2
MYSQL_PORT=3307
MYSQL_DATABASE=dotenv_db
JWT_SECRET=dotenv-jwt-secret
`
	if err := os.WriteFile(dotEnvPath, []byte(dotEnvContent), 0600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	cfg, err = loadFrom(yamlPath, dotEnvPath)
	if err != nil {
		t.Fatalf("loadFrom dotenv failed: %v", err)
	}
	wantDotEnv := "dotenv_user:dotenv_pass@tcp(10.0.0.2:3307)/dotenv_db?charset=utf8mb4&parseTime=True&loc=Local"
	if cfg.Database.DSN != wantDotEnv {
		t.Fatalf("expected %q, got %q", wantDotEnv, cfg.Database.DSN)
	}
	if cfg.JWT.Secret != "dotenv-jwt-secret" {
		t.Fatalf("expected JWT secret %q, got %q", "dotenv-jwt-secret", cfg.JWT.Secret)
	}

	// 3. Process env overrides .env
	t.Setenv("MYSQL_HOST", "db.internal")
	t.Setenv("JWT_SECRET", "env-jwt-secret")
	t.Setenv("SERVER_MODE", "release")
	t.Setenv("STORAGE_SECRET", "env-storage-secret")
	t.Setenv("Aliyun_User_AK", "test-ak")
	t.Setenv("Aliyun_User_SK", "test-sk")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.example.com, https://b.example.com")
	cfg, err = loadFrom(yamlPath, dotEnvPath)
	if err != nil {
		t.Fatalf("loadFrom env override failed: %v", err)
	}
	wantEnv := "dotenv_user:dotenv_pass@tcp(db.internal:3307)/dotenv_db?charset=utf8mb4&parseTime=True&loc=Local"
	if cfg.Database.DSN != wantEnv {
		t.Fatalf("expected %q, got %q", wantEnv, cfg.Database.DSN)
	}
	if cfg.JWT.Secret != "env-jwt-secret" {
		t.Fatalf("expected JWT secret %q, got %q", "env-jwt-secret", cfg.JWT.Secret)
	}
	if cfg.Server.Mode != "release" || cfg.Storage.Secret != "env-storage-secret" {
		t.Fatalf("unexpected server/storage config: mode=%q secret=%q", cfg.Server.Mode, cfg.Storage.Secret)
	}
	if cfg.Email.AliyunAK != "test-ak" || cfg.Email.AliyunSK != "test-sk" {
		t.Fatalf("unexpected email AK/SK: %+v", cfg.Email)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 || cfg.CORS.AllowedOrigins[0] != "https://a.example.com" || cfg.CORS.AllowedOrigins[1] != "https://b.example.com" {
		t.Fatalf("unexpected CORS allowed origins: %+v", cfg.CORS.AllowedOrigins)
	}
}
