ALTER TABLE issue
  ADD COLUMN creator_name TEXT,
  ADD COLUMN creator_avatar TEXT,
  ADD COLUMN legacy_votes INTEGER NOT NULL DEFAULT 0;

ALTER TABLE comment
  ADD COLUMN legacy_id TEXT;
