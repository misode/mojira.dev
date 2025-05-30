package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func indexHandler(service *IssueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issues, err := service.db.GetAllIssues(100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := map[string]interface{}{"Issues": issues}
		tmpl := template.Must(template.New("layout.html").Funcs(template.FuncMap{
			"FormatTime": FormatTime,
		}).ParseFiles("views/layout.html", "views/index.html"))
		tmpl.ExecuteTemplate(w, "base", data)
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
		tmpl := template.Must(template.New("layout.html").Funcs(template.FuncMap{
			"FormatTime": FormatTime,
			"RenderADF":  RenderADF,
		}).ParseFiles("views/layout.html", "views/issue.html"))
		err = tmpl.ExecuteTemplate(w, "base", map[string]interface{}{"Issue": issue})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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
		tmpl := template.Must(template.New("layout.html").ParseFiles("views/layout.html", "views/sync.html"))
		tmpl.ExecuteTemplate(w, "base", map[string]interface{}{"Stats": stats})
	}
}

func FormatTime(t interface{}) string {
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

func RenderADF(adf string) template.HTML {
	if adf == "" {
		return ""
	}
	var node map[string]interface{}
	if err := json.Unmarshal([]byte(adf), &node); err != nil {
		return template.HTML(template.HTMLEscapeString(adf))
	}
	return template.HTML(renderADFNode(node))
}

func renderADFNode(node map[string]interface{}) string {
	typeStr, _ := node["type"].(string)
	switch typeStr {
	case "doc":
		return renderADFChildren(node)
	case "paragraph":
		return "<p>" + renderADFChildren(node) + "</p>"
	case "heading":
		lvl := 1
		if attrs, ok := node["attrs"].(map[string]interface{}); ok {
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
		lang := ""
		if attrs, ok := node["attrs"].(map[string]interface{}); ok {
			if l, ok := attrs["language"].(string); ok {
				lang = l
			}
		}
		return fmt.Sprintf("<pre><code class='lang-%s'>%s</code></pre>", template.HTMLEscapeString(lang), renderADFChildren(node))
	case "rule":
		return "<hr/>"
	case "panel":
		return "<div class='panel'>" + renderADFChildren(node) + "</div>"
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
		if marks, ok := node["marks"].([]interface{}); ok {
			for _, m := range marks {
				if mark, ok := m.(map[string]interface{}); ok {
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
					case "strike":
						text = "<s>" + text + "</s>"
					case "link":
						if attrs, ok := mark["attrs"].(map[string]interface{}); ok {
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
		if attrs, ok := node["attrs"].(map[string]interface{}); ok {
			if txt, ok := attrs["text"].(string); ok && len([]rune(txt)) == 1 {
				return template.HTMLEscapeString(txt)
			}
			if short, ok := attrs["shortName"].(string); ok {
				return template.HTMLEscapeString(short)
			}
			if txt, ok := attrs["text"].(string); ok {
				return template.HTMLEscapeString(txt)
			}
		}
		return "[emoji]"
	case "mention":
		if attrs, ok := node["attrs"].(map[string]interface{}); ok {
			if text, ok := attrs["text"].(string); ok {
				return template.HTMLEscapeString(text)
			}
		}
		return "@mention"
	case "mediaSingle":
		return renderADFChildren(node)
	case "mediaGroup":
		return renderADFChildren(node)
	case "media":
		return "<span style='color:#888;'>[media]</span>"
	case "inlineCard":
		if attrs, ok := node["attrs"].(map[string]interface{}); ok {
			if url, ok := attrs["url"].(string); ok {
				if key := extractIssueKeyFromURL(url); key != "" {
					return fmt.Sprintf(`<a href='/%s'>%s</a>`, key, key)
				}
				return fmt.Sprintf(`<a href='%s' target='_blank'>%s</a>`, template.HTMLEscapeString(url), template.HTMLEscapeString(url))
			}
		}
		return "[inlineCard]"
	default:
		return fmt.Sprintf("<span style='color:red;font-weight:bold;'>[%s]</span>"+renderADFChildren(node), template.HTMLEscapeString(typeStr))
	}
}

func renderADFChildren(node map[string]interface{}) string {
	content, ok := node["content"].([]interface{})
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, c := range content {
		if child, ok := c.(map[string]interface{}); ok {
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
