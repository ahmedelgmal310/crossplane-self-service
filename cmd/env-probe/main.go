// Command env-probe is the stand-in workload deployed into every
// AppEnvironment. It reads DATABASE_URL from the platform-delivered Secret and
// reports whether it can use the database.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ahmedelgmal310/crossplane-self-service/internal/probe"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := serve(); err != nil {
		slog.Error("env-probe stopped", "err", err)
		os.Exit(1)
	}
}

func serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return run(ctx, ln, os.Getenv)
}

// run serves until ctx is cancelled, then shuts down gracefully.
func run(ctx context.Context, ln net.Listener, getenv func(string) string) error {
	var db probe.DB
	if url := getenv("DATABASE_URL"); url != "" {
		pg, err := probe.NewPostgres(ctx, url)
		if err != nil {
			return err
		}
		defer pg.Close()
		db = pg
	}
	hostname, _ := os.Hostname()

	srv := &http.Server{
		Handler:           probe.New(db, version, hostname).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	slog.Info("env-probe listening", "addr", ln.Addr().String(), "version", version, "database", db != nil)

	select {
	case err := <-errc:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
