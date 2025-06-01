package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kyokomi/emoji/v2"
)

func render(w http.ResponseWriter, name string, data any) {
	if !strings.HasSuffix(name, ".html") {
		name = fmt.Sprintf("%s.html", name)
	}

	tmpl, err := template.New(filepath.Base(name)).Funcs(template.FuncMap{
		"formatTime": formatTime,
		"renderADF":  renderADF,
		"icon":       icon,
		"join":       func(arr []string) string { return strings.Join(arr, ", ") },
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		ctx := r.Context()
		oldIssue, _ := service.db.GetIssueByKey(key)
		if oldIssue != nil && oldIssue.IsUpToDate() {
			return
		}
		issue, err := service.FetchIssue(ctx, key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = service.db.InsertIssue(ctx, issue)
		if err != nil {
			log.Printf("Error inserting issue %s: %v", key, err)
		}
		render(w, "pages/issue", map[string]any{
			"Issue":     issue,
			"IsRefresh": true,
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

func renderADF(adf string) template.HTML {
	if adf == "" {
		return ""
	}
	var node map[string]any
	if err := json.Unmarshal([]byte(adf), &node); err != nil {
		return template.HTML(template.HTMLEscapeString(adf))
	}
	return template.HTML(renderADFNode(node))
}

func renderADFNode(node map[string]any) string {
	typeStr, _ := node["type"].(string)
	switch typeStr {
	case "doc":
		return renderADFChildren(node)
	case "paragraph":
		return "<p>" + renderADFChildren(node) + "</p>"
	case "heading":
		lvl := 1
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if l, ok := attrs["level"].(float64); ok {
				lvl = int(l)
			}
		}
		if lvl < 1 || lvl > 6 {
			lvl = 1
		}
		return fmt.Sprintf("<h%d>%s</h%d>", lvl, renderADFChildren(node), lvl)
	case "blockquote":
		return "<blockquote>" + renderADFChildren(node) + "</blockquote>"
	case "bulletList":
		return "<ul>" + renderADFChildren(node) + "</ul>"
	case "orderedList":
		return "<ol>" + renderADFChildren(node) + "</ol>"
	case "listItem":
		return "<li>" + renderADFChildren(node) + "</li>"
	case "codeBlock":
		return "<pre><code>" + renderADFChildren(node) + "</code></pre>"
	case "rule":
		return "<hr>"
	case "panel":
		panelType := "info"
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if p, ok := attrs["panelType"].(string); ok {
				panelType = p
			}
		}
		return fmt.Sprintf("<div class='panel panel-%s'><img src='/static/icons/%s.svg' alt=''><div>", panelType, panelType) + renderADFChildren(node) + "</div></div>"
	case "table":
		return "<table>" + renderADFChildren(node) + "</table>"
	case "tableRow":
		return "<tr>" + renderADFChildren(node) + "</tr>"
	case "tableCell":
		return "<td>" + renderADFChildren(node) + "</td>"
	case "tableHeader":
		return "<th>" + renderADFChildren(node) + "</th>"
	case "text":
		text, _ := node["text"].(string)
		text = template.HTMLEscapeString(text)
		text = linkifyIssueKeys(text)
		if marks, ok := node["marks"].([]any); ok {
			for _, m := range marks {
				if mark, ok := m.(map[string]any); ok {
					typeMark, _ := mark["type"].(string)
					switch typeMark {
					case "strong":
						text = "<strong>" + text + "</strong>"
					case "em":
						text = "<em>" + text + "</em>"
					case "code":
						text = "<code>" + text + "</code>"
					case "underline":
						text = "<u>" + text + "</u>"
					case "subsup":
						if attrs, ok := mark["attrs"].(map[string]any); ok {
							if typeStr, ok := attrs["type"].(string); ok && (typeStr == "sub" || typeStr == "sup") {
								text = "<" + typeStr + ">" + text + "</" + typeStr + ">"
							}
						}
					case "textColor":
						if attrs, ok := mark["attrs"].(map[string]any); ok {
							if color, ok := attrs["color"].(string); ok {
								text = fmt.Sprintf("<span style='color:%s'>", color) + text + "</span>"
							}
						}
					case "strike":
						text = "<s>" + text + "</s>"
					case "link":
						if attrs, ok := mark["attrs"].(map[string]any); ok {
							if href, ok := attrs["href"].(string); ok {
								text = fmt.Sprintf("<a href='%s' rel='nofollow' target='_blank'>%s</a>", template.HTMLEscapeString(href), text)
							}
						}
					}
				}
			}
		}
		return text
	case "hardBreak":
		return "<br>"
	case "emoji":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if txt, ok := attrs["text"].(string); ok && len([]rune(txt)) == 1 {
				return template.HTMLEscapeString(txt)
			}
			if short, ok := attrs["shortName"].(string); ok {
				return template.HTMLEscapeString(emoji.Sprint(short))
			}
			if txt, ok := attrs["text"].(string); ok {
				return template.HTMLEscapeString(txt)
			}
		}
		return "<span class='placeholder'>[emoji]</span>"
	case "mention":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if text, ok := attrs["text"].(string); ok {
				return template.HTMLEscapeString(text)
			}
		}
		return "@unknown"
	case "mediaSingle":
		return renderADFChildren(node)
	case "mediaGroup":
		return renderADFChildren(node)
	case "media":
		return "<span class='placeholder'>[media]</span>"
	case "inlineCard":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if url, ok := attrs["url"].(string); ok {
				if key := extractIssueKeyFromURL(url); key != "" {
					return fmt.Sprintf("<a href='/%s'>%s</a>", key, key)
				}
				return fmt.Sprintf("<a href='%s' target='_blank'>%s</a>", template.HTMLEscapeString(url), template.HTMLEscapeString(url))
			}
		}
		return "[inlineCard]"
	default:
		return fmt.Sprintf("<span style='color:red;font-weight:bold;'>[%s]</span>", template.HTMLEscapeString(typeStr)) + renderADFChildren(node)
	}
}

func renderADFChildren(node map[string]any) string {
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, c := range content {
		if child, ok := c.(map[string]any); ok {
			sb.WriteString(renderADFNode(child))
		}
	}
	return sb.String()
}

func linkifyIssueKeys(text string) string {
	prefixes := []string{"MC", "MCPE", "MCL", "REALMS", "WEB", "BDS"}
	for _, prefix := range prefixes {
		re := regexp.MustCompile(fmt.Sprintf(`\b(%s-\d+)\b`, prefix))
		text = re.ReplaceAllStringFunc(text, func(key string) string {
			return fmt.Sprintf(`<a href='/%s'>%s</a>`, key, key)
		})
	}
	return text
}

func extractIssueKeyFromURL(url string) string {
	prefixes := []string{"MC", "MCPE", "MCL", "REALMS", "WEB", "BDS"}
	for _, prefix := range prefixes {
		re := regexp.MustCompile(prefix + `-\d+`)
		if match := re.FindString(url); match != "" {
			return match
		}
	}
	return ""
}

func icon(name string) template.HTML {
	path := fmt.Sprintf("templates/icons/%s.svg", name)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return template.HTML(fmt.Sprintf("<!-- icon '%s' not found -->", name))
	}
	return template.HTML(bytes)
}
