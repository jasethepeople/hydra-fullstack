package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Alerts   AlertConfig
	Queue    QueueConfig
	Storage  StorageConfig
	NATS     NATSConfig
	Metrics  MetricsConfig
	Memorial MemorialConfig
}

type ServerConfig struct {
	Port         string
	Environment  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type AlertConfig struct {
	DiscordWebhook   string
	SMTPHost         string
	SMTPPort         int
	SMTPUser         string
	SMTPPass         string
	AlertEmail       string
	SlackWebhook     string
	PagerDutyKey     string
	MemoryWarning    float64
	MemoryCritical   float64
	LatencyWarning   float64
	LatencyCritical  float64
	ErrorWarning     float64
	ErrorCritical    float64
}

type QueueConfig struct {
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	WorkerCount    int
	QueueName      string
	DeadLetterName string
}

type StorageConfig struct {
	SQLitePath     string
	MaxConnections int
	EnableWAL      bool
}

type NATSConfig struct {
	URL            string
	StreamName     string
	ConsumerName   string
	Enabled        bool
}

type MetricsConfig struct {
	Enabled        bool
	Port           string
	Path           string
}

type MemorialConfig struct {
	SeasonalEnabled    bool
	DefaultSeason      string
	UploadMaxSizeMB    int
	AllowedTypes       []string
	WebSocketEnabled   bool
	CanvasEnabled      bool
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			Environment:  getEnv("ENV", "development"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Alerts: AlertConfig{
			DiscordWebhook:  getEnv("DISCORD_WEBHOOK", ""),
			SMTPHost:        getEnv("SMTP_HOST", ""),
			SMTPPort:        getInt("SMTP_PORT", 587),
			SMTPUser:        getEnv("SMTP_USER", ""),
			SMTPPass:        getEnv("SMTP_PASS", ""),
			AlertEmail:      getEnv("ALERT_EMAIL", ""),
			SlackWebhook:    getEnv("SLACK_WEBHOOK", ""),
			PagerDutyKey:    getEnv("PAGERDUTY_KEY", ""),
			MemoryWarning:   getFloat("MEMORY_WARNING_THRESHOLD", 80.0),
			MemoryCritical:  getFloat("MEMORY_CRITICAL_THRESHOLD", 92.0),
			LatencyWarning:  getFloat("LATENCY_WARNING_THRESHOLD", 3.0),
			LatencyCritical: getFloat("LATENCY_CRITICAL_THRESHOLD", 5.0),
			ErrorWarning:    getFloat("ERROR_WARNING_THRESHOLD", 5.0),
			ErrorCritical:   getFloat("ERROR_CRITICAL_THRESHOLD", 15.0),
		},
		Queue: QueueConfig{
			RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
			RedisPassword:  getEnv("REDIS_PASSWORD", ""),
			RedisDB:        getInt("REDIS_DB", 0),
			WorkerCount:    getInt("WORKER_COUNT", 4),
			QueueName:      getEnv("QUEUE_NAME", "hydra:render:queue"),
			DeadLetterName: getEnv("DEAD_LETTER_NAME", "hydra:render:deadletter"),
		},
		Storage: StorageConfig{
			SQLitePath:     getEnv("SQLITE_PATH", "./hydra.db"),
			MaxConnections: getInt("DB_MAX_CONNS", 25),
			EnableWAL:      getBool("DB_WAL_ENABLED", true),
		},
		NATS: NATSConfig{
			URL:          getEnv("NATS_URL", "nats://localhost:4222"),
			StreamName:   getEnv("NATS_STREAM", "HYDRA_EVENTS"),
			ConsumerName: getEnv("NATS_CONSUMER", "hydra-worker"),
			Enabled:      getBool("NATS_ENABLED", false),
		},
		Metrics: MetricsConfig{
			Enabled: getBool("METRICS_ENABLED", true),
			Port:    getEnv("METRICS_PORT", "9090"),
			Path:    getEnv("METRICS_PATH", "/metrics"),
		},
		Memorial: MemorialConfig{
			SeasonalEnabled:  getBool("MEMORIAL_SEASONAL", true),
			DefaultSeason:    getEnv("MEMORIAL_DEFAULT_SEASON", "auto"),
			UploadMaxSizeMB:  getInt("MEMORIAL_MAX_UPLOAD_MB", 50),
			AllowedTypes:     strings.Split(getEnv("MEMORIAL_ALLOWED_TYPES", "image/jpeg,image/png,image/gif,video/mp4"), ","),
			WebSocketEnabled: getBool("MEMORIAL_WEBSOCKET", true),
			CanvasEnabled:    getBool("MEMORIAL_CANVAS", true),
		},
	}

	// Validate critical configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	log.Info().
		Str("environment", cfg.Server.Environment).
		Str("port", cfg.Server.Port).
		Bool("discord", cfg.Alerts.DiscordWebhook != "").
		Bool("nats", cfg.NATS.Enabled).
		Bool("metrics", cfg.Metrics.Enabled).
		Msg("configuration loaded")

	return cfg, nil
}

// Validate checks critical configuration values
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}

	if c.Queue.WorkerCount < 1 || c.Queue.WorkerCount > 100 {
		return fmt.Errorf("worker count must be between 1 and 100, got %d", c.Queue.WorkerCount)
	}

	if c.Memorial.UploadMaxSizeMB < 1 || c.Memorial.UploadMaxSizeMB > 500 {
		return fmt.Errorf("upload max size must be between 1 and 500 MB")
	}

	return nil
}

// IsProduction returns true if running in production
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}

// AlertChannels returns list of configured alert channels
func (c *Config) AlertChannels() []string {
	channels := []string{}
	if c.Alerts.DiscordWebhook != "" {
		channels = append(channels, "discord")
	}
	if c.Alerts.SMTPHost != "" {
		channels = append(channels, "smtp")
	}
	if c.Alerts.SlackWebhook != "" {
		channels = append(channels, "slack")
	}
	if c.Alerts.PagerDutyKey != "" {
		channels = append(channels, "pagerduty")
	}
	return channels
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
