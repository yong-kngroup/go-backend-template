package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesMCPConfigAndEnvironment(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mcp.yaml")
	if err := os.WriteFile(configPath, []byte("cms_base_url: https://yaml.example.internal\nrequest_timeout_seconds: 5\nallow_insecure_http: false\nclient_id: ignored-from-yaml\nclient_secret: ignored-from-yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envCMSBaseURL, "https://environment.example.internal")
	t.Setenv(envCMSRequestTimeout, "15")
	t.Setenv(envCMSAllowInsecureHTTP, "true")
	t.Setenv(envCMSMCPClientID, "mcp-client")
	t.Setenv(envCMSMCPClientSecret, "mcp-secret")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CMSBaseURL != "https://environment.example.internal" || cfg.RequestTimeoutSeconds != 15 || !cfg.AllowInsecureHTTP {
		t.Fatalf("MCP endpoint settings = (%q, %d, %t), want environment overrides", cfg.CMSBaseURL, cfg.RequestTimeoutSeconds, cfg.AllowInsecureHTTP)
	}
	if cfg.ClientID != "mcp-client" || cfg.ClientSecret != "mcp-secret" {
		t.Fatalf("MCP credentials = %q, %q, want environment values", cfg.ClientID, cfg.ClientSecret)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadDoesNotReadDotEnvOrCredentialsFromYAML(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	if err := os.WriteFile(".env", []byte("CMS_BASE_URL=https://dotenv.example.internal\nCMS_MCP_CLIENT_ID=dotenv-client\nCMS_MCP_CLIENT_SECRET=dotenv-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(tempDir, "mcp.yaml")
	if err := os.WriteFile(configPath, []byte("cms_base_url: https://yaml.example.internal\nclient_id: ignored-from-yaml\nclient_secret: ignored-from-yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envCMSBaseURL, "")
	t.Setenv(envCMSRequestTimeout, "")
	t.Setenv(envCMSAllowInsecureHTTP, "")
	t.Setenv(envCMSMCPClientID, "")
	t.Setenv(envCMSMCPClientSecret, "")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CMSBaseURL != "" || cfg.ClientID != "" || cfg.ClientSecret != "" {
		t.Fatalf("MCP config unexpectedly read .env or YAML credentials: %#v", cfg)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing environment credentials error")
	}
}
