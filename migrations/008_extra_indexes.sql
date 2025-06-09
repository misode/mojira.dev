-- Optimize ordering by votes
ALTER TABLE issue ADD COLUMN total_votes int GENERATED ALWAYS AS (legacy_votes + votes) STORED;
CREATE INDEX idx_issue_total_votes ON issue (total_votes DESC);
CREATE INDEX idx_issue_project_total_votes ON issue (project, total_votes DESC);

-- Optimize resolution filtering
CREATE INDEX idx_issue_resolution ON issue (resolution);
