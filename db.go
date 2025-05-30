package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"mojira/model"
	"os"
	"time"
)

type DBClient struct {
	db *sql.DB
}

func NewDBClient() (*DBClient, error) {
	connStr := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	initTables(db)
	return &DBClient{db: db}, nil
}

func initTables(db *sql.DB) {
	fmt.Println("Initializing database tables...")
	// _, _ = db.Exec(`DROP TABLE IF EXISTS comment;`)
	// _, _ = db.Exec(`DROP TABLE IF EXISTS issue_link;`)
	// _, _ = db.Exec(`DROP TABLE IF EXISTS attachment;`)
	// _, _ = db.Exec(`DROP TABLE IF EXISTS issue;`)
	// _, _ = db.Exec(`DROP TABLE IF EXISTS sync_queue;`)
	// _, _ = db.Exec(`DROP TABLE IF EXISTS sync_state;`)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS issue (
			key VARCHAR(32) PRIMARY KEY,
			project VARCHAR(16) GENERATED ALWAYS AS (substring(key from '^(.*?)-')) STORED,
			key_num INTEGER GENERATED ALWAYS AS (CAST(substring(key from '[0-9]+$') AS INTEGER)) STORED,
			summary TEXT,
			reporter_name TEXT,
			reporter_avatar TEXT,
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
			synced_date TIMESTAMPTZ NOT NULL,
			missing BOOLEAN NOT NULL DEFAULT FALSE
		);
		CREATE INDEX IF NOT EXISTS idx_issue_project ON issue(project);
		CREATE INDEX IF NOT EXISTS idx_issue_key_num ON issue(key_num);
		CREATE INDEX IF NOT EXISTS idx_issue_project_key_num ON issue(project, key_num);
		CREATE TABLE IF NOT EXISTS comment (
			id SERIAL PRIMARY KEY,
			issue_key VARCHAR(32) NOT NULL REFERENCES issue(key) ON DELETE CASCADE,
			date TIMESTAMPTZ,
			author TEXT,
			avatar_url TEXT,
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
			id VARCHAR(32) PRIMARY KEY,
			issue_key VARCHAR(32) NOT NULL REFERENCES issue(key) ON DELETE CASCADE,
			filename TEXT,
			author TEXT,
			avatar_url TEXT,
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
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func (c *DBClient) GetAllIssues(limit int) ([]model.Issue, error) {
	rows, err := c.db.Query("SELECT key, summary, reporter_name, reporter_avatar, description, environment, labels, created_date, updated_date, resolved_date, status, confirmation_status, resolution, affected_versions, fix_versions, mojang_priority, area, category FROM issue WHERE missing = FALSE ORDER BY created_date DESC LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		if err := rows.Scan(&issue.Key, &issue.Summary, &issue.ReporterName, &issue.ReporterAvatar, &issue.Description, &issue.Environment, &issue.Labels, &issue.CreatedDate, &issue.UpdatedDate, &issue.ResolvedDate, &issue.Status, &issue.ConfirmationStatus, &issue.Resolution, &issue.AffectedVersions, &issue.FixVersions, &issue.MojangPriority, &issue.Area, &issue.Category); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *DBClient) GetIssueByKey(key string) (*model.Issue, error) {
	row := c.db.QueryRow("SELECT summary, reporter_name, reporter_avatar, description, environment, labels, created_date, updated_date, resolved_date, status, confirmation_status, resolution, affected_versions, fix_versions, mojang_priority, area, category FROM issue WHERE key = $1 AND missing = FALSE", key)
	var issue model.Issue
	issue.Key = key
	err := row.Scan(&issue.Summary, &issue.ReporterName, &issue.ReporterAvatar, &issue.Description, &issue.Environment, &issue.Labels, &issue.CreatedDate, &issue.UpdatedDate, &issue.ResolvedDate, &issue.Status, &issue.ConfirmationStatus, &issue.Resolution, &issue.AffectedVersions, &issue.FixVersions, &issue.MojangPriority, &issue.Area, &issue.Category)
	if err != nil {
		return nil, err
	}
	comments := []model.Comment{}
	rows, err := c.db.Query(`SELECT date, author, avatar_url, adf_comment FROM comment WHERE issue_key = $1 ORDER BY date ASC`, key)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cmt model.Comment
			if err := rows.Scan(&cmt.Date, &cmt.Author, &cmt.AvatarUrl, &cmt.AdfComment); err == nil {
				comments = append(comments, cmt)
			}
		}
	}
	issue.Comments = comments
	links := []model.IssueLink{}
	rows, err = c.db.Query(`SELECT type, other_key, other_summary, other_status FROM issue_link WHERE issue_key = $1`, key)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var l model.IssueLink
			if err := rows.Scan(&l.Type, &l.OtherKey, &l.OtherSummary, &l.OtherStatus); err == nil {
				links = append(links, l)
			}
		}
	}
	issue.Links = links
	attachments := []model.Attachment{}
	rows, err = c.db.Query(`SELECT id, filename, author, avatar_url, created_date, size, mime_type FROM attachment WHERE issue_key = $1`, key)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a model.Attachment
			if err := rows.Scan(&a.Id, &a.Filename, &a.AuthorName, &a.AuthorAvatar, &a.CreatedDate, &a.Size, &a.MimeType); err == nil {
				attachments = append(attachments, a)
			}
		}
	}
	issue.Attachments = attachments
	return &issue, nil
}

