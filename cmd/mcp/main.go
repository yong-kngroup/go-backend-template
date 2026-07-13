package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
	"github.com/freeDog-wy/go-backend-template/internal/mcpauth"
	"github.com/freeDog-wy/go-backend-template/internal/mcpclient"
	"github.com/freeDog-wy/go-backend-template/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	if err := validate(cfg.MCP); err != nil {
		log.Fatalf("validate MCP configuration: %v", err)
	}

	httpClient := &http.Client{Timeout: time.Duration(cfg.MCP.RequestTimeoutSeconds) * time.Second}
	provider, err := mcpauth.New(cfg.MCP.CMSBaseURL, cfg.MCP.ClientID, cfg.MCP.ClientSecret, httpClient)
	if err != nil {
		log.Fatalf("initialize service token provider: %v", err)
	}
	client, err := mcpclient.New(cfg.MCP.CMSBaseURL, httpClient, provider)
	if err != nil {
		log.Fatalf("initialize CMS client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := mcpserver.New(client).Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Printf("MCP server stopped: %v", err)
	}
}

func validate(cfg config.MCPConfig) error {
	if !cfg.Enabled {
		return fmt.Errorf("mcp.enabled must be true")
	}
	if cfg.RequestTimeoutSeconds <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	if cfg.AccessTokenTTLMinutes <= 2 {
		return fmt.Errorf("service token TTL must exceed the two-minute refresh window")
	}
	return nil
}
