package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"maps"
	"mojira/model"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var pageSize = 50

func render(w http.ResponseWriter, name string, data any) {
	if !strings.HasSuffix(name, ".html") {
		name = fmt.Sprintf("%s.html", name)
	}

	tmpl, err := template.New(filepath.Base(name)).Funcs(template.FuncMap{
		"formatTime": formatTime,
		"previewADF": func(adf string) string {
			text := model.ExtractPlainTextFromADF(adf)
			if len(text) > 200 {
				return text[:200] + "..."
			}
			return text
		},
		"icon": icon,
		"join": func(arr []string) string {
			return strings.Join(arr, ", ")
		},
		"add": func(a int, b int) int {
			return a + b
		},
	}).ParseFiles("templates/base.html", fmt.Sprintf("templates/%s", name))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	filePath := filepath.Join("static", path)
	f, err := os.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}

	etag := fmt.Sprintf(`W/"%x-%x"`, fi.ModTime().Unix(), fi.Size())
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if strings.HasSuffix(r.URL.Path, ".png") || strings.HasSuffix(r.URL.Path, ".svg") {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}

	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

func issueRedirectHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	w.Header().Set("Location", fmt.Sprintf("/%s", key))
	w.WriteHeader(301)
}

func indexHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		search := query.Get("search")
		project := query.Get("project")
		status := query.Get("status")
		confirmation := query.Get("confirmation")
		resolution := query.Get("resolution")
		priority := query.Get("priority")
		reporter := query.Get("reporter")
		assignee := query.Get("assignee")
		affected_version := query.Get("affected_version")
		fix_version := query.Get("fix_version")
		category := query.Get("category")
		label := query.Get("label")
		component := query.Get("component")
		platform := query.Get("platform")
		area := query.Get("area")
		sort := query.Get("sort")
		page, err := strconv.Atoi(query.Get("page"))
		if err != nil {
			page = 1
		}
		page = max(page, 1)
		offset := (page - 1) * pageSize

		t0 := time.Now()
		issues, count, err := service.db.FilterIssues(search, project, status, confirmation, resolution, priority, reporter, assignee, affected_version, fix_version, category, label, component, platform, area, sort, offset, pageSize)
		t1 := time.Now()
		if t1.Sub(t0) > time.Duration(4)*time.Second {
			log.Printf("[WARNING] Slow filter! %s: project=%s status=%s confirmation=%s resolution=%s priority=%s sort=%s search=%s", t1.Sub(t0), project, status, confirmation, resolution, priority, sort, search)
		}
		if err != nil {
			log.Printf("[ERROR] FilterIssues: %s", err)
			issues = []model.Issue{}
		}

		outage, err := service.db.GetSyncOutage(r.Context())
		if err != nil {
			log.Printf("[ERROR] GetQueueSize: %s", err)
		}

		if r.Header.Get("Hx-Request") != "" {
			filtered := url.Values{}
			for k, v := range query {
				if len(v) > 0 && v[0] != "" {
					filtered[k] = v
				}
			}
			if page > 1 {
				filtered["page"] = []string{strconv.Itoa(page)}
			} else {
				filtered["page"] = nil
			}
			u := *r.URL
			u.RawQuery = filtered.Encode()
			w.Header().Add("Hx-Replace-Url", u.String())
		}
		queryMap := make(map[string]string)
		for k, v := range query {
			if len(v) > 0 {
				queryMap[k] = v[0]
			}
		}
		render(w, "pages/index", map[string]any{
			"Issues": issues,
			"Count":  count,
			"Query":  queryMap,
			"Page":   page,
			"Outage": outage,
		})
	}
}

func issueHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		issue, err := service.GetIssue(r.Context(), key)
		if err != nil {
			if errors.Is(err, model.ErrIssueRemoved) {
				render(w, "pages/issue_removed", map[string]any{
					"Key": key,
				})
				return
			}
			render(w, "pages/issue_not_found", map[string]any{
				"Key": key,
			})
			return
		}
		render(w, "pages/issue", map[string]any{
			"Issue": issue,
		})
	}
}

func userHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userName := r.PathValue("name")
		assignedIssues, err := service.db.GetIssueByAssignee(userName, 20)
		if err != nil {
			log.Printf("[ERROR] GetIssueByAssignee: %s", err)
			assignedIssues = []model.Issue{}
		}
		reportedIssues, err := service.db.GetIssueByReporter(userName, 20)
		if err != nil {
			log.Printf("[ERROR] GetIssueByReporter: %s", err)
			reportedIssues = []model.Issue{}
		}
		comments, err := service.db.GetCommentsByUser(userName, 20)
		if err != nil {
			log.Printf("[ERROR] GetCommentsByUser: %s", err)
			comments = []model.Comment{}
		}
		if len(assignedIssues) == 0 && len(reportedIssues) == 0 && len(comments) == 0 {
			render(w, "pages/user_not_found", map[string]any{})
			return
		}
		avatarSet := make(map[string]struct{})
		for _, issue := range assignedIssues {
			avatarSet[issue.AssigneeAvatar] = struct{}{}
		}
		for _, issue := range reportedIssues {
			avatarSet[issue.ReporterAvatar] = struct{}{}
		}
		for _, comment := range comments {
			avatarSet[comment.AuthorAvatar] = struct{}{}
		}
		delete(avatarSet, "")
		render(w, "pages/user", map[string]any{
			"UserName":        userName,
			"UserAvatars":     slices.Collect(maps.Keys(avatarSet)),
			"MultipleAvatars": len(avatarSet) > 1,
			"AssignedIssues":  assignedIssues,
			"ReportedIssues":  reportedIssues,
			"Comments":        comments,
		})
	}
}

func queueOverviewHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		queue, count, err := service.db.GetQueue(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, "pages/queue", map[string]any{
			"Queue": queue,
			"Count": count,
		})
	}
}

func apiSearchHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		search := r.Form.Get("search")
		if len(search) == 0 {
			w.Write([]byte(""))
			return
		}
		t0 := time.Now()
		issues, err := service.db.SearchIssues(search, 10)
		t1 := time.Now()
		if t1.Sub(t0) > time.Duration(4)*time.Second {
			log.Printf("[WARNING] Slow search! %s: search=%s", t1.Sub(t0), search)
		}
		if err != nil {
			log.Printf("[ERROR] Failed searching for '%s': %s", search, err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, "partials/search_results", map[string]any{
			"Issues": issues,
		})
	}
}

func apiRefreshHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		issue, err := service.RefreshIssue(r.Context(), key)
		if err != nil {
			if errors.Is(err, model.ErrIssueRemoved) {
				render(w, "pages/issue_removed", map[string]any{
					"Key": key,
				})
				return
			}
			log.Printf("Failed refreshing %s: %s", key, err.Error())
			render(w, "partials/sync_failed", map[string]any{
				"SyncedDate": issue.SyncedDate,
			})
			return
		}
		if issue == nil {
			return
		}
		render(w, "pages/issue", map[string]any{
			"Issue":     issue,
			"IsRefresh": true,
		})
	}
}

type V1Issue = struct {
	Key                string     `json:"key"`
	Summary            string     `json:"summary"`
	ReporterName       *string    `json:"reporter_name"`
	ReporterAvatar     *string    `json:"reporter_avatar"`
	AssigneeName       *string    `json:"assignee_name"`
	AssigneeAvatar     *string    `json:"assignee_avatar"`
	Description        *string    `json:"description"`
	Environment        *string    `json:"environment"`
	Labels             []string   `json:"labels"`
	CreatedDate        *time.Time `json:"created_date"`
	UpdatedDate        *time.Time `json:"updated_date"`
	ResolvedDate       *time.Time `json:"resolved_date"`
	Status             *string    `json:"status"`
	ConfirmationStatus string     `json:"confirmation_status"`
	Resolution         string     `json:"resolution"`
	AffectedVersions   []string   `json:"affected_versions"`
	FixVersions        []string   `json:"fix_versions"`
	Category           []string   `json:"category"`
	MojangPriority     *string    `json:"mojang_priority"`
	Area               *string    `json:"area"`
	Components         []string   `json:"components"`
	Platform           *string    `json:"platform"`
	OSVersion          *string    `json:"os_version"`
	RealmsPlatform     *string    `json:"realms_platform"`
	ADO                *string    `json:"ado"`
	Votes              int        `json:"votes"`
}

func apiField(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func newV1Issue(issue *model.Issue) V1Issue {
	confirmationStatus := "Unconfirmed"
	if issue.ConfirmationStatus != "" {
		confirmationStatus = issue.ConfirmationStatus
	}
	resolution := "Unresolved"
	if issue.Resolution != "" {
		resolution = issue.Resolution
	}
	category := []string{}
	if issue.Category != nil {
		category = issue.Category
	}
	components := []string{}
	for _, v := range issue.Components {
		if v != "" {
			components = append(components, v)
		}
	}
	return V1Issue{
		Key:                issue.Key,
		Summary:            issue.Summary,
		ReporterName:       apiField(issue.ReporterName),
		ReporterAvatar:     apiField(issue.ReporterAvatar),
		AssigneeName:       apiField(issue.AssigneeName),
		AssigneeAvatar:     apiField(issue.AssigneeAvatar),
		Description:        apiField(issue.Description),
		Environment:        apiField(issue.Environment),
		Labels:             issue.Labels,
		CreatedDate:        issue.CreatedDate,
		UpdatedDate:        issue.UpdatedDate,
		ResolvedDate:       issue.ResolvedDate,
		Status:             apiField(issue.Status),
		ConfirmationStatus: confirmationStatus,
		Resolution:         resolution,
		AffectedVersions:   issue.AffectedVersions,
		FixVersions:        issue.FixVersions,
		Category:           category,
		MojangPriority:     apiField(issue.MojangPriority),
		Area:               apiField(issue.Area),
		Components:         components,
		Platform:           apiField(issue.Platform),
		OSVersion:          apiField(issue.OSVersion),
		RealmsPlatform:     apiField(issue.RealmsPlatform),
		ADO:                apiField(issue.ADO),
		Votes:              issue.LegacyVotes + issue.Votes,
	}
}

func apiV1Issue(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		issue, err := service.GetIssue(r.Context(), key)
		if err != nil {
			if errors.Is(err, model.ErrIssueRemoved) || errors.Is(err, model.ErrIssueNotFound) {
				http.Error(w, "Issue not found", http.StatusNotFound)
				return
			}
			log.Printf("[ERROR] API /v1/issues/%s: %s", key, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "application/json")
		result := newV1Issue(issue)
		err = json.NewEncoder(w).Encode(result)
		if err != nil {
			log.Printf("[ERROR] API /v1/issues/%s: %s", key, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	metricsToken := os.Getenv("METRICS_TOKEN")
	if metricsToken == "" || auth != "Bearer "+metricsToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	promhttp.Handler().ServeHTTP(w, r)
}

func formatTime(t any) string {
	switch v := t.(type) {
	case nil:
		return ""
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.UTC().Format(time.RFC3339)
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

func icon(name string) template.HTML {
	path := fmt.Sprintf("templates/icons/%s.svg", name)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return template.HTML(fmt.Sprintf("<!-- icon '%s' not found -->", name))
	}
	return template.HTML(bytes)
}
