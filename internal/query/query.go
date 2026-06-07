// Package query translates the small query syntax exposed by query-api into
// ClickHouse SQL. User-supplied values are always passed as query parameters,
// never concatenated into the SQL string.
package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Filter struct {
	Col string
	Val string
}

var tables = map[string]string{
	"app":  "logs.app_logs",
	"http": "logs.http_logs",
}

var columns = map[string]map[string]bool{
	"app": {
		"level":   true,
		"service": true,
		"host":    true,
		"message": true,
	},
	"http": {
		"method":    true,
		"path":      true,
		"status":    true,
		"client_ip": true,
		"host":      true,
	},
}

var projections = map[string]string{
	"app":  "timestamp, level, service, host, message",
	"http": "timestamp, method, path, status, duration_ms, client_ip",
}

func ParseExpr(expr string) ([]Filter, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	var filters []Filter
	for tok := range strings.FieldsSeq(expr) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("bad filter %q, want key=value", tok)
		}
		filters = append(filters, Filter{Col: k, Val: v})
	}
	return filters, nil
}

func table(stream string) (string, error) {
	t, ok := tables[stream]
	if !ok {
		return "", fmt.Errorf("unknown table %q, want app or http", stream)
	}
	return t, nil
}

func where(stream string, filters []Filter) (string, []any, error) {
	allowed := columns[stream]
	var conds []string
	var args []any
	for _, f := range filters {
		if !allowed[f.Col] {
			return "", nil, fmt.Errorf("unknown filter key %q for %s logs", f.Col, stream)
		}
		conds = append(conds, f.Col+" = ?")
		args = append(args, value(f.Col, f.Val))
	}
	return strings.Join(conds, " AND "), args, nil
}

func value(col, raw string) any {
	if col == "status" {
		if n, err := strconv.Atoi(raw); err == nil {
			return uint16(n)
		}
	}
	return raw
}

type Search struct {
	Stream  string
	Filters []Filter
	Since   time.Duration
	Limit   int
}

func (s Search) SQL() (string, []any, error) {
	tbl, err := table(s.Stream)
	if err != nil {
		return "", nil, err
	}
	cond, args, err := where(s.Stream, s.Filters)
	if err != nil {
		return "", nil, err
	}
	var clauses []string
	if cond != "" {
		clauses = append(clauses, cond)
	}
	if s.Since > 0 {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, time.Now().Add(-s.Since).UTC())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s FROM %s", projections[s.Stream], tbl)
	if len(clauses) > 0 {
		b.WriteString(" WHERE " + strings.Join(clauses, " AND "))
	}
	b.WriteString(" ORDER BY timestamp DESC LIMIT ?")
	args = append(args, uint64(s.Limit))
	return b.String(), args, nil
}

type Tail struct {
	Stream  string
	Filters []Filter
	Limit   int
}

func (t Tail) SQL() (string, []any, error) {
	return Search{Stream: t.Stream, Filters: t.Filters, Limit: t.Limit}.SQL()
}

type Count struct {
	Stream  string
	Filters []Filter
	Since   time.Duration
	Bucket  time.Duration
}

func (c Count) SQL() (string, []any, error) {
	tbl, err := table(c.Stream)
	if err != nil {
		return "", nil, err
	}
	cond, condArgs, err := where(c.Stream, c.Filters)
	if err != nil {
		return "", nil, err
	}
	secs := int64(c.Bucket / time.Second)
	if secs <= 0 {
		secs = 60
	}
	args := []any{uint32(secs)}
	args = append(args, condArgs...)
	var clauses []string
	if cond != "" {
		clauses = append(clauses, cond)
	}
	if c.Since > 0 {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, time.Now().Add(-c.Since).UTC())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "SELECT toStartOfInterval(timestamp, toIntervalSecond(?)) AS bucket, count() AS n FROM %s", tbl)
	if len(clauses) > 0 {
		b.WriteString(" WHERE " + strings.Join(clauses, " AND "))
	}
	b.WriteString(" GROUP BY bucket ORDER BY bucket")
	return b.String(), args, nil
}
