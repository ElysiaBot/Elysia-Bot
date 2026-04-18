package runtimecore

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Runtime RuntimeConfig `yaml:"runtime"`
	AIChat  AIChatConfig  `yaml:"ai_chat,omitempty"`
	Secrets SecretsConfig `yaml:"secrets,omitempty"`
	RBAC    *RBACConfig   `yaml:"rbac,omitempty"`
}

type RuntimeConfig struct {
	Environment         string               `yaml:"environment"`
	LogLevel            string               `yaml:"log_level"`
	HTTPPort            int                  `yaml:"http_port"`
	SQLitePath          string               `yaml:"sqlite_path,omitempty"`
	SmokeStoreBackend   string               `yaml:"smoke_store_backend,omitempty"`
	PostgresDSN         string               `yaml:"postgres_dsn,omitempty"`
	SchedulerIntervalMs int                  `yaml:"scheduler_interval_ms,omitempty"`
	BotInstances        []RuntimeBotInstance `yaml:"bot_instances,omitempty"`
}

type RuntimeBotInstance struct {
	ID       string `yaml:"id,omitempty"`
	Adapter  string `yaml:"adapter,omitempty"`
	Source   string `yaml:"source,omitempty"`
	Platform string `yaml:"platform,omitempty"`
	DemoPath string `yaml:"demo_path,omitempty"`
	SelfID   int64  `yaml:"self_id,omitempty"`
}

type AIChatConfig struct {
	Provider         string `yaml:"provider,omitempty"`
	Endpoint         string `yaml:"endpoint,omitempty"`
	Model            string `yaml:"model,omitempty"`
	RequestTimeoutMs int    `yaml:"request_timeout_ms,omitempty"`
}

type SecretsConfig struct {
	WebhookTokenRef string `yaml:"webhook_token_ref,omitempty"`
	AIChatAPIKeyRef string `yaml:"ai_chat_api_key_ref,omitempty"`
}

type RBACConfig struct {
	ActorRoles            map[string][]string                      `yaml:"actor_roles,omitempty"`
	Policies              map[string]pluginsdk.AuthorizationPolicy `yaml:"policies,omitempty"`
	ConsoleReadPermission string                                   `yaml:"console_read_permission,omitempty"`
}

func (c *RBACConfig) Validate() error {
	if c == nil {
		return nil
	}
	for role, policy := range c.Policies {
		for _, permission := range policy.Permissions {
			if err := validateRBACPermission(permission); err != nil {
				return fmt.Errorf("invalid rbac policy %q: %w", role, err)
			}
		}
	}
	if c.ConsoleReadPermission != "" {
		if err := validateRBACPermission(c.ConsoleReadPermission); err != nil {
			return fmt.Errorf("invalid console_read_permission: %w", err)
		}
	}
	return nil
}

func validateRBACPermission(permission string) error {
	trimmed := strings.TrimSpace(permission)
	if trimmed == "" {
		return fmt.Errorf("permission is required")
	}
	if trimmed != permission {
		return fmt.Errorf("permission %q must not contain surrounding whitespace", permission)
	}
	resource, action, found := strings.Cut(trimmed, ":")
	if !found || strings.Contains(action, ":") {
		return fmt.Errorf("permission %q must use resource:action format", permission)
	}
	if strings.TrimSpace(resource) == "" || strings.TrimSpace(action) == "" {
		return fmt.Errorf("permission %q must use resource:action format", permission)
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal yaml: %w", err)
	}
	if cfg.Secrets.WebhookTokenRef != "" {
		if err := ValidateSecretRef(cfg.Secrets.WebhookTokenRef); err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", WebhookSecretMainPathContract().ConfigRef, err)
		}
	}
	if cfg.Secrets.AIChatAPIKeyRef != "" {
		if err := ValidateSecretRef(cfg.Secrets.AIChatAPIKeyRef); err != nil {
			return Config{}, fmt.Errorf("invalid %s: %w", AIChatAPIKeySecretConfigRef(), err)
		}
	}
	if err := cfg.RBAC.Validate(); err != nil {
		return Config{}, err
	}

	applyEnvOverride(&cfg)
	applyRuntimeDefaults(&cfg)
	if err := validateAIChatConfig(cfg.AIChat); err != nil {
		return Config{}, err
	}
	if err := validateRuntimeBotInstances(cfg.Runtime.BotInstances); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnvOverride(cfg *Config) {
	if value := strings.TrimSpace(os.Getenv("BOT_PLATFORM_RUNTIME_ENVIRONMENT")); value != "" {
		cfg.Runtime.Environment = value
	}
	if value := strings.TrimSpace(os.Getenv("BOT_PLATFORM_RUNTIME_LOG_LEVEL")); value != "" {
		cfg.Runtime.LogLevel = value
	}
	if value := strings.TrimSpace(os.Getenv("BOT_PLATFORM_RUNTIME_HTTP_PORT")); value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			cfg.Runtime.HTTPPort = port
		}
	}
	if value := strings.TrimSpace(os.Getenv("BOT_PLATFORM_RUNTIME_SQLITE_PATH")); value != "" {
		cfg.Runtime.SQLitePath = value
	}
	if value := strings.TrimSpace(os.Getenv("BOT_PLATFORM_RUNTIME_SMOKE_STORE_BACKEND")); value != "" {
		cfg.Runtime.SmokeStoreBackend = value
	}
	if value := strings.TrimSpace(os.Getenv("BOT_PLATFORM_RUNTIME_POSTGRES_DSN")); value != "" {
		cfg.Runtime.PostgresDSN = value
	}
	if value := strings.TrimSpace(os.Getenv("BOT_PLATFORM_RUNTIME_SCHEDULER_INTERVAL_MS")); value != "" {
		if interval, err := strconv.Atoi(value); err == nil {
			cfg.Runtime.SchedulerIntervalMs = interval
		}
	}
}

