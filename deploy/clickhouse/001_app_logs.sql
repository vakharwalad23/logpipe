CREATE DATABASE IF NOT EXISTS logs;
CREATE TABLE IF NOT EXISTS logs.app_logs
(
    timestamp DateTime64(3),
    level LowCardinality(String),
    service LowCardinality(String),
    host LowCardinality(String),
    message String,
    fields Map(String, String)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service, level, timestamp)
TTL toDateTime(timestamp) + INTERVAL 30 DAY;
