package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpauth "github.com/freeDog-wy/go-backend-template/internal/app/mcp/auth"
	mcpclient "github.com/freeDog-wy/go-backend-template/internal/app/mcp/client"
	mcpconfig "github.com/freeDog-wy/go-backend-template/internal/app/mcp/config"
	mcpserver "github.com/freeDog-wy/go-backend-template/internal/app/mcp/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Optional path to the MCP configuration file")
	flag.Parse()

	cfg, err := mcpconfig.Load(configPath)
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("validate MCP configuration: %v", err)
	}

	httpClient := &http.Client{Timeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second}
	provider, err := mcpauth.New(cfg.CMSBaseURL, cfg.ClientID, cfg.ClientSecret, httpClient, cfg.AllowInsecureHTTP)
	if err != nil {
		log.Fatalf("initialize service token provider: %v", err)
	}
	client, err := mcpclient.New(cfg.CMSBaseURL, httpClient, provider, cfg.AllowInsecureHTTP)
	if err != nil {
		log.Fatalf("initialize CMS client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := mcpserver.New(client).Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Printf("MCP server stopped: %v", err)
	}
}
