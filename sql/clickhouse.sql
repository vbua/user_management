CREATE TABLE IF NOT EXISTS user_adds_queue (
    timestamp UInt64,
    user_id UInt64,
    success Bool
)
ENGINE = Kafka('kafka:9092', 'user_adds', 'group1', 'JSONEachRow');

CREATE TABLE IF NOT EXISTS user_adds (
    timestamp UInt64,
    user_id UInt64,
    success Bool
) Engine = MergeTree()
ORDER BY (timestamp, user_id, success);

CREATE MATERIALIZED VIEW IF NOT EXISTS user_adds_queue_mv TO user_adds AS
SELECT timestamp, user_id, success
FROM user_adds_queue;