func applyRuntimeDefaults(cfg *Config) {
	if cfg.Runtime.SQLitePath == "" {
		cfg.Runtime.SQLitePath = filepath.Join("data", "dev", "runtime.sqlite")
	}
	cfg.AIChat.Provider = strings.ToLower(strings.TrimSpace(cfg.AIChat.Provider))
	if cfg.AIChat.Provider == "" {
		cfg.AIChat.Provider = "mock"
	}
	cfg.AIChat.Endpoint = strings.TrimSpace(cfg.AIChat.Endpoint)
	cfg.AIChat.Model = strings.TrimSpace(cfg.AIChat.Model)
	if cfg.AIChat.RequestTimeoutMs <= 0 {
		cfg.AIChat.RequestTimeoutMs = 30000
	}
	cfg.Runtime.SmokeStoreBackend = strings.ToLower(strings.TrimSpace(cfg.Runtime.SmokeStoreBackend))
	cfg.Runtime.PostgresDSN = strings.TrimSpace(cfg.Runtime.PostgresDSN)
	if cfg.Runtime.SmokeStoreBackend == "" {
		cfg.Runtime.SmokeStoreBackend = "sqlite"
	}
	if cfg.Runtime.SchedulerIntervalMs <= 0 {
		cfg.Runtime.SchedulerIntervalMs = 100
	}
	for index := range cfg.Runtime.BotInstances {
		cfg.Runtime.BotInstances[index].ID = strings.TrimSpace(cfg.Runtime.BotInstances[index].ID)
		cfg.Runtime.BotInstances[index].Adapter = strings.ToLower(strings.TrimSpace(cfg.Runtime.BotInstances[index].Adapter))
		if cfg.Runtime.BotInstances[index].Adapter == "" {
			cfg.Runtime.BotInstances[index].Adapter = "onebot"
		}
		cfg.Runtime.BotInstances[index].Source = strings.TrimSpace(cfg.Runtime.BotInstances[index].Source)
		if cfg.Runtime.BotInstances[index].Source == "" {
			cfg.Runtime.BotInstances[index].Source = "onebot"
		}
		cfg.Runtime.BotInstances[index].Platform = strings.TrimSpace(cfg.Runtime.BotInstances[index].Platform)
		if cfg.Runtime.BotInstances[index].Platform == "" {
			cfg.Runtime.BotInstances[index].Platform = "onebot/v11"
		}
		cfg.Runtime.BotInstances[index].DemoPath = strings.TrimSpace(cfg.Runtime.BotInstances[index].DemoPath)
		if cfg.Runtime.BotInstances[index].DemoPath == "" {
			cfg.Runtime.BotInstances[index].DemoPath = "/demo/onebot/message"
		}
	}
}

func validateAIChatConfig(cfg AIChatConfig) error {
	switch cfg.Provider {
	case "mock", "openai_compat":
	default:
		return fmt.Errorf("ai_chat.provider %q is unsupported; must be \"mock\" or \"openai_compat\"", cfg.Provider)
	}
	if cfg.Provider != "openai_compat" {
		return nil
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("ai_chat.endpoint is required when ai_chat.provider=openai_compat")
	}
	parsed, err := url.ParseRequestURI(cfg.Endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("ai_chat.endpoint must be a valid absolute URL when ai_chat.provider=openai_compat")
	}
	if parsed.Scheme != "https" && !isLoopbackAIChatEndpoint(parsed) {
		return fmt.Errorf("ai_chat.endpoint must use https unless it targets localhost or loopback when ai_chat.provider=openai_compat")
	}
	if cfg.Model == "" {
		return fmt.Errorf("ai_chat.model is required when ai_chat.provider=openai_compat")
	}
	return nil
}

func isLoopbackAIChatEndpoint(parsed *url.URL) bool {
	if parsed == nil || parsed.Scheme != "http" {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateRuntimeBotInstances(instances []RuntimeBotInstance) error {
	seen := make(map[string]struct{}, len(instances))
	for index, instance := range instances {
		if instance.ID == "" {
			return fmt.Errorf("runtime.bot_instances[%d].id is required", index)
		}
		if instance.Adapter != "onebot" {
			return fmt.Errorf("runtime.bot_instances[%d].adapter %q is unsupported; only \"onebot\" is currently supported", index, instance.Adapter)
		}
		if _, exists := seen[instance.ID]; exists {
			return fmt.Errorf("runtime.bot_instances[%d].id %q must be unique", index, instance.ID)
		}
		seen[instance.ID] = struct{}{}
	}
	return nil
}
