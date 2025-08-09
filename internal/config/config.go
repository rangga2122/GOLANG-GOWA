package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App       AppConfig
	Database  DatabaseConfig
	WhatsApp  WhatsAppConfig
	Broadcast BroadcastConfig
	Scheduler SchedulerConfig
}

type AppConfig struct {
	Port      string
	Debug     bool
	OS        string
	BasicAuth string
	BasePath  string
}

type DatabaseConfig struct {
	URI string
}

type WhatsAppConfig struct {
	AutoReply           string
	AutoMarkRead        bool
	Webhook             string
	WebhookSecret       string
	AccountValidation   bool
	ChatStorage         bool
}

type BroadcastConfig struct {
	RateLimit     int
	DelayMS       int
	MaxRecipients int
}

type SchedulerConfig struct {
	Enabled  bool
	Timezone string
}

func Load() *Config {
	return &Config{
		App: AppConfig{
			Port:      getEnv("APP_PORT", "3000"),
			Debug:     getEnvBool("APP_DEBUG", false),
			OS:        getEnv("APP_OS", "GOWA-Broadcast"),
			BasicAuth: getEnv("APP_BASIC_AUTH", ""),
			BasePath:  getEnv("APP_BASE_PATH", ""),
		},
		Database: DatabaseConfig{
			URI: getEnv("DB_URI", "file:storages/whatsapp.db?_foreign_keys=on"),
		},
		WhatsApp: WhatsAppConfig{
			AutoReply:         getEnv("WHATSAPP_AUTO_REPLY", ""),
			AutoMarkRead:      getEnvBool("WHATSAPP_AUTO_MARK_READ", false),
			Webhook:           getEnv("WHATSAPP_WEBHOOK", ""),
			WebhookSecret:     getEnv("WHATSAPP_WEBHOOK_SECRET", "secret"),
			AccountValidation: getEnvBool("WHATSAPP_ACCOUNT_VALIDATION", true),
			ChatStorage:       getEnvBool("WHATSAPP_CHAT_STORAGE", true),
		},
		Broadcast: BroadcastConfig{
			RateLimit:     getEnvInt("BROADCAST_RATE_LIMIT", 10),
			DelayMS:       getEnvInt("BROADCAST_DELAY_MS", 1000),
			MaxRecipients: getEnvInt("BROADCAST_MAX_RECIPIENTS", 100),
		},
		Scheduler: SchedulerConfig{
			Enabled:  getEnvBool("SCHEDULER_ENABLED", true),
			Timezone: getEnv("SCHEDULER_TIMEZONE", "Asia/Jakarta"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// ParseBasicAuth parses basic auth string into map
func (c *AppConfig) ParseBasicAuth() map[string]string {
	auth := make(map[string]string)
	if c.BasicAuth == "" {
		return auth
	}

	pairs := strings.Split(c.BasicAuth, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			auth[parts[0]] = parts[1]
		}
	}
	return auth
}

// ParseWebhooks parses webhook URLs
func (c *WhatsAppConfig) ParseWebhooks() []string {
	if c.Webhook == "" {
		return []string{}
	}
	return strings.Split(c.Webhook, ",")
}