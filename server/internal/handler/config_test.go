package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetConfigIncludesRuntimeAuthConfig(t *testing.T) {
	origStorage := testHandler.Storage
	testHandler.Storage = &mockStorage{}
	defer func() { testHandler.Storage = origStorage }()

	t.Setenv("ALLOW_SIGNUP", "false")
	t.Setenv("GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("MULTICA_PRIVATE_LOGIN_CODE", "123456")
	t.Setenv("POSTHOG_API_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://eu.i.posthog.com")

	origCfg := testHandler.cfg
	testHandler.cfg = Config{
		AllowSignup:   false,
		AllowedEmails: []string{"a@company.com"},
	}
	defer func() { testHandler.cfg = origCfg }()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	testHandler.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetConfig: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	if cfg.CdnDomain != "cdn.example.com" {
		t.Fatalf("cdn_domain: want cdn.example.com, got %q", cfg.CdnDomain)
	}
	if cfg.AllowSignup {
		t.Fatalf("allow_signup: want false, got true")
	}
	if cfg.GoogleClientID != "google-client-id" {
		t.Fatalf("google_client_id: want google-client-id, got %q", cfg.GoogleClientID)
	}
	if !cfg.PrivateLoginMode {
		t.Fatalf("private_login_mode: want true, got false")
	}
	if cfg.PosthogKey != "phc_test" {
		t.Fatalf("posthog_key: want phc_test, got %q", cfg.PosthogKey)
	}
	if cfg.PosthogHost != "https://eu.i.posthog.com" {
		t.Fatalf("posthog_host: want https://eu.i.posthog.com, got %q", cfg.PosthogHost)
	}
}
