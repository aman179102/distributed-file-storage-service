package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Storage  StorageConfig
	Auth     AuthConfig
	Metrics  MetricsConfig
	Tracing  TracingConfig
	Redis    RedisConfig
	Logging  LoggingConfig
}

type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	CORSOrigins     []string
	RateLimitPerIP  int
	RateLimitPerUser int
	MaxBodySize     int64
	APIBasePath     string
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	MigrationsDir   string
}

type StorageConfig struct {
	DataDir          string
	ChunkSize        int64
	ReplicationFactor int
	DataShards       int
	ParityShards     int
	MaxFileSize      int64
	TempDir          string
}

type AuthConfig struct {
	JWTSecret           string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	BcryptCost          int
	DefaultAdminUser    string
	DefaultAdminPassword string
}

type MetricsConfig struct {
	Enabled bool
	Path    string
}

type TracingConfig struct {
	Enabled     bool
	ServiceName string
	Endpoint    string
	SampleRate  float64
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type LoggingConfig struct {
	Level  string
	Format string
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
			CORSOrigins:     getEnvSlice("CORS_ORIGINS", []string{"*"}),
			RateLimitPerIP:  getEnvInt("RATE_LIMIT_PER_IP", 100),
			RateLimitPerUser: getEnvInt("RATE_LIMIT_PER_USER", 1000),
			MaxBodySize:     int64(getEnvInt("MAX_BODY_SIZE_MB", 5120)) * 1024 * 1024,
			APIBasePath:     getEnv("API_BASE_PATH", "/api/v1"),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "filestore"),
			Password:        getEnv("DB_PASSWORD", "filestore"),
			DBName:          getEnv("DB_NAME", "filestore"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			MigrationsDir:   getEnv("DB_MIGRATIONS_DIR", "src/infrastructure/database/migrations"),
		},
		Storage: StorageConfig{
			DataDir:           getEnv("STORAGE_DATA_DIR", "/data/storage"),
			ChunkSize:         int64(getEnvInt("STORAGE_CHUNK_SIZE_MB", 64)) * 1024 * 1024,
			ReplicationFactor: getEnvInt("STORAGE_REPLICATION_FACTOR", 3),
			DataShards:        getEnvInt("STORAGE_DATA_SHARDS", 4),
			ParityShards:      getEnvInt("STORAGE_PARITY_SHARDS", 2),
			MaxFileSize:       int64(getEnvInt("STORAGE_MAX_FILE_SIZE_GB", 512)) * 1024 * 1024 * 1024,
			TempDir:           getEnv("STORAGE_TEMP_DIR", "/tmp/filestore"),
		},
		Auth: AuthConfig{
			JWTSecret:           getEnv("AUTH_JWT_SECRET", "change-me-in-production"),
			AccessTokenTTL:      getEnvDuration("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL:     getEnvDuration("AUTH_REFRESH_TOKEN_TTL", 7*24*time.Hour),
			BcryptCost:          getEnvInt("AUTH_BCRYPT_COST", 12),
			DefaultAdminUser:    getEnv("AUTH_DEFAULT_ADMIN_USER", "admin"),
			DefaultAdminPassword: getEnv("AUTH_DEFAULT_ADMIN_PASSWORD", "admin123"),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvBool("METRICS_ENABLED", true),
			Path:    getEnv("METRICS_PATH", "/metrics"),
		},
		Tracing: TracingConfig{
			Enabled:     getEnvBool("TRACING_ENABLED", false),
			ServiceName: getEnv("TRACING_SERVICE_NAME", "distributed-file-storage"),
			Endpoint:    getEnv("TRACING_ENDPOINT", "http://localhost:4318"),
			SampleRate:  getEnvFloat("TRACING_SAMPLE_RATE", 0.1),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Storage.DataShards < 1 || c.Storage.ParityShards < 1 {
		return fmt.Errorf("data and parity shards must be >= 1")
	}
	if c.Auth.BcryptCost < 4 || c.Auth.BcryptCost > 31 {
		return fmt.Errorf("bcrypt cost must be between 4 and 31")
	}
	return nil
}

func (c *Config) DatabaseDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User, c.Database.Password, c.Database.DBName, c.Database.SSLMode,
	)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		result := []string{}
		for _, s := range splitAndTrim(v, ",") {
			if s != "" {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}

func splitAndTrim(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			if i > start {
				result = append(result, trimSpace(s[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, trimSpace(s[start:]))
	}
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
