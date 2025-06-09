-- Optimize text indexes for present issues only
DROP INDEX idx_issue_summary;
DROP INDEX idx_issue_text;
ALTER INDEX idx_issue_summary_present RENAME TO idx_issue_summary;
ALTER INDEX idx_issue_text_present RENAME TO idx_issue_text;

-- Add extra columns for sorting issues
ALTER TABLE issue ADD COLUMN mojang_priority_rank int GENERATED ALWAYS AS (
  CASE mojang_priority
    WHEN 'Very Important' THEN 4
    WHEN 'Important' THEN 3
    WHEN 'Normal' THEN 2
    WHEN 'Low' THEN 1
    ELSE 0
  END
) STORED;
ALTER TABLE issue ADD COLUMN comment_count int DEFAULT 0;
ALTER TABLE issue ADD COLUMN duplicate_count int DEFAULT 0;

-- Populate comments and duplicates count
UPDATE issue i
SET comment_count = COALESCE(c.count, 0)
FROM (
  SELECT issue_key, COUNT(*) AS count
  FROM comment
  GROUP BY issue_key
) c
WHERE i.key = c.issue_key;

UPDATE issue i
SET duplicate_count = COALESCE(l.count, 0)
FROM (
  SELECT issue_key, COUNT(*) AS count
  FROM issue_link
  WHERE type = 'is duplicated by'
  GROUP BY issue_key
) l
WHERE i.key = l.issue_key;

-- Add matching indexes
CREATE INDEX idx_issue_mojang_priority_rank ON issue(mojang_priority_rank DESC);
CREATE INDEX idx_issue_comment_count ON issue(comment_count DESC);
CREATE INDEX idx_issue_duplicate_count ON issue(duplicate_count DESC);
