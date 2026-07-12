package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/freeDog-wy/go-backend-template/pkg/envfile"
	"github.com/spf13/viper"
)

type Config struct {
	App            AppConfig
	Server         ServerConfig
	Database       DatabaseConfig
	MQ             MQConfig
	Redis          RedisConfig
	RateLimit      RateLimitConfig `mapstructure:"rate_limit"`
	Worker         WorkerConfig
	Auth           AuthConfig
	Email          EmailConfig
	Captcha        CaptchaConfig
	Cron           CronConfig
	Tracing        TracingConfig
	BootstrapAdmin BootstrapAdminConfig `mapstructure:"bootstrap_admin"`
	Storage        StorageConfig
}
type StorageConfig struct{ R2 R2Config }
type R2Config struct {
	AccountID         string `mapstructure:"account_id"`
	AccessKeyID       string `mapstructure:"access_key_id"`
	SecretAccessKey   string `mapstructure:"secret_access_key"`
	Bucket            string
	PublicBaseURL     string `mapstructure:"public_base_url"`
	Prefix            string
	PresignTTLMinutes int `mapstructure:"presign_ttl_minutes"`
}

type AuthConfig struct {
	JWTIssuer             string
	JWTAudience           string
	JWTSecret             string
	AccessTokenTTLMinutes int
	RefreshTokenTTLHours  int
	LoginFailThreshold    int
}

type DatabaseConfig struct {
	DSN string
}

type MQConfig struct {
	EventsName string      `mapstructure:"events_name"`
	Kafka      KafkaConfig `mapstructure:"kafka"`
}

