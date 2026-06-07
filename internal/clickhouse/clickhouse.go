// Package clickhouse wraps connecting to and querying the ClickHouse logs
// database. It is the single client used by query-api to read the app_logs and
// http_logs tables.
package clickhouse

import (
	"context"
	"fmt"
	"os"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func Open(ctx context.Context) (driver.Conn, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{env("CH_ADDR", "localhost:9000")},
		Auth: ch.Auth{
			Database: env("CH_DB", "logs"),
			Username: env("CH_USER", "logstore"),
			Password: env("CH_PASS", "logstore"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}
	return conn, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
