package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"mojira/model"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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
			if len(text) > 250 {
				return text[:250] + "..."
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

	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

func indexHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issues, err := service.db.GetAllIssues(100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, "pages/index", map[string]any{
			"Issues": issues,
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

func syncOverviewHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		stats, err := service.db.GetSyncStats(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		total := struct {
			MaxKeyNum int
			Count     int
			Percent   float64
		}{0, 0, 0}
		for _, s := range stats {
			total.MaxKeyNum += s.MaxKeyNum
			total.Count += s.Count
		}
		if total.MaxKeyNum > 0 {
			total.Percent = float64(total.Count) / float64(total.MaxKeyNum) * 100
		}
		render(w, "pages/sync", map[string]any{
			"Stats": stats,
			"Total": total,
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
		issues, err := service.db.SearchIssues(search, 10)
		if err != nil {
			log.Printf("Failed searching for '%s': %s", search, err.Error())
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

func apiFilterHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		project := query.Get("project")
		status := query.Get("status")
		confirmation := query.Get("confirmation")
		resolution := query.Get("resolution")
		mojangPriority := query.Get("mojang_priority")
		issues, err := service.db.FilterIssues(project, status, confirmation, resolution, mojangPriority, 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, "pages/index", map[string]any{
			"Issues": issues,
		})
	}
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
