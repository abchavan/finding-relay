// Package config reads the relay settings from the environment.
package config

import (
	"os"
	"strconv"
)

// Config holds everything the relay needs at startup.
type Config struct {
	NATSURL     string
	NATSUser    string
	NATSPass    string
	Stream      string
	Consumer    string
	Subjects    string
	TicketsURL  string
	TicketsTok  string
	TicketsRPS  float64
	ChatWebhook string
	Suppress    string
	AdminToken  string
	HTTPAddr    string
	Batch       int
}

// Load builds a Config from environment variables.
func Load() Config {
	return Config{
		NATSURL:     get("NATS_URL", "nats://127.0.0.1:4222"),
		NATSUser:    get("NATS_USER", ""),
		NATSPass:    get("NATS_PASSWORD", ""),
		Stream:      get("STREAM", "FINDINGS"),
		Consumer:    get("CONSUMER", "finding-relay"),
		Subjects:    get("SUBJECTS", "findings.>"),
		TicketsURL:  get("TICKETS_URL", ""),
		TicketsTok:  get("TICKETS_TOKEN", ""),
		TicketsRPS:  getFloat("TICKETS_RPS", 4),
		ChatWebhook: get("CHAT_WEBHOOK_URL", ""),
		Suppress:    get("SUPPRESS_FINGERPRINTS", ""),
		AdminToken:  get("ADMIN_TOKEN", "changeme"),
		HTTPAddr:    get("HTTP_ADDR", ":8080"),
		Batch:       getInt("BATCH", 1),
	}
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && v > 0 {
		return v
	}
	return def
}
