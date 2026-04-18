package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	runtimecore "github.com/ohmyopencode/bot-platform/packages/runtime-core"
)

type aiProviderSecretResolver interface {
	Resolve(context.Context, string, string) (string, error)
}

type aiProviderHTTPConfig struct {
	Endpoint       string
	Model          string
	Timeout        time.Duration
	APIKey         string
	Logger         *runtimecore.Logger
	Consumer       string
	ProviderKind   string
	RequestTimeout int
	HTTPClient     *http.Client
}

type aiProviderHTTP struct {
	endpoint string
	model    string
	client   *http.Client
	logger   *runtimecore.Logger
}

type aiProviderHTTPRequest struct {
	Model    string                  `json:"model"`
	Messages []aiProviderHTTPMessage `json:"messages"`
}

type aiProviderHTTPMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiProviderHTTPResponse struct {
	Choices []struct {
		Message aiProviderHTTPMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func newAIProviderHTTP(cfg aiProviderHTTPConfig) (*aiProviderHTTP, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("ai_chat.endpoint is required when ai_chat.provider=openai_compat")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("ai_chat.model is required when ai_chat.provider=openai_compat")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("ai chat provider api key is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	client.CheckRedirect = rejectAIProviderRedirect
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = aiProviderHTTPAuthTransport{apiKey: cfg.APIKey, base: transport}
	if cfg.Logger != nil {
		_ = cfg.Logger.Log("info", "runtime ai provider configured", runtimecore.LogContext{}, map[string]any{
			"provider":           cfg.ProviderKind,
			"endpoint":           endpoint,
			"model":              model,
			"request_timeout_ms": cfg.RequestTimeout,
			"consumer":           cfg.Consumer,
		})
	}
	return &aiProviderHTTP{endpoint: endpoint, model: model, client: client, logger: cfg.Logger}, nil
}

func rejectAIProviderRedirect(req *http.Request, via []*http.Request) error {
	if req == nil || req.URL == nil {
		return errors.New("openai_compat request failed: redirects are not allowed")
	}
	return fmt.Errorf("openai_compat request failed: redirects are not allowed for %s", req.URL.String())
}

func (p *aiProviderHTTP) Generate(ctx context.Context, prompt string) (string, error) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "", fmt.Errorf("empty prompt")
	}
	body, err := json.Marshal(aiProviderHTTPRequest{
		Model: p.model,
		Messages: []aiProviderHTTPMessage{{
			Role:    "user",
			Content: trimmed,
		}},
	})
	if err != nil {
		return "", fmt.Errorf("marshal ai request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build ai request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		providerErr := err
		if errors.Is(err, context.DeadlineExceeded) {
			providerErr = context.DeadlineExceeded
		} else if urlErr := new(url.Error); errors.As(err, &urlErr) && urlErr.Err != nil {
			providerErr = urlErr.Err
		}
		if p.logger != nil {
			_ = p.logger.Log("error", "runtime ai provider request failed", runtimecore.LogContext{}, map[string]any{
				"provider": "openai_compat",
				"endpoint": p.endpoint,
				"model":    p.model,
				"error":    providerErr.Error(),
			})
		}
		return "", fmt.Errorf("openai_compat request failed: %w", providerErr)
	}
	defer response.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read ai response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(rawBody))
		var failure aiProviderHTTPResponse
		if json.Unmarshal(rawBody, &failure) == nil && failure.Error != nil && strings.TrimSpace(failure.Error.Message) != "" {
			message = strings.TrimSpace(failure.Error.Message)
		}
		if message == "" {
			message = response.Status
		}
		if p.logger != nil {
			_ = p.logger.Log("error", "runtime ai provider returned non-success response", runtimecore.LogContext{}, map[string]any{
				"provider":    "openai_compat",
				"endpoint":    p.endpoint,
				"model":       p.model,
				"status_code": response.StatusCode,
				"error":       message,
			})
		}
		return "", fmt.Errorf("openai_compat request failed: status %d: %s", response.StatusCode, message)
	}
	var payload aiProviderHTTPResponse
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return "", fmt.Errorf("decode ai response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("openai_compat response missing choices")
	}
	content := strings.TrimSpace(payload.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("openai_compat response missing message content")
	}
	return content, nil
}

type aiProviderHTTPAuthTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t aiProviderHTTPAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	cloned.Header.Set("Authorization", "Bearer "+t.apiKey)
	return t.base.RoundTrip(cloned)
}
