package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"mojira/model"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"
)

type DBClient struct {
	db *sql.DB
}

func NewDBClient() (*DBClient, error) {
	log.Println("Connecting to database...")
	connStr := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &DBClient{db: db}, nil
}

func (c *DBClient) RunMigration(filepath string) error {
	log.Printf("Running migration '%s'...\n", filepath)
	sqlBytes, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(string(sqlBytes))
	if err != nil {
		return err
	}
	log.Printf("Migration '%s' executed successfully\n", filepath)
	return nil
}

func (c *DBClient) GetAllIssues(limit int) ([]model.Issue, error) {
	rows, err := c.db.Query("SELECT key, summary, reporter_name, created_date FROM issue WHERE state = 'present' ORDER BY created_date DESC LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		if err := rows.Scan(&issue.Key, &issue.Summary, &issue.ReporterName, &issue.CreatedDate); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *DBClient) SearchIssues(text string, limit int) ([]model.Issue, error) {
	// Disallow queries starting with "-" for performance reasons
	if strings.HasPrefix(strings.TrimSpace(text), "-") {
		return []model.Issue{}, nil
	}

	query := `(
			SELECT key, summary, created_date
			FROM issue
			WHERE state = 'present' AND to_tsvector('english', summary) @@ websearch_to_tsquery('english', $1)
			LIMIT $2
		)
		UNION
		(
			SELECT key, summary, created_date
			FROM issue
			WHERE state = 'present' AND to_tsvector('english', text) @@ websearch_to_tsquery('english', $1)
			LIMIT $2
		)
		ORDER BY created_date DESC
		LIMIT $2;`
	rows, err := c.db.Query(query, text, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		if err := rows.Scan(&issue.Key, &issue.Summary, &issue.CreatedDate); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *DBClient) FilterIssues(project string, status string, confirmation string, resolution string, mojangPriority string, sort string, limit int) ([]model.Issue, error) {
	sortStr := `created_date DESC`
	filterStr := ``
	if sort == "Updated" {
		sortStr = `updated_date DESC`
		filterStr = ` AND (updated_date IS NOT NULL)`
	} else if sort == "Resolved" {
		sortStr = `resolved_date DESC`
		filterStr = ` AND (resolved_date IS NOT NULL)`
	} else if sort == "Votes" {
		sortStr = `votes DESC`
	}
	rows, err := c.db.Query(`SELECT key, summary, reporter_name, created_date FROM issue WHERE state = 'present' AND ($1 = '' OR project = $1) AND ($2 = '' OR status = $2) AND ($3 = '' OR confirmation_status = $3) AND ($4 = '' OR resolution = $4) AND ($5 = '' OR mojang_priority = $5)`+filterStr+` ORDER BY `+sortStr+` LIMIT $6`, project, status, confirmation, resolution, mojangPriority, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		if err := rows.Scan(&issue.Key, &issue.Summary, &issue.ReporterName, &issue.CreatedDate); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *DBClient) GetIssueForSync(key string) (*model.Issue, error) {
	row := c.db.QueryRow("SELECT synced_date FROM issue WHERE key = $1 AND state != 'unknown'", key)
	var issue model.Issue
	issue.Key = key
	err := row.Scan(&issue.SyncedDate)
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

func (c *DBClient) GetIssueByKey(key string) (*model.Issue, error) {
	row := c.db.QueryRow("SELECT summary, reporter_name, reporter_avatar, assignee_name, assignee_avatar, description, environment, labels, created_date, updated_date, resolved_date, status, confirmation_status, resolution, affected_versions, fix_versions, category, mojang_priority, area, components, ado, platform, os_version, realms_platform, votes, synced_date, state FROM issue WHERE key = $1 AND state != 'unknown'", key)
	var state string
	var issue model.Issue
	issue.Key = key
	err := row.Scan(&issue.Summary, &issue.ReporterName, &issue.ReporterAvatar, &issue.AssigneeName, &issue.AssigneeAvatar, &issue.Description, &issue.Environment, pq.Array(&issue.Labels), &issue.CreatedDate, &issue.UpdatedDate, &issue.ResolvedDate, &issue.Status, &issue.ConfirmationStatus, &issue.Resolution, pq.Array(&issue.AffectedVersions), pq.Array(&issue.FixVersions), pq.Array(&issue.Category), &issue.MojangPriority, &issue.Area, pq.Array(&issue.Components), &issue.ADO, &issue.Platform, &issue.OSVersion, &issue.RealmsPlatform, &issue.Votes, &issue.SyncedDate, &state)
	if err != nil {
		return nil, err
	}
	if state == "removed" {
		return nil, model.ErrIssueRemoved
	}
	comments := []model.Comment{}
	rows, err := c.db.Query(`SELECT comment_id, date, author_name, author_avatar, adf_comment FROM comment WHERE issue_key = $1 ORDER BY date ASC`, key)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cmt model.Comment
			if err := rows.Scan(&cmt.Id, &cmt.Date, &cmt.AuthorName, &cmt.AuthorAvatar, &cmt.AdfComment); err == nil {
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
	rows, err = c.db.Query(`SELECT attachment_id, filename, author_name, author_avatar, created_date, size, mime_type FROM attachment WHERE issue_key = $1`, key)
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
	if issue.Partial {
		return errors.New("tried to insert a partial issue")
	}
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
	var textParts []string
	if issue.Summary != "" {
		textParts = append(textParts, issue.Summary)
	}
	if issue.Description != "" {
		textParts = append(textParts, model.ExtractPlainTextFromADF(issue.Description))
	}
	if issue.Environment != "" {
		textParts = append(textParts, model.ExtractPlainTextFromADF(issue.Environment))
	}
	for _, cmt := range issue.Comments {
		if cmt.AdfComment != "" {
			textParts = append(textParts, model.ExtractPlainTextFromADF(cmt.AdfComment))
		}
	}
	text := strings.Join(textParts, "\n")

	_, err := tx.Exec(`DELETE FROM issue WHERE key = $1`, issue.Key)
	if err != nil {
		return err
	}
	query := `INSERT INTO issue (key, summary, reporter_name, reporter_avatar, assignee_name, assignee_avatar, description, environment, labels, created_date, updated_date, resolved_date, status, confirmation_status, resolution, affected_versions, fix_versions, category, mojang_priority, area, components, ado, platform, os_version, realms_platform, votes, text, synced_date, state) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, 'present')`
	_, err = tx.Exec(query, issue.Key, issue.Summary, issue.ReporterName, issue.ReporterAvatar, issue.AssigneeName, issue.AssigneeAvatar, issue.Description, issue.Environment, pq.Array(issue.Labels), issue.CreatedDate, issue.UpdatedDate, issue.ResolvedDate, issue.Status, issue.ConfirmationStatus, issue.Resolution, pq.Array(issue.AffectedVersions), pq.Array(issue.FixVersions), pq.Array(issue.Category), issue.MojangPriority, issue.Area, pq.Array(issue.Components), issue.ADO, issue.Platform, issue.OSVersion, issue.RealmsPlatform, issue.Votes, text, issue.SyncedDate)
	if err != nil {
		return errors.New("failed to insert issue: " + err.Error())
	}
	for _, cmt := range issue.Comments {
		_, err = tx.Exec(`INSERT INTO comment (issue_key, comment_id, date, author_name, author_avatar, adf_comment) VALUES ($1, $2, $3, $4, $5, $6)`, issue.Key, cmt.Id, cmt.Date, cmt.AuthorName, cmt.AuthorAvatar, cmt.AdfComment)
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
		_, err = tx.Exec(`INSERT INTO attachment (issue_key, attachment_id, filename, author_name, author_avatar, created_date, size, mime_type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, issue.Key, a.Id, a.Filename, a.AuthorName, a.AuthorAvatar, a.CreatedDate, a.Size, a.MimeType)
		if err != nil {
			return errors.New("failed to insert attachment: " + err.Error())
		}
	}
	return nil
}

func (c *DBClient) MarkIssueRemoved(key string) error {
	query := `UPDATE issue SET state = 'removed' WHERE key = $1`
	_, err := c.db.Exec(query, key)
	if err != nil {
		return errors.New("failed to mark issue as removed: " + err.Error())
	}
	return nil
}

func (c *DBClient) QueueIssueKeys(keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	query := `
		WITH new_keys AS (
			SELECT k AS issue_key
			FROM UNNEST($1::text[]) AS k
			WHERE
				NOT EXISTS (SELECT 1 FROM sync_queue q WHERE q.issue_key = k)
				AND NOT EXISTS (SELECT 1 FROM issue i WHERE i.key = k AND i.synced_date >= NOW() - INTERVAL '5 minutes')
		)
		INSERT INTO sync_queue (issue_key)
		SELECT issue_key FROM new_keys
		RETURNING issue_key;
	`
	rows, err := c.db.Query(query, pq.Array(keys))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		result = append(result, key)
	}
	return result, nil
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

func (c *DBClient) ReQueueIssueKey(ctx context.Context, key string) error {
	_, err := c.db.ExecContext(ctx, `UPDATE sync_queue SET queued_date = NOW() WHERE issue_key = $1`, key)
	return err
}

func (c *DBClient) GetMaxIssueNumberForPrefix(ctx context.Context, prefix string) (int, error) {
	var max int
	row := c.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(key_num), 0) FROM issue WHERE project = $1 AND state != 'unknown'`, prefix)
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

type SyncStateRow struct {
	Project   string
	MaxKeyNum int
	Count     int
	Percent   float64
}

func (c *DBClient) GetSyncStats(ctx context.Context) ([]SyncStateRow, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT project, COALESCE(MAX(key_num),0) AS max_key_num, COUNT(*) AS count
		FROM issue
		GROUP BY project
		ORDER BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []SyncStateRow
	for rows.Next() {
		var s SyncStateRow
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

type QueueRow struct {
	Key        string
	QueuedDate *time.Time
}

func (c *DBClient) GetQueue(ctx context.Context) ([]QueueRow, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT issue_key, queued_date FROM sync_queue`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var queue []QueueRow
	for rows.Next() {
		var q QueueRow
		if err := rows.Scan(&q.Key, &q.QueuedDate); err != nil {
			return nil, err
		}
		queue = append(queue, q)
	}
	return queue, nil
}
