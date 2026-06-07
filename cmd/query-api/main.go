// Command query-api is a small command-line client that queries the ClickHouse
// log tables in three modes: search, count, and tail.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/vakharwalad23/logpipe/internal/clickhouse"
	"github.com/vakharwalad23/logpipe/internal/query"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	argv := os.Args[2:]

	var (
		sql    string
		params []any
		stream string
		err    error
	)
	switch cmd {
	case "search":
		sql, params, stream, err = searchCmd(argv)
	case "count":
		sql, params, stream, err = countCmd(argv)
	case "tail":
		sql, params, stream, err = tailCmd(argv)
	default:
		usage()
	}
	if err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := clickhouse.Open(ctx)
	if err != nil {
		fatal(err)
	}
	defer conn.Close()

	rows, err := conn.Query(ctx, sql, params...)
	if err != nil {
		fatal(err)
	}
	defer rows.Close()

	if cmd == "count" {
		printCount(rows)
	} else {
		printRows(stream, rows)
	}
	if err := rows.Err(); err != nil {
		fatal(err)
	}
}

func searchCmd(argv []string) (string, []any, string, error) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	table := fs.String("table", "app", "app | http")
	filter := fs.String("filter", "", "key=value filters, space-separated")
	level := fs.String("level", "", "shortcut for level=")
	service := fs.String("service", "", "shortcut for service=")
	status := fs.String("status", "", "shortcut for status=")
	path := fs.String("path", "", "shortcut for path=")
	since := fs.Duration("since", time.Hour, "look-back window, e.g. 10m or 1h")
	limit := fs.Int("limit", 50, "max rows")
	fs.Parse(argv)

	filters, err := collectFilters(*filter, *level, *service, *status, *path)
	if err != nil {
		return "", nil, "", err
	}
	sql, params, err := query.Search{Stream: *table, Filters: filters, Since: *since, Limit: *limit}.SQL()
	return sql, params, *table, err
}

func countCmd(argv []string) (string, []any, string, error) {
	fs := flag.NewFlagSet("count", flag.ExitOnError)
	table := fs.String("table", "app", "app | http")
	filter := fs.String("filter", "", "key=value filters, space-separated")
	level := fs.String("level", "", "shortcut for level=")
	service := fs.String("service", "", "shortcut for service=")
	status := fs.String("status", "", "shortcut for status=")
	path := fs.String("path", "", "shortcut for path=")
	since := fs.Duration("since", time.Hour, "look-back window, e.g. 10m or 1h")
	bucket := fs.Duration("bucket", time.Minute, "bucket size, e.g. 10s or 1m")
	fs.Parse(argv)

	filters, err := collectFilters(*filter, *level, *service, *status, *path)
	if err != nil {
		return "", nil, "", err
	}
	sql, params, err := query.Count{Stream: *table, Filters: filters, Since: *since, Bucket: *bucket}.SQL()
	return sql, params, *table, err
}

func tailCmd(argv []string) (string, []any, string, error) {
	fs := flag.NewFlagSet("tail", flag.ExitOnError)
	table := fs.String("table", "app", "app | http")
	filter := fs.String("filter", "", "key=value filters, space-separated")
	limit := fs.Int("limit", 20, "rows")
	fs.Parse(argv)

	filters, err := query.ParseExpr(*filter)
	if err != nil {
		return "", nil, "", err
	}
	sql, params, err := query.Tail{Stream: *table, Filters: filters, Limit: *limit}.SQL()
	return sql, params, *table, err
}

func collectFilters(expr, level, service, status, path string) ([]query.Filter, error) {
	filters, err := query.ParseExpr(expr)
	if err != nil {
		return nil, err
	}
	for _, sc := range []query.Filter{
		{Col: "level", Val: level},
		{Col: "service", Val: service},
		{Col: "status", Val: status},
		{Col: "path", Val: path},
	} {
		if sc.Val != "" {
			filters = append(filters, sc)
		}
	}
	return filters, nil
}

func printRows(stream string, rows driver.Rows) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	defer w.Flush()
	if stream == "http" {
		fmt.Fprintln(w, "timestamp\tmethod\tpath\tstatus\tduration_ms\tclient_ip")
		for rows.Next() {
			var (
				ts               time.Time
				method, path, ip string
				status           uint16
				dur              uint32
			)
			if err := rows.Scan(&ts, &method, &path, &status, &dur, &ip); err != nil {
				fatal(err)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n", ts.Format(time.RFC3339), method, path, status, dur, ip)
		}
		return
	}
	fmt.Fprintln(w, "timestamp\tlevel\tservice\thost\tmessage")
	for rows.Next() {
		var (
			ts                            time.Time
			level, service, host, message string
		)
		if err := rows.Scan(&ts, &level, &service, &host, &message); err != nil {
			fatal(err)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ts.Format(time.RFC3339), level, service, host, message)
	}
}

func printCount(rows driver.Rows) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "bucket\tcount")
	for rows.Next() {
		var (
			bucket time.Time
			n      uint64
		)
		if err := rows.Scan(&bucket, &n); err != nil {
			fatal(err)
		}
		fmt.Fprintf(w, "%s\t%d\n", bucket.Format(time.RFC3339), n)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: query-api <search|count|tail> [flags]")
	os.Exit(2)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "query-api:", err)
	os.Exit(1)
}
