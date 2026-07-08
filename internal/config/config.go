package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App            AppConfig
	Server         ServerConfig
	Database       DatabaseConfig
	Redis          RedisConfig
	Auth           AuthConfig
	Email          EmailConfig
	Captcha        CaptchaConfig
	Cron           CronConfig
	Tracing        TracingConfig
	BootstrapAdmin BootstrapAdminConfig `mapstructure:"bootstrap_admin"`
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

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AppConfig struct {
	Mode string
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
	Enabled                            bool
	VerificationCleanupIntervalSeconds int `mapstructure:"verification_cleanup_interval_seconds"`
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
	v := viper.New()

	// set default config
	v.SetDefault("app.mode", "development")
	v.SetDefault("server.ip", "localhost")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.readTimeout", 30)
	v.SetDefault("server.writeTimeout", 30)
	v.SetDefault("email.smtpPort", 465)
	v.SetDefault("email.siteBaseURL", "http://localhost:5173")
	v.SetDefault("auth.jwtIssuer", "go-backend-template")
	v.SetDefault("auth.jwtAudience", "go-backend-template-client")
	v.SetDefault("auth.jwtSecret", "change-me")
	v.SetDefault("auth.accessTokenTTLMinutes", 15)
	v.SetDefault("auth.refreshTokenTTLHours", 24*7)
	v.SetDefault("auth.loginFailThreshold", 5)

	v.SetDefault("captcha.width", 120)
	v.SetDefault("captcha.height", 40)
	v.SetDefault("captcha.length", 6)
	v.SetDefault("cron.enabled", true)
	v.SetDefault("cron.verification_cleanup_interval_seconds", 300)
	v.SetDefault("bootstrap_admin.enabled", false)
	v.SetDefault("bootstrap_admin.name", "Admin")
	v.SetDefault("bootstrap_admin.email", "")
	v.SetDefault("bootstrap_admin.password", "12345678")

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

	// load Env
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("failed to unmarshal config: %v", err))
	}

	return &cfg
}
