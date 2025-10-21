package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"math"
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
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(string(sqlBytes))
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
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

func (c *DBClient) FilterIssues(search string, project string, status string, confirmation string, resolution string, priority string, reporter string, assignee string, affected_version string, fix_version string, category string, label string, component string, platform string, area string, sort string, offset int, limit int) ([]model.Issue, int, error) {
	// Disallow queries starting with "-" for performance reasons
	if strings.HasPrefix(strings.TrimSpace(search), "-") {
		return []model.Issue{}, 0, nil
	}
	sortStr := `created_date DESC`
	filterStr := ``
	switch sort {
	case "Updated":
		sortStr = `updated_date DESC`
		filterStr += ` AND (updated_date IS NOT NULL)`
	case "Resolved":
		sortStr = `resolved_date DESC`
		filterStr += ` AND (resolved_date IS NOT NULL)`
	case "Priority":
		sortStr = `mojang_priority_rank DESC`
	case "Votes":
		sortStr = `total_votes DESC, created_date DESC`
	case "Comments":
		sortStr = `comment_count DESC`
	case "Duplicates":
		sortStr = `duplicate_count DESC`
	}
	rows, err := c.db.Query(`SELECT key, summary, status, resolution, confirmation_status, reporter_avatar, reporter_name, assignee_avatar, assignee_name, created_date, total_votes FROM issue WHERE state = 'present' AND ($2 = '' OR project = $2) AND ($3 = '' OR status = $3) AND ($4 = '' OR confirmation_status = $4) AND ($5 = '' OR resolution = $5 OR (resolution = '' AND $5 = 'Unresolved')) AND ($6 = '' OR mojang_priority = $6) AND ($7 = '' OR reporter_name = $7) AND ($8 = '' OR assignee_name = $8) AND ($9 = '' OR $9=ANY(affected_versions)) AND ($10 = '' OR $10=ANY(fix_versions)) AND ($11 = '' OR $11=ANY(category)) AND ($12 = '' OR $12=ANY(labels)) AND ($13 = '' OR $13=ANY(components)) AND ($14 = '' OR platform = $14) AND ($15 = '' OR area = $15) AND ($1 = '' OR to_tsvector('english', text) @@ websearch_to_tsquery('english', $1))`+filterStr+` ORDER BY `+sortStr+` OFFSET $16 LIMIT $17`, search, project, status, confirmation, resolution, priority, reporter, assignee, affected_version, fix_version, category, label, component, platform, area, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		var ignoredTotalVotes int
		if err := rows.Scan(&issue.Key, &issue.Summary, &issue.Status, &issue.Resolution, &issue.ConfirmationStatus, &issue.ReporterAvatar, &issue.ReporterName, &issue.AssigneeAvatar, &issue.AssigneeName, &issue.CreatedDate, &ignoredTotalVotes); err != nil {
			return nil, 0, err
		}
		issues = append(issues, issue)
	}

	var count int
	if search == "" && priority == "" && reporter == "" && assignee == "" && affected_version == "" && fix_version == "" && category == "" && label == "" && component == "" && platform == "" && area == "" {
		countRow := c.db.QueryRow(`SELECT COALESCE(SUM(count), 0) FROM issue_count WHERE ($1 = '' OR project = $1) AND ($2 = '' OR status = $2) AND ($3 = '' OR confirmation_status = $3) AND ($4 = '' OR resolution = $4 OR (resolution = '' AND $4 = 'Unresolved'))`, project, status, confirmation, resolution)
		err = countRow.Scan(&count)
		if err != nil {
			return nil, 0, err
		}
	} else {
		countRow := c.db.QueryRow(`SELECT COUNT(*) FROM issue WHERE state = 'present' AND ($2 = '' OR project = $2) AND ($3 = '' OR status = $3) AND ($4 = '' OR confirmation_status = $4) AND ($5 = '' OR resolution = $5 OR (resolution = '' AND $5 = 'Unresolved')) AND ($6 = '' OR mojang_priority = $6) AND ($7 = '' OR reporter_name = $7) AND ($8 = '' OR assignee_name = $8) AND ($9 = '' OR $9=ANY(affected_versions)) AND ($10 = '' OR $10=ANY(fix_versions)) AND ($11 = '' OR $11=ANY(category)) AND ($12 = '' OR $12=ANY(labels)) AND ($13 = '' OR $13=ANY(components)) AND ($14 = '' OR platform = $14) AND ($15 = '' OR area = $15) AND ($1 = '' OR to_tsvector('english', text) @@ websearch_to_tsquery('english', $1))`+filterStr, search, project, status, confirmation, resolution, priority, reporter, assignee, affected_version, fix_version, category, label, component, platform, area)
		err = countRow.Scan(&count)
		if err != nil {
			return nil, 0, err
		}
	}
	return issues, count, nil
}

