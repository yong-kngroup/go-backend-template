package mcpauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Provider struct {
	baseURL, clientID, clientSecret string
	httpClient                      *http.Client
	mu                              sync.Mutex
	token                           string
	expiresAt                       time.Time
}

func New(baseURL, clientID, clientSecret string, httpClient *http.Client, allowInsecureHTTP bool) (*Provider, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, err := url.Parse(baseURL)
	validScheme := u != nil && (u.Scheme == "https" || (allowInsecureHTTP && u.Scheme == "http"))
	if baseURL == "" || err != nil || !validScheme || u.Host == "" || strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" || httpClient == nil {
		return nil, fmt.Errorf("mcp cms base URL and service credentials are required")
	}
	return &Provider{baseURL: baseURL, clientID: clientID, clientSecret: clientSecret, httpClient: httpClient}, nil
}

func (p *Provider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Until(p.expiresAt) > 2*time.Minute {
		return p.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/v1/auth/service-token", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(p.clientID, p.clientSecret)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request service token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("service token HTTP %d", resp.StatusCode)
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		} `json:"data"`
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode service token: %w", err)
	}
	if !body.Success || body.Data.AccessToken == "" || body.Data.ExpiresIn <= 0 {
		code := "UNKNOWN"
		if body.Error != nil && body.Error.Code != "" {
			code = body.Error.Code
		}
		return "", fmt.Errorf("service token rejected: %s", code)
	}
	p.token = body.Data.AccessToken
	p.expiresAt = time.Now().Add(time.Duration(body.Data.ExpiresIn) * time.Second)
	return p.token, nil
}
