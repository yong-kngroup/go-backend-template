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

	"github.com/freeDog-wy/go-backend-template/internal/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to the configuration file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	app, err := initApp(cfg)
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}

	go func() {
		if err := app.Run(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
}
