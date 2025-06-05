-- Change missing column to state: one of 'present', 'unknown', 'removed'
ALTER TABLE issue ADD COLUMN state VARCHAR(32) NOT NULL DEFAULT 'present';
UPDATE issue
  SET state = 'unknown' WHERE missing = TRUE;
ALTER TABLE issue ALTER COLUMN state DROP DEFAULT;
ALTER TABLE issue DROP COLUMN missing;
