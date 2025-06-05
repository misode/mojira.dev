-- Optimize homepage
CREATE INDEX IF NOT EXISTS idx_issue_present_created_desc
  ON issue (created_date DESC)
  WHERE state = 'present';

-- Optimize search box
CREATE INDEX IF NOT EXISTS idx_issue_text_present ON issue USING GIN (to_tsvector('english', text)) WHERE state = 'present';
CREATE INDEX IF NOT EXISTS idx_issue_summary_present ON issue USING GIN (to_tsvector('english', summary)) WHERE state = 'present';
