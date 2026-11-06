package config

import "testing"

func TestDefaultsAndPostgresValidation(t *testing.T) {
	for _, k := range []string{"APP_ENV", "DATA_MODE", "SERVER_ADDR", "PORT", "FRONTEND_URL", "DATABASE_URL", "JWT_SECRET"} {
		t.Setenv(k, "")
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DataMode != "demo" || c.ServerAddr != "localhost:8080" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	t.Setenv("DATA_MODE", "postgres")
	if _, err = Load(); err == nil {
		t.Fatal("postgres mode without DATABASE_URL must fail")
	}
	t.Setenv("DATABASE_URL", "postgres://user:pass@db.example:5432/campus?sslmode=require")
	if _, err = Load(); err != nil {
		t.Fatalf("valid postgres URL rejected: %v", err)
	}
}

func TestProductionRequiresStrongSecret(t *testing.T) {
	for _, k := range []string{"APP_ENV", "DATA_MODE", "SERVER_ADDR", "PORT", "FRONTEND_URL", "DATABASE_URL", "JWT_SECRET"} {
		t.Setenv(k, "")
	}
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("production default secret must fail")
	}
	t.Setenv("JWT_SECRET", "this-is-a-long-random-secret-for-test")
	if _, err := Load(); err != nil {
		t.Fatalf("strong production secret rejected: %v", err)
	}
}
