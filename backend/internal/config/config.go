package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct{ Env, DataMode, ServerAddr, FrontendURL, DatabaseURL, JWTSecret string }

func Load() (Config, error) {
	loadDotEnv()
	c := Config{Env: env("APP_ENV", "development"), DataMode: strings.ToLower(env("DATA_MODE", "demo")), ServerAddr: env("SERVER_ADDR", "localhost:8080"), FrontendURL: env("FRONTEND_URL", "http://localhost:5173"), DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: env("JWT_SECRET", "development-secret-change-me")}
	if port := os.Getenv("PORT"); port != "" {
		c.ServerAddr = "0.0.0.0:" + port
	}
	if c.DataMode != "demo" && c.DataMode != "postgres" {
		return c, fmt.Errorf("DATA_MODE must be demo or postgres")
	}
	if c.DataMode == "postgres" {
		u, err := url.Parse(c.DatabaseURL)
		if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
			return c, fmt.Errorf("DATA_MODE=postgres requires a valid PostgreSQL DATABASE_URL")
		}
	}
	if c.Env == "production" && (c.JWTSecret == "development-secret-change-me" || len(c.JWTSecret) < 32) {
		return c, fmt.Errorf("JWT_SECRET must contain at least 32 characters in production")
	}
	return c, nil
}

func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || os.Getenv(key) != "" {
			continue
		}
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
