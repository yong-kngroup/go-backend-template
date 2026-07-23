package sitegen

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPerPage     = 20
	defaultConcurrency = 4
	defaultTimeout     = 15 * time.Second
)

// Config contains the build-time inputs for a static site generation run.
type Config struct {
	APIBaseURL  *url.URL
	SiteURL     *url.URL
	OutputDir   string
	PerPage     int
	Concurrency int
	HTTPTimeout time.Duration
	SiteName    string
}

// ConfigInput keeps command-line and environment parsing outside the builder.
type ConfigInput struct {
	APIBaseURL  string
	SiteURL     string
	OutputDir   string
	PerPage     int
	Concurrency int
	HTTPTimeout time.Duration
	SiteName    string
}

func NewConfig(input ConfigInput) (Config, error) {
	apiURL, err := parseAbsoluteURL("CMS API base URL", input.APIBaseURL)
	if err != nil {
		return Config{}, err
	}
	siteURL, err := parseAbsoluteURL("site URL", input.SiteURL)
	if err != nil {
		return Config{}, err
	}
	if siteURL.RawQuery != "" || siteURL.Fragment != "" || siteURL.Path != "" && siteURL.Path != "/" {
		return Config{}, fmt.Errorf("site URL must not contain a path, query, or fragment")
	}
	siteURL.Path = ""

	outputDir := strings.TrimSpace(input.OutputDir)
	if outputDir == "" {
		outputDir = "dist"
	}
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve output directory: %w", err)
	}
	workingDir, err := filepath.Abs(".")
	if err != nil {
		return Config{}, fmt.Errorf("resolve working directory: %w", err)
	}
	volumeRoot := filepath.VolumeName(absOutput) + string(filepath.Separator)
	if absOutput == workingDir || absOutput == volumeRoot {
		return Config{}, fmt.Errorf("output directory must not be the working directory or filesystem root")
	}

	perPage := input.PerPage
	if perPage == 0 {
		perPage = defaultPerPage
	}
	if perPage < 1 || perPage > 100 {
		return Config{}, fmt.Errorf("per-page must be between 1 and 100")
	}
	concurrency := input.Concurrency
	if concurrency == 0 {
		concurrency = defaultConcurrency
	}
	if concurrency < 1 || concurrency > 16 {
		return Config{}, fmt.Errorf("concurrency must be between 1 and 16")
	}
	timeout := input.HTTPTimeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if timeout <= 0 {
		return Config{}, fmt.Errorf("HTTP timeout must be positive")
	}
	siteName := strings.TrimSpace(input.SiteName)
	if siteName == "" {
		siteName = "Content Site"
	}

	return Config{
		APIBaseURL:  apiURL,
		SiteURL:     siteURL,
		OutputDir:   absOutput,
		PerPage:     perPage,
		Concurrency: concurrency,
		HTTPTimeout: timeout,
		SiteName:    siteName,
	}, nil
}

func parseAbsoluteURL(name, value string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%s must be an absolute HTTP URL", name)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("%s must use HTTP or HTTPS", name)
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

func (c Config) absoluteURL(route string) string {
	u := *c.SiteURL
	u.Path = "/" + strings.TrimLeft(route, "/")
	return u.String()
}
