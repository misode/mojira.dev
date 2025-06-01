BEGIN;

CREATE INDEX IF NOT EXISTS idx_issue_summary ON issue USING GIN (to_tsvector('english', summary));

CREATE INDEX IF NOT EXISTS idx_issue_text ON issue USING GIN (to_tsvector('english', text));

CREATE INDEX IF NOT EXISTS idx_issue_created_date ON issue(created_date DESC);

CREATE INDEX IF NOT EXISTS idx_issue_updated_date ON issue(updated_date DESC);

CREATE INDEX IF NOT EXISTS idx_issue_resolved_date ON issue(resolved_date DESC);

CREATE INDEX IF NOT EXISTS idx_issue_synced_date ON issue(synced_date DESC);

-- Convert `labels` column from TEXT to TEXT[]
ALTER TABLE issue RENAME COLUMN labels TO labels_text;
ALTER TABLE issue ADD COLUMN labels TEXT[];
UPDATE issue
  SET labels = string_to_array(trim(both from labels_text), ',')
  WHERE labels_text IS NOT NULL AND trim(both from labels_text) <> '';
ALTER TABLE issue DROP COLUMN labels_text;
CREATE INDEX idx_issue_labels ON issue USING GIN (labels);

-- Convert `category` column from TEXT to TEXT[]
ALTER TABLE issue RENAME COLUMN category TO category_text;
ALTER TABLE issue ADD COLUMN category TEXT[];
UPDATE issue
  SET category = string_to_array(trim(both from category_text), ',')
  WHERE category_text IS NOT NULL AND trim(both from category_text) <> '';
ALTER TABLE issue DROP COLUMN category_text;
CREATE INDEX idx_issue_category ON issue USING GIN (category);

-- Convert `components` column from TEXT to TEXT[]
ALTER TABLE issue RENAME COLUMN components TO components_text;
ALTER TABLE issue ADD COLUMN components TEXT[];
UPDATE issue
  SET components = string_to_array(trim(both from components_text), ',')
  WHERE components_text IS NOT NULL AND trim(both from components_text) <> '';
ALTER TABLE issue DROP COLUMN components_text;
CREATE INDEX idx_issue_components ON issue USING GIN (components);

-- Convert `affected_versions` column from TEXT to TEXT[]
ALTER TABLE issue RENAME COLUMN affected_versions TO affected_versions_text;
ALTER TABLE issue ADD COLUMN affected_versions TEXT[];
UPDATE issue
  SET affected_versions = string_to_array(trim(both from affected_versions_text), ',')
  WHERE affected_versions_text IS NOT NULL AND trim(both from affected_versions_text) <> '';
ALTER TABLE issue DROP COLUMN affected_versions_text;
CREATE INDEX idx_issue_affected_versions ON issue USING GIN (affected_versions);

-- Convert `fix_versions` column from TEXT to TEXT[]
ALTER TABLE issue RENAME COLUMN fix_versions TO fix_versions_text;
ALTER TABLE issue ADD COLUMN fix_versions TEXT[];
UPDATE issue
  SET fix_versions = string_to_array(trim(both from fix_versions_text), ',')
  WHERE fix_versions_text IS NOT NULL AND trim(both from fix_versions_text) <> '';
ALTER TABLE issue DROP COLUMN fix_versions_text;
CREATE INDEX idx_issue_fix_versions ON issue USING GIN (fix_versions);

COMMIT;
