package probe

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewPostgresRejectsBadURL(t *testing.T) {
	if _, err := NewPostgres(t.Context(), "postgres://%zz"); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestPostgresUnreachable(t *testing.T) {
	// Port 1 on loopback refuses connections immediately.
	p, err := NewPostgres(t.Context(), "postgres://app:secret@127.0.0.1:1/app?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if _, err := p.Info(ctx); err == nil {
		t.Error("Info: expected an error")
	}
	if _, err := p.Write(ctx); err == nil {
		t.Error("Write: expected an error")
	}
}

// TestPostgresIntegration runs against a real PostgreSQL when
// TEST_DATABASE_URL is set (CI starts one as a service container).
func TestPostgresIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	p, err := NewPostgres(t.Context(), url)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	info, err := p.Info(ctx)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.User == "" || info.Database == "" || info.ServerVersion == "" {
		t.Errorf("incomplete info %+v", info)
	}
	first, err := p.Write(ctx)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	second, err := p.Write(ctx)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if second != first+1 {
		t.Errorf("rows did not grow by one: %d then %d", first, second)
	}
}
