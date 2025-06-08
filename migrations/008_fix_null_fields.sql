-- Set null comments to empty strings
UPDATE issue SET creator_name = '' WHERE creator_name IS NULL;
UPDATE issue SET creator_avatar = '' WHERE creator_avatar IS NULL;
UPDATE comment SET legacy_id = '' WHERE legacy_id IS NULL;