func (c *DBClient) GetIssueByReporter(reporter string, limit int) ([]model.Issue, error) {
	rows, err := c.db.Query(`SELECT key, summary, status, resolution, confirmation_status, reporter_avatar, reporter_name, created_date FROM issue WHERE state = 'present' AND reporter_name = $1 ORDER BY created_date DESC LIMIT $2`, reporter, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		if err := rows.Scan(&issue.Key, &issue.Summary, &issue.Status, &issue.Resolution, &issue.ConfirmationStatus, &issue.ReporterAvatar, &issue.ReporterName, &issue.CreatedDate); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *DBClient) GetIssueByAssignee(assignee string, limit int) ([]model.Issue, error) {
	rows, err := c.db.Query(`SELECT key, summary, status, resolution, confirmation_status, assignee_avatar, assignee_name, created_date FROM issue WHERE state = 'present' AND assignee_name = $1 ORDER BY created_date DESC LIMIT $2`, assignee, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var issues []model.Issue
	for rows.Next() {
		var issue model.Issue
		if err := rows.Scan(&issue.Key, &issue.Summary, &issue.Status, &issue.Resolution, &issue.ConfirmationStatus, &issue.AssigneeAvatar, &issue.AssigneeName, &issue.CreatedDate); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *DBClient) GetIssueForSync(key string) (*model.Issue, error) {
	row := c.db.QueryRow("SELECT synced_date FROM issue WHERE key = $1", key)
	var issue model.Issue
	issue.Key = key
	err := row.Scan(&issue.SyncedDate)
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

func (c *DBClient) GetIssueByKey(key string) (*model.Issue, error) {
	row := c.db.QueryRow("SELECT summary, creator_name, creator_avatar, reporter_name, reporter_avatar, assignee_name, assignee_avatar, description, environment, labels, created_date, updated_date, resolved_date, status, confirmation_status, resolution, affected_versions, fix_versions, category, mojang_priority, area, components, ado, platform, os_version, realms_platform, votes, legacy_votes, synced_date, state FROM issue WHERE key = $1", key)
	var state string
	var issue model.Issue
	issue.Key = key
	err := row.Scan(&issue.Summary, &issue.CreatorName, &issue.CreatorAvatar, &issue.ReporterName, &issue.ReporterAvatar, &issue.AssigneeName, &issue.AssigneeAvatar, &issue.Description, &issue.Environment, pq.Array(&issue.Labels), &issue.CreatedDate, &issue.UpdatedDate, &issue.ResolvedDate, &issue.Status, &issue.ConfirmationStatus, &issue.Resolution, pq.Array(&issue.AffectedVersions), pq.Array(&issue.FixVersions), pq.Array(&issue.Category), &issue.MojangPriority, &issue.Area, pq.Array(&issue.Components), &issue.ADO, &issue.Platform, &issue.OSVersion, &issue.RealmsPlatform, &issue.Votes, &issue.LegacyVotes, &issue.SyncedDate, &state)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrIssueNotStored
		}
		return nil, err
	}
	if state == "removed" {
		return nil, model.ErrIssueRemoved
	}
	comments := []model.Comment{}
	rows, err := c.db.Query(`SELECT comment_id, legacy_id, date, author_name, author_avatar, adf_comment FROM comment WHERE issue_key = $1 ORDER BY date ASC`, key)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cmt model.Comment
			err := rows.Scan(&cmt.Id, &cmt.LegacyId, &cmt.Date, &cmt.AuthorName, &cmt.AuthorAvatar, &cmt.AdfComment)
			if err != nil {
				return nil, err
			}
			cmt.Issue = &issue
			comments = append(comments, cmt)
		}
	}
	issue.Comments = comments
	links := []model.IssueLink{}
	rows, err = c.db.Query(`SELECT type, other_key, other_summary, other_status FROM issue_link WHERE issue_key = $1`, key)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var l model.IssueLink
			err := rows.Scan(&l.Type, &l.OtherKey, &l.OtherSummary, &l.OtherStatus)
			if err != nil {
				return nil, err
			}
			links = append(links, l)
		}
	}
	issue.Links = links
	attachments := []model.Attachment{}
	rows, err = c.db.Query(`SELECT attachment_id, filename, author_name, author_avatar, created_date, size, mime_type FROM attachment WHERE issue_key = $1`, key)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a model.Attachment
			err := rows.Scan(&a.Id, &a.Filename, &a.AuthorName, &a.AuthorAvatar, &a.CreatedDate, &a.Size, &a.MimeType)
			if err != nil {
				return nil, err
			}
			attachments = append(attachments, a)
		}
	}
	issue.Attachments = attachments
	return &issue, nil
}

