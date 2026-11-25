package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Storage  StorageConfig  `mapstructure:"storage"`
	OSS      OSSConfig      `mapstructure:"oss"`
	SevenO   SevenOConfig   `mapstructure:"seveno"`
	Email    EmailConfig    `mapstructure:"email"`
	CORS     CORSConfig     `mapstructure:"cors"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	DSN          string `mapstructure:"dsn"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type StorageConfig struct {
	UploadDir string `mapstructure:"upload_dir"`
	Secret    string `mapstructure:"secret"`
}

type OSSConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	Bucket          string `mapstructure:"bucket"`
	Endpoint        string `mapstructure:"endpoint"`
	RetentionHours  int    `mapstructure:"retention_hours"`
	MaxLocalMB      int64  `mapstructure:"max_local_mb"`
}

type EmailConfig struct {
	AliyunAK    string `mapstructure:"aliyun_ak"`
	AliyunSK    string `mapstructure:"aliyun_sk"`
	Endpoint    string `mapstructure:"endpoint"`
	AccountName string `mapstructure:"account_name"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

type SevenOConfig struct {
	BinPath         string `mapstructure:"bin_path"`
	ReleaseDir      string `mapstructure:"release_dir"`
	MaxConcurrent   int    `mapstructure:"max_concurrent"`
	MaxRequestBytes int64  `mapstructure:"max_request_bytes"`
}

// Load reads configuration from yaml file, optional .env file, and environment variables.
func Load(path string) (*Config, error) {
	return loadFrom(path, ".env")
}

func loadFrom(path, dotEnvPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	dotEnv := loadDotEnv(dotEnvPath)
	user := getEnvValue(dotEnv, "MYSQL_USER")
	password := getEnvValue(dotEnv, "MYSQL_PASSWORD")
	host := getEnvValue(dotEnv, "MYSQL_HOST")
	port := getEnvValue(dotEnv, "MYSQL_PORT")
	database := getEnvValue(dotEnv, "MYSQL_DATABASE")

	if user != "" || password != "" || host != "" || port != "" || database != "" {
		if user == "" {
			user = "root"
		}
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "3306"
		}
		if database == "" {
			database = "ascp"
		}
		cfg.Database.DSN = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port, database,
		)
	}

	if jwtSecret := getEnvValue(dotEnv, "JWT_SECRET"); jwtSecret != "" {
		cfg.JWT.Secret = jwtSecret
	}

	if serverMode := getEnvValue(dotEnv, "SERVER_MODE"); serverMode != "" {
		cfg.Server.Mode = serverMode
	}

	if storageSecret := getEnvValue(dotEnv, "STORAGE_SECRET"); storageSecret != "" {
		cfg.Storage.Secret = storageSecret
	}

	if ak := getEnvValue(dotEnv, "Aliyun_User_AK"); ak != "" {
		cfg.Email.AliyunAK = ak
	}
	if sk := getEnvValue(dotEnv, "Aliyun_User_SK"); sk != "" {
		cfg.Email.AliyunSK = sk
	}
	if endpoint := getEnvValue(dotEnv, "ALIYUN_DM_ENDPOINT"); endpoint != "" {
		cfg.Email.Endpoint = endpoint
	}
	if cfg.Email.Endpoint == "" {
		cfg.Email.Endpoint = "dm.aliyuncs.com"
	}
	if accountName := getEnvValue(dotEnv, "ALIYUN_DM_ACCOUNT_NAME"); accountName != "" {
		cfg.Email.AccountName = accountName
	}
	if cfg.Email.AccountName == "" {
		cfg.Email.AccountName = "verify@email.7thcv.cn"
	}

	if corsOrigins := getEnvValue(dotEnv, "CORS_ALLOWED_ORIGINS"); corsOrigins != "" {
		parts := strings.Split(corsOrigins, ",")
		origins := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		cfg.CORS.AllowedOrigins = origins
	}

	if binPath := getEnvValue(dotEnv, "SEVENO_BIN_PATH"); binPath != "" {
		cfg.SevenO.BinPath = binPath
	}
	if cfg.SevenO.BinPath == "" {
		cfg.SevenO.BinPath = "./bin/seventhcv-7o"
	}

	if releaseDir := getEnvValue(dotEnv, "SEVENO_RELEASE_DIR"); releaseDir != "" {
		cfg.SevenO.ReleaseDir = releaseDir
	}
	if cfg.SevenO.ReleaseDir == "" {
		cfg.SevenO.ReleaseDir = "./releases"
	}

	if ossAK := getEnvValue(dotEnv, "OSS_AK"); ossAK != "" {
		cfg.OSS.AccessKeyID = ossAK
	}
	if ossSK := getEnvValue(dotEnv, "OSS_SK"); ossSK != "" {
		cfg.OSS.AccessKeySecret = ossSK
	}
	if ossBucket := getEnvValue(dotEnv, "OSS_BUCKET"); ossBucket != "" {
		cfg.OSS.Bucket = ossBucket
	}
	if cfg.OSS.Bucket == "" {
		cfg.OSS.Bucket = "ascp-7thcv-sh"
	}
	if ossEndpoint := getEnvValue(dotEnv, "OSS_ENDPOINT"); ossEndpoint != "" {
		cfg.OSS.Endpoint = ossEndpoint
	}
	if cfg.OSS.Endpoint == "" {
		cfg.OSS.Endpoint = "oss-cn-shanghai-internal.aliyuncs.com"
	}
	if retentionStr := getEnvValue(dotEnv, "OSS_LOCAL_RETENTION_HOURS"); retentionStr != "" {
		if v, err := strconv.Atoi(retentionStr); err == nil && v > 0 {
			cfg.OSS.RetentionHours = v
		}
	}
	if cfg.OSS.RetentionHours <= 0 {
		cfg.OSS.RetentionHours = 48
	}
	if maxMBStr := getEnvValue(dotEnv, "OSS_LOCAL_MAX_MB"); maxMBStr != "" {
		if v, err := strconv.ParseInt(maxMBStr, 10, 64); err == nil && v > 0 {
			cfg.OSS.MaxLocalMB = v
		}
	}
	if cfg.OSS.MaxLocalMB <= 0 {
		cfg.OSS.MaxLocalMB = 8192
	}

	return &cfg, nil
}

func getEnvValue(dotEnv map[string]string, key string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return dotEnv[key]
}

func loadDotEnv(filename string) map[string]string {
	values := make(map[string]string)
	f, err := os.Open(filename)
	if err != nil {
		return values
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		values[key] = val
	}
	return values
}