type KafkaConfig struct {
	Brokers  []string
	ClientID string `mapstructure:"client_id"`
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type RateLimitConfig struct {
	Enabled       bool
	Requests      int `mapstructure:"requests"`
	WindowSeconds int `mapstructure:"window_seconds"`
}

type AppConfig struct {
	Mode string
}

type WorkerConfig struct {
	Probe                         ProbeConfig             `mapstructure:"probe"`
	ConsumerGroup                 string                  `mapstructure:"consumer_group"`
	ConsumerMaxRetries            int                     `mapstructure:"consumer_max_retries"`
	ConsumerProcessingLockSeconds int                     `mapstructure:"consumer_processing_lock_seconds"`
	KafkaReadMinBytes             int                     `mapstructure:"kafka_read_min_bytes"`
	KafkaReadMaxBytes             int                     `mapstructure:"kafka_read_max_bytes"`
	KafkaMaxWaitSeconds           int                     `mapstructure:"kafka_max_wait_seconds"`
	KafkaRetryTopics              []KafkaRetryTopicConfig `mapstructure:"kafka_retry_topics"`
	KafkaDeadLetterTopic          string                  `mapstructure:"kafka_dead_letter_topic"`
}

type ProbeConfig struct {
	IP   string
	Port int
}

func (c ProbeConfig) Address() string {
	return net.JoinHostPort(c.IP, strconv.Itoa(c.Port))
}

type KafkaRetryTopicConfig struct {
	Topic        string `mapstructure:"topic"`
	DelaySeconds int    `mapstructure:"delay_seconds"`
}

type ServerConfig struct {
	IP             string
	Port           int
	ReadTimeout    int
	WriteTimeout   int
	TrustedProxies []string
}

type EmailConfig struct {
	SmtpHost     string
	SmtpPort     int
	SmtpUser     string
	SmtpPassword string
	FromAddress  string
	SiteBaseURL  string
}

type CaptchaConfig struct {
	Width  int
	Height int
	Length int
}

type CronConfig struct {
	Probe                              ProbeConfig `mapstructure:"probe"`
	Enabled                            bool
	OutboxPublishIntervalSeconds       int    `mapstructure:"outbox_publish_interval_seconds"`
	OutboxBatchSize                    int    `mapstructure:"outbox_batch_size"`
	VerificationCleanupIntervalSeconds int    `mapstructure:"verification_cleanup_interval_seconds"`
	DLQInspectionEnabled               bool   `mapstructure:"dlq_inspection_enabled"`
	DLQInspectionIntervalSeconds       int    `mapstructure:"dlq_inspection_interval_seconds"`
	DLQInspectionBatchSize             int    `mapstructure:"dlq_inspection_batch_size"`
	DLQInspectionGroup                 string `mapstructure:"dlq_inspection_group"`
	DLQReplayEnabled                   bool   `mapstructure:"dlq_replay_enabled"`
	DLQReplayIntervalSeconds           int    `mapstructure:"dlq_replay_interval_seconds"`
	DLQReplayBatchSize                 int    `mapstructure:"dlq_replay_batch_size"`
	DLQReplayGroup                     string `mapstructure:"dlq_replay_group"`
	DLQReplayTarget                    string `mapstructure:"dlq_replay_target"`
}

type TracingConfig struct {
	Endpoint string // Jaeger OTLP HTTP 地址，为空时回退到 stdout
}

type BootstrapAdminConfig struct {
	Enabled  bool
	Name     string
	Email    string
	Password string
}

func Load(configPath string) *Config {
	if err := envfile.Load(".env"); err != nil {
		panic(fmt.Errorf("failed to load .env: %w", err))
	}

	v := viper.New()

	// set default config
	v.SetDefault("app.mode", "development")
	v.SetDefault("server.ip", "localhost")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.readTimeout", 30)
	v.SetDefault("server.writeTimeout", 30)
	v.SetDefault("email.smtpPort", 465)
	v.SetDefault("email.siteBaseURL", "http://localhost:5173")
	v.SetDefault("mq.events_name", "domain.events")
	v.SetDefault("mq.kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("mq.kafka.client_id", "go-backend-template")
	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.requests", 20)
	v.SetDefault("rate_limit.window_seconds", 60)
	v.SetDefault("auth.jwtIssuer", "go-backend-template")
	v.SetDefault("auth.jwtAudience", "go-backend-template-client")
	v.SetDefault("auth.jwtSecret", "change-me")
	v.SetDefault("auth.accessTokenTTLMinutes", 15)
	v.SetDefault("auth.refreshTokenTTLHours", 24*7)
	v.SetDefault("auth.loginFailThreshold", 5)
	v.SetDefault("worker.consumer_group", "user-worker")
	v.SetDefault("worker.probe.ip", "0.0.0.0")
	v.SetDefault("worker.probe.port", 8081)
	v.SetDefault("worker.consumer_max_retries", 10)
	v.SetDefault("worker.consumer_processing_lock_seconds", 300)
	v.SetDefault("worker.kafka_read_min_bytes", 1024)
	v.SetDefault("worker.kafka_read_max_bytes", 10*1024*1024)
	v.SetDefault("worker.kafka_max_wait_seconds", 1)
	v.SetDefault("worker.kafka_retry_topics", []map[string]any{
		{"topic": "domain.events.retry.30s", "delay_seconds": 30},
		{"topic": "domain.events.retry.5m", "delay_seconds": 300},
		{"topic": "domain.events.retry.30m", "delay_seconds": 1800},
	})
	v.SetDefault("worker.kafka_dead_letter_topic", "domain.events.dlq")

	v.SetDefault("captcha.width", 120)
	v.SetDefault("captcha.height", 40)
	v.SetDefault("captcha.length", 6)
	v.SetDefault("cron.enabled", true)
	v.SetDefault("cron.probe.ip", "0.0.0.0")
	v.SetDefault("cron.probe.port", 8082)
	v.SetDefault("cron.outbox_publish_interval_seconds", 5)
	v.SetDefault("cron.outbox_batch_size", 100)
	v.SetDefault("cron.verification_cleanup_interval_seconds", 300)
	v.SetDefault("cron.dlq_inspection_enabled", true)
	v.SetDefault("cron.dlq_inspection_interval_seconds", 60)
	v.SetDefault("cron.dlq_inspection_batch_size", 50)
	v.SetDefault("cron.dlq_replay_enabled", false)
	v.SetDefault("cron.dlq_replay_interval_seconds", 300)
	v.SetDefault("cron.dlq_replay_batch_size", 20)
	v.SetDefault("cron.dlq_replay_target", "main")
	v.SetDefault("bootstrap_admin.enabled", false)
	v.SetDefault("bootstrap_admin.name", "Admin")
	v.SetDefault("bootstrap_admin.email", "")
	v.SetDefault("bootstrap_admin.password", "12345678")
	v.SetDefault("storage.r2.prefix", "cms")
	v.SetDefault("storage.r2.presign_ttl_minutes", 15)

	// load config file
	if configPath == "" {
		configPath = "config.yaml"
	}

	// If configPath contains a path separator, treat it as a direct file path.
	// Otherwise, search for it in known config directories.
	if strings.Contains(configPath, string(os.PathSeparator)) {
		v.SetConfigFile(configPath)
	} else {
		name := strings.TrimSuffix(configPath, ".yaml")
		v.SetConfigName(name)
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./internal/config")
		v.AddConfigPath("../internal/config")
	}

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("failed to read config file (%s): %v", configPath, err))
	}

	// Environment variables override file configuration. Explicitly setting the
	// keys keeps nested values reliable when unmarshalling into Config.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	applyEnvOverrides(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("failed to unmarshal config: %v", err))
	}

	if strings.TrimSpace(cfg.Cron.DLQInspectionGroup) == "" {
		cfg.Cron.DLQInspectionGroup = strings.TrimSpace(cfg.Worker.ConsumerGroup) + "-dlq-inspector"
	}
	if strings.TrimSpace(cfg.Cron.DLQReplayGroup) == "" {
		cfg.Cron.DLQReplayGroup = strings.TrimSpace(cfg.Worker.ConsumerGroup) + "-dlq-replay"
	}

	return &cfg
}

