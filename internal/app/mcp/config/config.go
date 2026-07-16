package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

const (
	envCMSBaseURL             = "CMS_BASE_URL"
	envCMSRequestTimeout      = "CMS_REQUEST_TIMEOUT_SECONDS"
	envCMSAllowInsecureHTTP   = "CMS_ALLOW_INSECURE_HTTP"
	envCMSMCPClientID         = "CMS_MCP_CLIENT_ID"
	envCMSMCPClientSecret     = "CMS_MCP_CLIENT_SECRET"
	defaultRequestTimeoutSecs = 10
)

// Config contains only the settings required by the MCP HTTP client.
// Service-account lifecycle settings remain owned by the CMS server.
type Config struct {
	CMSBaseURL            string `mapstructure:"cms_base_url"`
	RequestTimeoutSeconds int    `mapstructure:"request_timeout_seconds"`
	AllowInsecureHTTP     bool   `mapstructure:"allow_insecure_http"`
	ClientID              string `mapstructure:"-"`
	ClientSecret          string `mapstructure:"-"`
}

// Load reads an optional MCP-only YAML file and MCP-specific environment
// variables. It deliberately does not load the application's .env file.
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetDefault("request_timeout_seconds", defaultRequestTimeoutSecs)

	if strings.TrimSpace(configPath) != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read MCP config file %s: %w", configPath, err)
		}
	}

	applyEnvOverrides(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal MCP configuration: %w", err)
	}
	cfg.ClientID = strings.TrimSpace(os.Getenv(envCMSMCPClientID))
	cfg.ClientSecret = os.Getenv(envCMSMCPClientSecret)
	return &cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.CMSBaseURL) == "" {
		return fmt.Errorf("%s is required", envCMSBaseURL)
	}
	if c.RequestTimeoutSeconds <= 0 {
		return fmt.Errorf("%s must be positive", envCMSRequestTimeout)
	}
	if c.ClientID == "" || strings.TrimSpace(c.ClientSecret) == "" {
		return fmt.Errorf("%s and %s are required", envCMSMCPClientID, envCMSMCPClientSecret)
	}
	return nil
}

func applyEnvOverrides(v *viper.Viper) {
	for key, envKey := range map[string]string{
		"cms_base_url":            envCMSBaseURL,
		"request_timeout_seconds": envCMSRequestTimeout,
		"allow_insecure_http":     envCMSAllowInsecureHTTP,
	} {
		if value, exists := os.LookupEnv(envKey); exists {
			v.Set(key, value)
		}
	}
}
