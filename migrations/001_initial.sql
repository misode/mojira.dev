CREATE TABLE IF NOT EXISTS issue (
  key VARCHAR(32) PRIMARY KEY,
  project VARCHAR(16) GENERATED ALWAYS AS (substring(key from '^(.*?)-')) STORED,
  key_num INTEGER GENERATED ALWAYS AS (CAST(substring(key from '[0-9]+$') AS INTEGER)) STORED,
  summary TEXT,
  reporter_name TEXT,
  reporter_avatar TEXT,
  assignee_name TEXT,
  assignee_avatar TEXT,
  description TEXT,
  environment TEXT,
  labels TEXT,
  created_date TIMESTAMPTZ,
  updated_date TIMESTAMPTZ,
  resolved_date TIMESTAMPTZ,
  status TEXT,
  confirmation_status TEXT,
  resolution TEXT,
  affected_versions TEXT,
  fix_versions TEXT,
  category TEXT,
  mojang_priority TEXT,
  area TEXT,
  components TEXT,
  ado TEXT,
  platform TEXT,
  os_version TEXT,
  realms_platform TEXT,
  votes INTEGER NOT NULL DEFAULT 0,
  text TEXT,
  synced_date TIMESTAMPTZ NOT NULL,
  missing BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_issue_project ON issue(project);
CREATE INDEX IF NOT EXISTS idx_issue_key_num ON issue(key_num);
CREATE INDEX IF NOT EXISTS idx_issue_project_key_num ON issue(project, key_num);

CREATE TABLE IF NOT EXISTS comment (
  id SERIAL PRIMARY KEY,
  issue_key VARCHAR(32) NOT NULL REFERENCES issue(key) ON DELETE CASCADE,
  comment_id TEXT,
  date TIMESTAMPTZ,
  author_name TEXT,
  author_avatar TEXT,
  adf_comment TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS issue_link (
  id SERIAL PRIMARY KEY,
  issue_key VARCHAR(32) NOT NULL REFERENCES issue(key) ON DELETE CASCADE,
  type TEXT NOT NULL,
  other_key VARCHAR(32) NOT NULL,
  other_summary TEXT,
  other_status TEXT
);

CREATE TABLE IF NOT EXISTS attachment (
  id SERIAL PRIMARY KEY,
  issue_key VARCHAR(32) NOT NULL REFERENCES issue(key) ON DELETE CASCADE,
  attachment_id VARCHAR(32),
  filename TEXT,
  author_name TEXT,
  author_avatar TEXT,
  created_date TIMESTAMPTZ,
  size INTEGER,
  mime_type TEXT
);

CREATE TABLE IF NOT EXISTS sync_queue (
  issue_key VARCHAR(32) PRIMARY KEY,
  queued_date TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sync_state (
  prefix VARCHAR(16) PRIMARY KEY,
  last_processed INTEGER NOT NULL
);
