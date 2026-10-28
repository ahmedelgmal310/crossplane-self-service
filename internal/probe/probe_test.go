package probe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDB struct {
	info    Info
	infoErr error
	rows    int64
	err     error
}

func (f *fakeDB) Info(context.Context) (Info, error) { return f.info, f.infoErr }
func (f *fakeDB) Write(context.Context) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.rows++
	return f.rows, nil
}

func do(t *testing.T, h http.Handler, method, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body map[string]any
	if rec.Header().Get("Content-Type") == "application/json" {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: invalid JSON %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec, body
}

func TestEndpoints(t *testing.T) {
	healthy := &fakeDB{info: Info{User: "app", Database: "app", ServerVersion: "18.6"}}
	broken := &fakeDB{infoErr: errors.New("connection refused"), err: errors.New("connection refused")}

	tests := []struct {
		name   string
		db     DB
		method string
		path   string
		code   int
		check  func(t *testing.T, body map[string]any)
	}{
		{"healthz", nil, http.MethodGet, "/healthz", http.StatusOK, func(t *testing.T, b map[string]any) {
			if b["status"] != "ok" {
				t.Errorf("status = %v", b["status"])
			}
		}},
		{"root without db", nil, http.MethodGet, "/", http.StatusOK, func(t *testing.T, b map[string]any) {
			if b["database_configured"] != false || b["version"] != "v1" || b["pod"] != "pod-1" {
				t.Errorf("unexpected root body %v", b)
			}
		}},
		{"root with db", healthy, http.MethodGet, "/", http.StatusOK, func(t *testing.T, b map[string]any) {
			if b["database_configured"] != true {
				t.Errorf("database_configured = %v", b["database_configured"])
			}
		}},
		{"unknown path", nil, http.MethodGet, "/nope", http.StatusNotFound, nil},
		{"readyz without db", nil, http.MethodGet, "/readyz", http.StatusOK, func(t *testing.T, b map[string]any) {
			if b["database"] != "not configured" {
				t.Errorf("database = %v", b["database"])
			}
		}},
		{"readyz healthy db", healthy, http.MethodGet, "/readyz", http.StatusOK, func(t *testing.T, b map[string]any) {
			if b["database"] != "connected" {
				t.Errorf("database = %v", b["database"])
			}
		}},
		{"readyz broken db", broken, http.MethodGet, "/readyz", http.StatusServiceUnavailable, func(t *testing.T, b map[string]any) {
			if b["ready"] != false || b["error"] != "connection refused" {
				t.Errorf("unexpected body %v", b)
			}
		}},
		{"db without db", nil, http.MethodGet, "/db", http.StatusOK, func(t *testing.T, b map[string]any) {
			if b["configured"] != false || b["connected"] != false {
				t.Errorf("unexpected body %v", b)
			}
		}},
		{"db healthy", healthy, http.MethodGet, "/db", http.StatusOK, func(t *testing.T, b map[string]any) {
			info, ok := b["info"].(map[string]any)
			if b["connected"] != true || !ok || info["user"] != "app" || info["server_version"] != "18.6" {
				t.Errorf("unexpected body %v", b)
			}
		}},
		{"db broken", broken, http.MethodGet, "/db", http.StatusServiceUnavailable, func(t *testing.T, b map[string]any) {
			if b["connected"] != false || b["error"] != "connection refused" {
				t.Errorf("unexpected body %v", b)
			}
		}},
		{"write without db", nil, http.MethodPost, "/db/write", http.StatusConflict, nil},
		{"write healthy", healthy, http.MethodPost, "/db/write", http.StatusOK, func(t *testing.T, b map[string]any) {
			if b["connected"] != true || b["rows"] != float64(1) {
				t.Errorf("unexpected body %v", b)
			}
		}},
		{"write broken", broken, http.MethodPost, "/db/write", http.StatusServiceUnavailable, nil},
		{"write needs POST", healthy, http.MethodGet, "/db/write", http.StatusMethodNotAllowed, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := New(tc.db, "v1", "pod-1").Handler()
			rec, body := do(t, h, tc.method, tc.path)
			if rec.Code != tc.code {
				t.Fatalf("code = %d, want %d (body %s)", rec.Code, tc.code, rec.Body.String())
			}
			if tc.check != nil {
				tc.check(t, body)
			}
		})
	}
}

func TestNilInterfaceIsNotConfigured(t *testing.T) {
	// A typed nil must not be passed as DB; New callers pass a plain nil.
	var db DB
	h := New(db, "v1", "p").Handler()
	rec, body := do(t, h, http.MethodGet, "/readyz")
	if rec.Code != http.StatusOK || body["database"] != "not configured" {
		t.Fatalf("got %d %v", rec.Code, body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}
