package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"go.yaml.in/yaml/v4"
)

func TestParseConfigYAMLDefaults(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Backend: &Backend{
			Target:    "http://localhost:9090",
			Readiness: "/-/ready",
			Liveness:  "/-/healthy",
		},
		Users: map[string]string{
			"alice": "$2a$10$Yb7v1f7Q1Wl1ye2b0kJ94eI9Jc1DSpP1Qx7Bz7m.cM0f5FZ9t1A8K",
		},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if got.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", got.Port)
	}
	if got.Backend == nil || got.Backend.Target != cfg.Backend.Target {
		t.Fatalf("backend target mismatch")
	}
}

func TestParseConfigJSON(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Port: 9090,
		Backend: &Backend{
			Target: "http://localhost:9091",
		},
		Users: map[string]string{
			"bob": "$2a$10$Yb7v1f7Q1Wl1ye2b0kJ94eI9Jc1DSpP1Qx7Bz7m.cM0f5FZ9t1A8K",
		},
	}

	data, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if got.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", got.Port)
	}
	if got.Backend == nil || got.Backend.Target != cfg.Backend.Target {
		t.Fatalf("backend target mismatch")
	}
}

func TestParseConfigUnsupportedExtension(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.txt")
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := ParseConfig(path); err == nil {
		t.Fatalf("expected error for unsupported extension")
	}
}

func TestVerifyUserPass(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	db := map[string]string{
		"alice": string(hash),
	}

	if !VerifyUserPass(db, "alice", "secret") {
		t.Fatalf("expected valid credentials")
	}
	if VerifyUserPass(db, "alice", "wrong") {
		t.Fatalf("expected invalid password")
	}
	if VerifyUserPass(db, "missing", "secret") {
		t.Fatalf("expected invalid user")
	}
}
