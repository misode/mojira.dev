-- Create view of issue counts grouped by most common filters
CREATE MATERIALIZED VIEW issue_count AS
SELECT project, status, confirmation_status, resolution, COUNT(*) AS count
FROM issue
WHERE state = 'present'
GROUP BY project, status, confirmation_status, resolution;
