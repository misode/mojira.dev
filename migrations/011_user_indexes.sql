CREATE INDEX IF NOT EXISTS idx_issue_reporter_created ON issue(reporter_name, created_date DESC) WHERE state = 'present';
CREATE INDEX IF NOT EXISTS idx_issue_assignee_created ON issue(assignee_name, created_date DESC) WHERE state = 'present';
CREATE INDEX IF NOT EXISTS idx_comment_author_date ON comment(author_name, date DESC);