func (c *DBClient) GetCommentsByUser(name string, limit int) ([]model.Comment, error) {
	comments := []model.Comment{}
	query := `SELECT c.issue_key, c.comment_id, c.legacy_id, c.date, c.author_name, c.author_avatar, c.adf_comment
		FROM comment c
		JOIN issue i ON c.issue_key = i.key
		WHERE c.author_name = $1 AND i.state = 'present'
		ORDER BY c.date DESC
		LIMIT $2;`
	rows, err := c.db.Query(query, name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cmt model.Comment
		var issueKey string
		err := rows.Scan(&issueKey, &cmt.Id, &cmt.LegacyId, &cmt.Date, &cmt.AuthorName, &cmt.AuthorAvatar, &cmt.AdfComment)
		if err != nil {
			return nil, err
		}
		cmt.Issue = &model.Issue{Key: issueKey}
		comments = append(comments, cmt)
	}
	return comments, nil
}

func (c *DBClient) UpdateIssue(ctx context.Context, issue *model.Issue) error {
	if issue.Partial {
		return errors.New("tried to insert a partial issue")
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := c.updateIssueImpl(tx, issue); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (c *DBClient) updateIssueImpl(tx *sql.Tx, issue *model.Issue) error {
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
	duplicateCount := 0
	for _, l := range issue.Links {
		if l.Type == "is duplicated by" {
			duplicateCount += 1
		}
	}

	_, err := tx.Exec(`INSERT INTO issue (key, creator_name, creator_avatar, synced_date, state) VALUES ($1, '', '', NOW(), 'present') ON CONFLICT DO NOTHING`, issue.Key)
	if err != nil {
		return err
	}
	query := `UPDATE issue SET summary = $2, creator_name = $3, creator_avatar = $4, reporter_name = $5, reporter_avatar = $6, assignee_name = $7, assignee_avatar = $8, description = $9, environment = $10, labels = $11, created_date = $12, updated_date = $13, resolved_date = $14, status = $15, confirmation_status = $16, resolution = $17, affected_versions = $18, fix_versions = $19, category = $20, mojang_priority = $21, area = $22, components = $23, ado = $24, platform = $25, os_version = $26, realms_platform = $27, votes = $28, legacy_votes = $29, text = $30, comment_count = $31, duplicate_count = $32, synced_date = $33, state = 'present' WHERE key = $1`
	_, err = tx.Exec(query, issue.Key, issue.Summary, issue.CreatorName, issue.CreatorAvatar, issue.ReporterName, issue.ReporterAvatar, issue.AssigneeName, issue.AssigneeAvatar, issue.Description, issue.Environment, pq.Array(issue.Labels), issue.CreatedDate, issue.UpdatedDate, issue.ResolvedDate, issue.Status, issue.ConfirmationStatus, issue.Resolution, pq.Array(issue.AffectedVersions), pq.Array(issue.FixVersions), pq.Array(issue.Category), issue.MojangPriority, issue.Area, pq.Array(issue.Components), issue.ADO, issue.Platform, issue.OSVersion, issue.RealmsPlatform, issue.Votes, issue.LegacyVotes, text, len(issue.Comments), duplicateCount, issue.SyncedDate)
	if err != nil {
		return errors.New("failed to update issue: " + err.Error())
	}

	_, err = tx.Exec(`DELETE FROM comment WHERE issue_key = $1`, issue.Key)
	if err != nil {
		return errors.New("failed to delete comments: " + err.Error())
	}
	for _, cmt := range issue.Comments {
		_, err = tx.Exec(`INSERT INTO comment (issue_key, comment_id, legacy_id, date, author_name, author_avatar, adf_comment) VALUES ($1, $2, $3, $4, $5, $6, $7)`, issue.Key, cmt.Id, cmt.LegacyId, cmt.Date, cmt.AuthorName, cmt.AuthorAvatar, cmt.AdfComment)
		if err != nil {
			return errors.New("failed to insert comment: " + err.Error())
		}
	}
	_, err = tx.Exec(`DELETE FROM issue_link WHERE issue_key = $1`, issue.Key)
	if err != nil {
		return errors.New("failed to delete issue links: " + err.Error())
	}
	for _, l := range issue.Links {
		_, err = tx.Exec(`INSERT INTO issue_link (issue_key, type, other_key, other_summary, other_status) VALUES ($1, $2, $3, $4, $5)`, issue.Key, l.Type, l.OtherKey, l.OtherSummary, l.OtherStatus)
		if err != nil {
			return errors.New("failed to insert issue_link: " + err.Error())
		}
	}
	_, err = tx.Exec(`DELETE FROM attachment WHERE issue_key = $1`, issue.Key)
	if err != nil {
		return errors.New("failed to delete attachments: " + err.Error())
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

func (c *DBClient) QueueIssueKeys(keys []string, priority int, reason string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	query := `
		WITH new_keys AS (
			SELECT k AS issue_key
			FROM UNNEST($1::text[]) AS k
			WHERE
				NOT EXISTS (SELECT 1 FROM sync_queue q WHERE q.issue_key = k)
				AND NOT EXISTS (SELECT 1 FROM issue i WHERE i.key = k AND i.synced_date >= NOW() - INTERVAL '15 minutes')
		)
		INSERT INTO sync_queue (issue_key, priority, reason)
		SELECT issue_key, $2, $3 FROM new_keys
		RETURNING issue_key
	`
	rows, err := c.db.Query(query, pq.Array(keys), priority, reason)
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

func (c *DBClient) PeekQueuedIssues(ctx context.Context, limit int) ([]string, error) {
	query := `SELECT issue_key
		FROM sync_queue
		WHERE retry_after <= NOW()
		ORDER BY priority DESC, failed_count ASC, queued_date ASC
		LIMIT $1`
	rows, err := c.db.QueryContext(ctx, query, limit)
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

func (c *DBClient) PeekFutureVersionIssues(ctx context.Context, limit int) ([]string, error) {
	query := `SELECT key
		FROM issue
		WHERE EXISTS (
			SELECT 1 FROM unnest(fix_versions) AS v
			WHERE v LIKE 'Future%'
		)
		LIMIT $1`
	rows, err := c.db.QueryContext(ctx, query, limit)
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

func (c *DBClient) RetryQueuedIssue(ctx context.Context, key string) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var failedCount int
	var priority int
	query := `SELECT failed_count, priority FROM sync_queue WHERE issue_key = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, query, key).Scan(&failedCount, &priority)
	if err != nil {
		return err
	}

	failedCount += 1
	if failedCount > 4 && priority < 10 {
		_, err = tx.ExecContext(ctx, `DELETE FROM sync_queue WHERE issue_key = $1`, key)
	} else {
		// Delay will be: 5m, 25m, 2h5m, 10h25m
		delay := time.Duration(math.Pow(5, float64(failedCount))) * time.Minute
		retryAfter := time.Now().Add(delay)
		_, err = tx.ExecContext(ctx, `
			UPDATE sync_queue
			SET 
				failed_count = $2,
				queued_date = NOW(),
				retry_after = $3
			WHERE issue_key = $1
		`, key, failedCount, retryAfter)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (c *DBClient) DeleteQueuedIssue(ctx context.Context, key string) error {
	query := `DELETE FROM sync_queue WHERE issue_key = $1`
	_, err := c.db.ExecContext(ctx, query, key)
	return err
}

func (c *DBClient) GetQueueSize(ctx context.Context) (int, error) {
	row := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue`)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

type QueueRow struct {
	Key         string
	QueuedDate  *time.Time
	Priority    int
	Reason      string
	FailedCount int
	RetryAfter  *time.Time
}

func (c *DBClient) GetQueue(ctx context.Context) ([]QueueRow, int, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT issue_key, queued_date, priority, reason, failed_count, retry_after FROM sync_queue ORDER BY priority DESC, queued_date ASC LIMIT 100`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var queue []QueueRow
	for rows.Next() {
		var q QueueRow
		if err := rows.Scan(&q.Key, &q.QueuedDate, &q.Priority, &q.Reason, &q.FailedCount, &q.RetryAfter); err != nil {
			return nil, 0, err
		}
		queue = append(queue, q)
	}
	count, err := c.GetQueueSize(ctx)
	if err != nil {
		return nil, 0, err
	}
	return queue, count, nil
}
