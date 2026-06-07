// Package query translates the small query syntax exposed by query-api into
// ClickHouse SQL. User-supplied values are always passed as query parameters,
// never concatenated into the SQL string.
package query
