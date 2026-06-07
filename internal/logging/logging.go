// Package logging produces the service's two log streams — application logs and
// HTTP access logs — as JSON, one object per line, using the standard library's
// log/slog. Each line carries a type field so the Vector pipeline can route it,
// and entries are written to a file on the shared volume.
package logging
