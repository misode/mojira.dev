-- Replace the username indexes with their lowercase variant
DROP INDEX idx_issue_reporter_created;
DROP INDEX idx_issue_assignee_created;
DROP INDEX idx_comment_author_date;

CREATE INDEX IF NOT EXISTS idx_issue_reporter_created ON issue(LOWER(reporter_name), created_date DESC) WHERE state = 'present';
CREATE INDEX IF NOT EXISTS idx_issue_assignee_created ON issue(LOWER(assignee_name), created_date DESC) WHERE state = 'present';
CREATE INDEX IF NOT EXISTS idx_comment_author_date ON comment(LOWER(author_name), date DESC);
