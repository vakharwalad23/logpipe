// Package loadgen drives load against the system in two modes: concurrent HTTP
// traffic aimed at crud-api at a configurable rate, and synthetic log lines
// written straight to the tailed log file to reach high volume quickly.
package loadgen
