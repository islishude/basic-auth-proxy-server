package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func hashedPassword(t *testing.T, plain string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(h)
}

// ---------------------------------------------------------------------------
// newBasicAuthHandler tests
// ---------------------------------------------------------------------------

func TestBasicAuthHandler_ValidCredentials(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend reached")) // nolint:errcheck
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(remote)
	users := map[string]string{"alice": hashedPassword(t, "secret")}

	handler := newBasicAuthHandler(proxy, remote, users)
	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	req.SetBasicAuth("alice", "secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "backend reached" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestBasicAuthHandler_InvalidPassword(t *testing.T) {
	t.Parallel()

	remote, _ := url.Parse("http://localhost:9999")
	proxy := httputil.NewSingleHostReverseProxy(remote)
	users := map[string]string{"alice": hashedPassword(t, "secret")}

	handler := newBasicAuthHandler(proxy, remote, users)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("alice", "wrong")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if v := rr.Header().Get("WWW-Authenticate"); v != "Basic" {
		t.Fatalf("expected WWW-Authenticate: Basic, got %q", v)
	}
}

func TestBasicAuthHandler_MissingCredentials(t *testing.T) {
	t.Parallel()

	remote, _ := url.Parse("http://localhost:9999")
	proxy := httputil.NewSingleHostReverseProxy(remote)

	handler := newBasicAuthHandler(proxy, remote, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestBasicAuthHandler_UnknownUser(t *testing.T) {
	t.Parallel()

	remote, _ := url.Parse("http://localhost:9999")
	proxy := httputil.NewSingleHostReverseProxy(remote)
	users := map[string]string{"alice": hashedPassword(t, "secret")}

	handler := newBasicAuthHandler(proxy, remote, users)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("bob", "secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestBasicAuthHandler_SetsHostHeader(t *testing.T) {
	t.Parallel()

	var receivedHost string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(remote)
	users := map[string]string{"alice": hashedPassword(t, "secret")}

	handler := newBasicAuthHandler(proxy, remote, users)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("alice", "secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedHost != remote.Host {
		t.Fatalf("expected host %q, got %q", remote.Host, receivedHost)
	}
}

// ---------------------------------------------------------------------------
// newHealthCheckHandler tests
// ---------------------------------------------------------------------------

func TestHealthCheckHandler_ReadyOK(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	b := &Backend{Target: backend.URL, Readiness: "/backend-ready"}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHealthCheckHandler_HealthyOK(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	b := &Backend{Target: backend.URL, Liveness: "/backend-health"}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/healthy", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHealthCheckHandler_ReadyNotConfigured(t *testing.T) {
	t.Parallel()

	remote, _ := url.Parse("http://localhost:9999")
	b := &Backend{Target: "http://localhost:9999", Readiness: ""}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// When readiness is not configured, handler returns 200 (implicit)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when readiness not configured, got %d", rr.Code)
	}
}

func TestHealthCheckHandler_HealthyNotConfigured(t *testing.T) {
	t.Parallel()

	remote, _ := url.Parse("http://localhost:9999")
	b := &Backend{Target: "http://localhost:9999", Liveness: ""}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/healthy", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when liveness not configured, got %d", rr.Code)
	}
}

func TestHealthCheckHandler_UnknownPath(t *testing.T) {
	t.Parallel()

	remote, _ := url.Parse("http://localhost:9999")
	b := &Backend{Target: "http://localhost:9999", Readiness: "/ready", Liveness: "/health"}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/unknown", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Unknown path returns 200 implicitly (no error written)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown path, got %d", rr.Code)
	}
}

func TestHealthCheckHandler_BackendUnavailable(t *testing.T) {
	t.Parallel()

	// Point to a port that nothing is listening on
	remote, _ := url.Parse("http://127.0.0.1:1")
	b := &Backend{
		Target:                     "http://127.0.0.1:1",
		Readiness:                  "/ready",
		HealthCheckTimeoutInSecond: 1,
	}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHealthCheckHandler_BackendReturnsNon200(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	b := &Backend{Target: backend.URL, Readiness: "/ready"}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHealthCheckHandler_CustomTimeout(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	b := &Backend{
		Target:                     backend.URL,
		Readiness:                  "/ready",
		HealthCheckTimeoutInSecond: 5,
	}
	handler := newHealthCheckHandler(remote, b)

	req := httptest.NewRequest(http.MethodGet, "/-/ready", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// newServeMux tests
// ---------------------------------------------------------------------------

func TestNewServeMux_ProxyRoute(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path}) // nolint:errcheck
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	config := &Config{
		Port:    8080,
		Backend: &Backend{Target: backend.URL},
		Users:   map[string]string{"alice": hashedPassword(t, "pass")},
	}

	mux := newServeMux(config, remote)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Without auth → 401
	resp, err := http.Get(srv.URL + "/api/data")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close() // nolint:errcheck
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	// With auth → 200
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/data", nil)
	req.SetBasicAuth("alice", "pass")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close() // nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNewServeMux_HealthRouteNoAuth(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	remote, _ := url.Parse(backend.URL)
	config := &Config{
		Port: 8080,
		Backend: &Backend{
			Target:    backend.URL,
			Readiness: "/ready",
			Liveness:  "/health",
		},
		Users: map[string]string{"alice": hashedPassword(t, "pass")},
	}

	mux := newServeMux(config, remote)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Health check endpoints should not require auth
	for _, path := range []string{"/-/ready", "/-/healthy"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("request %s failed: %v", path, err)
		}
		resp.Body.Close() // nolint:errcheck
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, resp.StatusCode)
		}
	}
}
