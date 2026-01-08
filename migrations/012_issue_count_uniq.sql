--- Create an index so we can concurrently refresh this materialized view
CREATE UNIQUE INDEX issue_count_uniq ON issue_count (project, status, confirmation_status, resolution);
