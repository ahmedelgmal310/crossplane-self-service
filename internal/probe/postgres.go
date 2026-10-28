package probe

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres implements DB with a small pgx connection pool. The pool connects
// lazily, so the process starts even while the database is still coming up;
// readiness reports the database state instead.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres parses a libpq URL (DATABASE_URL) and prepares a pool.
func NewPostgres(ctx context.Context, url string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 4
	cfg.MaxConnIdleTime = time.Minute
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Info implements DB.
func (p *Postgres) Info(ctx context.Context) (Info, error) {
	var info Info
	err := p.pool.QueryRow(ctx,
		"SELECT current_user, current_database(), current_setting('server_version')").
		Scan(&info.User, &info.Database, &info.ServerVersion)
	if err != nil {
		return Info{}, fmt.Errorf("query: %w", err)
	}
	return info, nil
}

// Write implements DB.
func (p *Postgres) Write(ctx context.Context) (int64, error) {
	if _, err := p.pool.Exec(ctx,
		"CREATE TABLE IF NOT EXISTS platform_probe (id bigserial PRIMARY KEY, at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return 0, fmt.Errorf("create table: %w", err)
	}
	if _, err := p.pool.Exec(ctx, "INSERT INTO platform_probe DEFAULT VALUES"); err != nil {
		return 0, fmt.Errorf("insert: %w", err)
	}
	var rows int64
	if err := p.pool.QueryRow(ctx, "SELECT count(*) FROM platform_probe").Scan(&rows); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return rows, nil
}

// Close releases the pool.
func (p *Postgres) Close() {
	p.pool.Close()
}
