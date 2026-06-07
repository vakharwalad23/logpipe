CREATE TABLE IF NOT EXISTS logs.http_logs
(
    timestamp DateTime64(3),
    host LowCardinality(String),
    method LowCardinality(String),
    path String,
    status UInt16,
    duration_ms UInt32,
    bytes UInt32,
    client_ip String,
    user_agent String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
ORDER BY (status, timestamp)
TTL toDateTime(timestamp) + INTERVAL 30 DAY;