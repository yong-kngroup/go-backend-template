package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/freeDog-wy/go-backend-template/internal/app/sitegen"
)

func main() {
	var (
		apiBase     = flag.String("api-base", envOr("SITEGEN_CMS_API_BASE_URL", ""), "CMS public API base URL")
		siteURL     = flag.String("site-url", envOr("SITEGEN_SITE_URL", ""), "Public site URL")
		outputDir   = flag.String("out", envOr("SITEGEN_OUTPUT_DIR", "dist"), "Static output directory")
		perPage     = flag.Int("per-page", envInt("SITEGEN_PER_PAGE", 20), "CMS and static listing page size")
		concurrency = flag.Int("concurrency", envInt("SITEGEN_CONCURRENCY", 4), "Concurrent article detail requests")
		timeout     = flag.Duration("http-timeout", envDuration("SITEGEN_HTTP_TIMEOUT_SECONDS", 15*time.Second), "CMS request timeout")
		siteName    = flag.String("site-name", envOr("SITEGEN_SITE_NAME", "Content Site"), "Site name")
	)
	flag.Parse()

	cfg, err := sitegen.NewConfig(sitegen.ConfigInput{
		APIBaseURL:  *apiBase,
		SiteURL:     *siteURL,
		OutputDir:   *outputDir,
		PerPage:     *perPage,
		Concurrency: *concurrency,
		HTTPTimeout: *timeout,
		SiteName:    *siteName,
	})
	if err != nil {
		log.Fatalf("invalid site generator configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stats, err := sitegen.New(cfg).Build(ctx)
	if err != nil {
		log.Fatalf("build static site: %v", err)
	}
	log.Printf("static site built: locales=%d articles=%d pages=%d output=%s", stats.Locales, stats.Articles, stats.Pages, cfg.OutputDir)
}

func envOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s=%q; using %d\n", name, value, fallback)
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		fmt.Fprintf(os.Stderr, "invalid %s=%q; using %s\n", name, value, fallback)
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
