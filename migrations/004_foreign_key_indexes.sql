-- Create indexes for foreign issue_key columns
CREATE INDEX idx_comment_issue_key ON comment(issue_key);
CREATE INDEX idx_attachment_issue_key ON attachment(issue_key);
CREATE INDEX idx_issue_link_issue_key ON issue_link(issue_key);
