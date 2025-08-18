package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"mojira/model"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func render(w http.ResponseWriter, name string, data any) {
	if !strings.HasSuffix(name, ".html") {
		name = fmt.Sprintf("%s.html", name)
	}

	tmpl, err := template.New(filepath.Base(name)).Funcs(template.FuncMap{
		"formatTime": formatTime,
		"renderADF":  model.RenderADF,
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
		sort := query.Get("sort")
		t0 := time.Now()
		issues, count, err := service.db.FilterIssues(search, project, status, confirmation, resolution, priority, sort, 50)
		t1 := time.Now()
		if t1.Sub(t0) > time.Duration(4)*time.Second {
			log.Printf("[WARNING] Slow filter! %s: project=%s status=%s confirmation=%s resolution=%s priority=%s sort=%s search=%s", t1.Sub(t0), project, status, confirmation, resolution, priority, sort, search)
		}
		if err != nil {
			log.Printf("[ERROR] FilterIssues: %s", err)
			issues = []model.Issue{}
		}
		if r.Header.Get("Hx-Request") != "" {
			filtered := url.Values{}
			for k, v := range query {
				if len(v) > 0 && v[0] != "" {
					filtered[k] = v
				}
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
