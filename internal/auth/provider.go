package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/oneslash/upwind-cli/internal/config"
)

type Provider struct {
	client *http.Client
	cfg    config.Runtime

	mu        sync.Mutex
	token     string
	tokenType string
	expiresAt time.Time
}

func NewProvider(client *http.Client, cfg config.Runtime) *Provider {
	return &Provider{
		client: client,
		cfg:    cfg,
	}
}

func (p *Provider) AuthorizationHeader(ctx context.Context) (string, error) {
	if token := strings.TrimSpace(p.cfg.AccessToken); token != "" {
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			return token, nil
		}
		return "Bearer " + token, nil
	}

	if p.cfg.ClientID == "" || p.cfg.ClientSecret == "" {
		return "", fmt.Errorf("missing credentials: set %s/%s or %s", config.EnvClientID, config.EnvClientSecret, config.EnvAccessToken)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && time.Until(p.expiresAt) > 30*time.Second {
		return p.tokenType + " " + p.token, nil
	}

	form := url.Values{}
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("audience", p.cfg.Audience)
	form.Set("grant_type", "client_credentials")

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.AuthURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := p.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	if response.StatusCode >= http.StatusBadRequest {
		var failure struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &failure); err == nil && failure.Error != "" {
			return "", fmt.Errorf("oauth token request failed: %s (%s)", failure.Error, failure.Description)
		}

		bodyText := strings.TrimSpace(string(body))
		if bodyText != "" {
			return "", fmt.Errorf("oauth token request failed with status %s: %s", response.Status, bodyText)
		}
		return "", fmt.Errorf("oauth token request failed with status %s", response.Status)
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode oauth token response: %w", err)
	}

	if payload.AccessToken == "" {
		return "", fmt.Errorf("oauth token response did not include access_token")
	}

	tokenType := strings.TrimSpace(payload.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}

	p.token = payload.AccessToken
	p.tokenType = tokenType
	if payload.ExpiresIn > 0 {
		p.expiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	} else {
		p.expiresAt = time.Now().Add(5 * time.Minute)
	}

	return p.tokenType + " " + p.token, nil
}
