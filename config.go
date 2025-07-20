package eotel

import "os"

type Config struct {
	ServiceName   string
	JobName       string
	SentryDSN     string
	SentryOrg     string
	LokiURL       string
	OtelCollector string
	EnableTracing bool
	EnableMetrics bool
	EnableSentry  bool
	EnableLoki    bool
	LogLevel      string
}

var globalCfg Config

func LoadConfigFromEnv() Config {
	return Config{
		ServiceName:   getEnv("SERVICE_NAME", "eotel"),
		JobName:       getEnv("JOB_NAME", "eotel-job"),
		SentryDSN:     getEnv("SENTRY_DSN", ""),
		SentryOrg:     getEnv("SENTRY_ORG", ""),
		LokiURL:       getEnv("LOKI_URL", "http://loki:3100/loki/api/v1/push"),
		OtelCollector: getEnv("OTEL_COLLECTOR", "otel-collector:4317"),
		EnableTracing: getEnvBool("ENABLE_TRACING", true),
		EnableMetrics: getEnvBool("ENABLE_METRICS", true),
		EnableSentry:  getEnvBool("ENABLE_SENTRY", true),
		EnableLoki:    getEnvBool("ENABLE_LOKI", true),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "true" || val == "1" {
		return true
	}
	if val == "false" || val == "0" {
		return false
	}
	return fallback
}