func (c *DBClient) InsertIssue(ctx context.Context, issue *model.Issue) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := c.insertIssueImpl(tx, issue); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (c *DBClient) insertIssueImpl(tx *sql.Tx, issue *model.Issue) error {
	_, err := tx.Exec(`DELETE FROM issue WHERE key = $1`, issue.Key)
	if err != nil {
		return err
	}
	query := `INSERT INTO issue (key, summary, reporter_name, reporter_avatar, description, environment, labels, created_date, updated_date, resolved_date, status, confirmation_status, resolution, affected_versions, fix_versions, category, mojang_priority, area, synced_date) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`
	_, err = tx.Exec(query, issue.Key, issue.Summary, issue.ReporterName, issue.ReporterAvatar, issue.Description, issue.Environment, issue.Labels, issue.CreatedDate, issue.UpdatedDate, issue.ResolvedDate, issue.Status, issue.ConfirmationStatus, issue.Resolution, issue.AffectedVersions, issue.FixVersions, issue.Category, issue.MojangPriority, issue.Area, time.Now())
	if err != nil {
		return errors.New("failed to insert issue: " + err.Error())
	}
	for _, cmt := range issue.Comments {
		_, err = tx.Exec(`INSERT INTO comment (issue_key, date, author, avatar_url, adf_comment) VALUES ($1, $2, $3, $4, $5)`, issue.Key, cmt.Date, cmt.Author, cmt.AvatarUrl, cmt.AdfComment)
		if err != nil {
			return errors.New("failed to insert comment: " + err.Error())
		}
	}
	for _, l := range issue.Links {
		_, err = tx.Exec(`INSERT INTO issue_link (issue_key, type, other_key, other_summary, other_status) VALUES ($1, $2, $3, $4, $5)`, issue.Key, l.Type, l.OtherKey, l.OtherSummary, l.OtherStatus)
		if err != nil {
			return errors.New("failed to insert issue_link: " + err.Error())
		}
	}
	for _, a := range issue.Attachments {
		_, err = tx.Exec(`INSERT INTO attachment (id, issue_key, filename, author, avatar_url, created_date, size, mime_type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, a.Id, issue.Key, a.Filename, a.AuthorName, a.AuthorAvatar, a.CreatedDate, a.Size, a.MimeType)
		if err != nil {
			return errors.New("failed to insert attachment: " + err.Error())
		}
	}
	return nil
}

func (c *DBClient) InsertMissingIssue(key string) error {
	query := `INSERT INTO issue (key, synced_date, missing) VALUES ($1, NOW(), TRUE) ON CONFLICT (key) DO NOTHING`
	_, err := c.db.Exec(query, key)
	if err != nil {
		return errors.New("failed to insert missing issue: " + err.Error())
	}
	return nil
}

func (c *DBClient) QueueIssueKeys(keys []string) (int, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	count := 0
	for _, key := range keys {
		query := `INSERT INTO sync_queue (issue_key)
			SELECT CAST($1 AS VARCHAR)
			WHERE NOT EXISTS (SELECT 1 FROM sync_queue WHERE issue_key = $1)
			AND NOT EXISTS (SELECT 1 FROM issue WHERE key = $1 AND synced_date >= NOW() - INTERVAL '5 minutes')`
		res, err := c.db.Exec(query, key)
		if err != nil {
			return count, errors.New("failed to queue issue key: " + err.Error())
		}
		rowsAffected, _ := res.RowsAffected()
		count += int(rowsAffected)
	}
	return count, nil
}

func (c *DBClient) GetQueuedIssueKeys(ctx context.Context, limit int) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT issue_key FROM sync_queue ORDER BY queued_date ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (c *DBClient) RemoveQueuedIssueKey(ctx context.Context, key string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM sync_queue WHERE issue_key = $1`, key)
	return err
}

func (c *DBClient) GetMaxIssueNumberForPrefix(ctx context.Context, prefix string) (int, error) {
	var max int
	row := c.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(key_num), 0) FROM issue WHERE project = $1`, prefix)
	err := row.Scan(&max)
	return max, err
}

func (c *DBClient) GetSyncState(ctx context.Context, prefix string) (int, error) {
	var last int
	row := c.db.QueryRowContext(ctx, `SELECT last_processed FROM sync_state WHERE prefix = $1`, prefix)
	err := row.Scan(&last)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return last, err
}

func (c *DBClient) SetSyncState(ctx context.Context, prefix string, last int) error {
	_, err := c.db.ExecContext(ctx, `INSERT INTO sync_state (prefix, last_processed) VALUES ($1, $2)
		ON CONFLICT (prefix) DO UPDATE SET last_processed = EXCLUDED.last_processed`, prefix, last)
	return err
}

func (c *DBClient) GetSyncStats(ctx context.Context) ([]struct {
	Project   string
	MaxKeyNum int
	Count     int
	Percent   float64
}, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT project, COALESCE(MAX(key_num),0) AS max_key_num, COUNT(*) AS count
		FROM issue
		WHERE missing = FALSE
		GROUP BY project
		ORDER BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []struct {
		Project   string
		MaxKeyNum int
		Count     int
		Percent   float64
	}
	for rows.Next() {
		var s struct {
			Project   string
			MaxKeyNum int
			Count     int
			Percent   float64
		}
		if err := rows.Scan(&s.Project, &s.MaxKeyNum, &s.Count); err != nil {
			return nil, err
		}
		if s.MaxKeyNum > 0 {
			s.Percent = float64(s.Count) / float64(s.MaxKeyNum) * 100
		} else {
			s.Percent = 0
		}
		stats = append(stats, s)
	}
	return stats, nil
}
