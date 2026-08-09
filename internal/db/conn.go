package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/claymor333/squal/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

type Conn struct {
	Name string
	db   *sql.DB
}

func Open(ctx context.Context, p config.Profile) (*Conn, error) {
	dsn, err := buildDSN(p)
	if err != nil {
		return nil, err
	}
	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(8)
	pool.SetMaxIdleConns(4)
	pool.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.Timeout)*time.Second)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Conn{Name: p.Name, db: pool}, nil
}

func (c *Conn) DB() *sql.DB { return c.db }

func (c *Conn) Close() error { return c.db.Close() }

func (c *Conn) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, q, args...)
}

func (c *Conn) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return c.db.ExecContext(ctx, q, args...)
}

func buildDSN(p config.Profile) (string, error) {
	if p.Socket != "" {
		return fmt.Sprintf("%s@unix(%s)/%s?parseTime=true", p.User, p.Socket, p.Database), nil
	}
	if p.Host == "" {
		return "", fmt.Errorf("profile %q: host is required (or set socket)", p.Name)
	}
	port := p.Port
	if port == 0 {
		port = 3306
	}
	tls := "false"
	if p.SSL {
		tls = "true"
	}
	charset := p.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=%s&tls=%s&timeout=5s&readTimeout=0",
		p.User, p.Password, p.Host, port, p.Database, charset, tls,
	), nil
}
