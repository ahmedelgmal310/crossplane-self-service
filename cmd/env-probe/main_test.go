package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestRunServesAndShutsDown(t *testing.T) {
	for _, tc := range []struct {
		name     string
		vars     map[string]string
		wantDBOn bool
	}{
		{"without database", nil, false},
		// The pool connects lazily, so an unreachable database still starts.
		{"with database", map[string]string{"DATABASE_URL": "postgres://app:x@127.0.0.1:1/app?sslmode=disable"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			go func() { done <- run(ctx, ln, env(tc.vars)) }()

			url := "http://" + ln.Addr().String() + "/"
			var body map[string]any
			deadline := time.Now().Add(5 * time.Second)
			for {
				req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					err = json.NewDecoder(resp.Body).Decode(&body)
					_ = resp.Body.Close()
					if err != nil {
						t.Fatal(err)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("server never answered: %v", err)
				}
				time.Sleep(20 * time.Millisecond)
			}
			if body["database_configured"] != tc.wantDBOn {
				t.Errorf("database_configured = %v, want %v", body["database_configured"], tc.wantDBOn)
			}

			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("run returned %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("run did not return after cancel")
			}
		})
	}
}

func TestRunRejectsBadDatabaseURL(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := run(t.Context(), ln, env(map[string]string{"DATABASE_URL": "postgres://%zz"})); err == nil {
		t.Fatal("expected an error for a malformed DATABASE_URL")
	}
}

func TestRunReportsServeError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close() // Serve on a closed listener fails immediately.
	if err := run(t.Context(), ln, env(nil)); err == nil {
		t.Fatal("expected a serve error")
	}
}
