package main

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// newBasicAuthHandler returns an http.HandlerFunc that authenticates requests
// using HTTP Basic Auth against the provided user database, then proxies
// authenticated requests to the remote host via the given reverse proxy.
func newBasicAuthHandler(proxy *httputil.ReverseProxy, remote *url.URL, users map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok && VerifyUserPass(users, user, pass) {
			r.Host = remote.Host
			proxy.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", "Basic")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}

// newHealthCheckHandler returns an http.HandlerFunc that handles readiness and
// liveness probe requests by forwarding them to the backend.
func newHealthCheckHandler(remote *url.URL, backend *Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var target string
		switch r.URL.Path {
		case "/-/ready":
			if backend.Readiness == "" {
				return
			}
			target = remote.JoinPath(backend.Readiness).String()
		case "/-/healthy":
			if backend.Liveness == "" {
				return
			}
			target = remote.JoinPath(backend.Liveness).String()
		default:
			return
		}

		timeout := 3 * time.Second
		if t := backend.HealthCheckTimeoutInSecond; t != 0 {
			timeout = time.Duration(t) * time.Second
		}

		newctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(newctx, http.MethodGet, target, nil)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close() // nolint:errcheck
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	}
}

// newServeMux creates an http.ServeMux with all routes configured based on the
// provided config and remote URL.
func newServeMux(config *Config, remote *url.URL) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /-/", newHealthCheckHandler(remote, config.Backend))
	mux.HandleFunc("/", newBasicAuthHandler(httputil.NewSingleHostReverseProxy(remote), remote, config.Users))
	return mux
}
