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
	for _, k := range []string{"MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_DATABASE"} {
		t.Setenv(k, "")
	}
	cfg, err := loadFrom(yamlPath, dotEnvPath)
	if err != nil {
		t.Fatalf("loadFrom fallback failed: %v", err)
	}
	wantYAML := "yaml_user:yaml_pass@tcp(127.0.0.1:3306)/yaml_db?charset=utf8mb4&parseTime=True&loc=Local"
	if cfg.Database.DSN != wantYAML {
		t.Fatalf("expected %q, got %q", wantYAML, cfg.Database.DSN)
	}

	// 2. Override from .env
	dotEnvContent := `
# MySQL settings
MYSQL_USER=dotenv_user
MYSQL_PASSWORD=dotenv_pass
MYSQL_HOST=10.0.0.2
MYSQL_PORT=3307
MYSQL_DATABASE=dotenv_db
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

	// 3. Process env overrides .env
	t.Setenv("MYSQL_HOST", "db.internal")
	cfg, err = loadFrom(yamlPath, dotEnvPath)
	if err != nil {
		t.Fatalf("loadFrom env override failed: %v", err)
	}
	wantEnv := "dotenv_user:dotenv_pass@tcp(db.internal:3307)/dotenv_db?charset=utf8mb4&parseTime=True&loc=Local"
	if cfg.Database.DSN != wantEnv {
		t.Fatalf("expected %q, got %q", wantEnv, cfg.Database.DSN)
	}
}