func applyEnvOverrides(v *viper.Viper) {
	for key, envKey := range configEnvBindings {
		if value, exists := os.LookupEnv(envKey); exists {
			v.Set(key, value)
		}
	}
}

var configEnvBindings = map[string]string{
	"app.mode":  "APP_MODE",
	"server.ip": "SERVER_IP", "server.port": "SERVER_PORT", "server.readTimeout": "SERVER_READ_TIMEOUT", "server.writeTimeout": "SERVER_WRITE_TIMEOUT", "server.trustedProxies": "SERVER_TRUSTED_PROXIES",
	"database.dsn":   "DATABASE_DSN",
	"mq.events_name": "MQ_EVENTS_NAME", "mq.kafka.brokers": "MQ_KAFKA_BROKERS", "mq.kafka.client_id": "MQ_KAFKA_CLIENT_ID",
	"redis.addr": "REDIS_ADDR", "redis.password": "REDIS_PASSWORD", "redis.db": "REDIS_DB",
	"rate_limit.enabled": "RATE_LIMIT_ENABLED", "rate_limit.requests": "RATE_LIMIT_REQUESTS", "rate_limit.window_seconds": "RATE_LIMIT_WINDOW_SECONDS",
	"worker.consumer_group": "WORKER_CONSUMER_GROUP", "worker.probe.ip": "WORKER_PROBE_IP", "worker.probe.port": "WORKER_PROBE_PORT", "worker.consumer_max_retries": "WORKER_CONSUMER_MAX_RETRIES", "worker.consumer_processing_lock_seconds": "WORKER_CONSUMER_PROCESSING_LOCK_SECONDS", "worker.kafka_read_min_bytes": "WORKER_KAFKA_READ_MIN_BYTES", "worker.kafka_read_max_bytes": "WORKER_KAFKA_READ_MAX_BYTES", "worker.kafka_max_wait_seconds": "WORKER_KAFKA_MAX_WAIT_SECONDS", "worker.kafka_dead_letter_topic": "WORKER_KAFKA_DEAD_LETTER_TOPIC",
	"auth.jwtIssuer": "AUTH_JWT_ISSUER", "auth.jwtAudience": "AUTH_JWT_AUDIENCE", "auth.jwtSecret": "AUTH_JWT_SECRET", "auth.accessTokenTTLMinutes": "AUTH_ACCESS_TOKEN_TTL_MINUTES", "auth.refreshTokenTTLHours": "AUTH_REFRESH_TOKEN_TTL_HOURS", "auth.loginFailThreshold": "AUTH_LOGIN_FAIL_THRESHOLD",
	"email.smtpHost": "EMAIL_SMTP_HOST", "email.smtpPort": "EMAIL_SMTP_PORT", "email.smtpUser": "EMAIL_SMTP_USER", "email.smtpPassword": "EMAIL_SMTP_PASSWORD", "email.fromAddress": "EMAIL_FROM_ADDRESS", "email.siteBaseURL": "EMAIL_SITE_BASE_URL",
	"captcha.width": "CAPTCHA_WIDTH", "captcha.height": "CAPTCHA_HEIGHT", "captcha.length": "CAPTCHA_LENGTH",
	"cron.enabled": "CRON_ENABLED", "cron.probe.ip": "CRON_PROBE_IP", "cron.probe.port": "CRON_PROBE_PORT", "cron.outbox_publish_interval_seconds": "CRON_OUTBOX_PUBLISH_INTERVAL_SECONDS", "cron.outbox_batch_size": "CRON_OUTBOX_BATCH_SIZE", "cron.verification_cleanup_interval_seconds": "CRON_VERIFICATION_CLEANUP_INTERVAL_SECONDS",
	"tracing.endpoint":        "TRACING_ENDPOINT",
	"bootstrap_admin.enabled": "BOOTSTRAP_ADMIN_ENABLED", "bootstrap_admin.name": "BOOTSTRAP_ADMIN_NAME", "bootstrap_admin.email": "BOOTSTRAP_ADMIN_EMAIL", "bootstrap_admin.password": "BOOTSTRAP_ADMIN_PASSWORD",
	"storage.r2.account_id": "STORAGE_R2_ACCOUNT_ID", "storage.r2.access_key_id": "STORAGE_R2_ACCESS_KEY_ID", "storage.r2.secret_access_key": "STORAGE_R2_SECRET_ACCESS_KEY", "storage.r2.bucket": "STORAGE_R2_BUCKET", "storage.r2.public_base_url": "STORAGE_R2_PUBLIC_BASE_URL", "storage.r2.prefix": "STORAGE_R2_PREFIX", "storage.r2.presign_ttl_minutes": "STORAGE_R2_PRESIGN_TTL_MINUTES",
}
