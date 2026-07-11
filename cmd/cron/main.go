package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	cfg := config.Load(configPath)

	cronApp := initCronApp(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type componentResult struct {
		name string
		err  error
	}
	errCh := make(chan componentResult, 2)
	go func() {
		errCh <- componentResult{name: "scheduler", err: cronApp.Run(ctx)}
	}()
	go func() {
		errCh <- componentResult{name: "probe server", err: cronApp.ServeProbe()}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	var terminalErr error
	received := 0
	select {
	case signal := <-quit:
		log.Printf("cron received signal %s, shutting down", signal)
	case result := <-errCh:
		received++
		terminalErr = componentError(result.name, result.err, false)
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := cronApp.Shutdown(shutdownCtx); err != nil && terminalErr == nil {
		terminalErr = fmt.Errorf("shutdown cron: %w", err)
	}

	for received < 2 {
		result := <-errCh
		received++
		if err := componentError(result.name, result.err, true); err != nil && terminalErr == nil {
			terminalErr = err
		}
	}
	if terminalErr != nil {
		log.Fatal(terminalErr)
	}
}

func componentError(name string, err error, stopping bool) error {
	if stopping && (err == nil || errors.Is(err, context.Canceled)) {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("%s stopped unexpectedly", name)
	}
	return fmt.Errorf("%s stopped: %w", name, err)
}
