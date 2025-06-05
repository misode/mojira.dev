-- Adds a priority and retry-delay system to the sync queue
ALTER TABLE sync_queue
  ADD COLUMN priority INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN reason VARCHAR(32),
  ADD COLUMN failed_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN retry_after TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE sync_queue SET reason = 'update-feed' WHERE reason IS NULL;

ALTER TABLE sync_queue ALTER COLUMN reason SET NOT NULL;

-- Removes the old sync_state table, now covered by the sync_queue
DROP TABLE sync_state;

-- Delete 'unknown' issues
DELETE FROM issue WHERE state = 'unknown';
