package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/kyokomi/emoji/v2"
)

func RenderADF(adf string) template.HTML {
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
		text := extractPlainTextFromADFChildren(node)
		lang := "mcfunction"
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if p, ok := attrs["language"].(string); ok {
				lang = p
			}
		}
		lexer := lexers.Get(lang)
		if lexer == nil {
			lexer = lexers.Fallback
		}
		iterator, err := lexer.Tokenise(nil, text)
		if err != nil {
			log.Printf("[WARNING] Error during code tokenizing: %s", err)
			return fmt.Sprintf("<pre><code>%s</code></pre>", text)
		}
		formatter := html.New(
			html.PreventSurroundingPre(true),
			html.TabWidth(4),
		)
		var buf bytes.Buffer
		err = formatter.Format(&buf, styles.Get("vs"), iterator)
		if err != nil {
			log.Printf("[WARNING] Error during code highlighting: %s", err)
			return fmt.Sprintf("<pre><code>%s</code></pre>", text)
		}
		return fmt.Sprintf("<pre><code>%s</code></pre>", buf.String())
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
								if id := extractCommentIdFromURL(href); id != "" {
									text = fmt.Sprintf("<a href='%s'>%s</a>", id, text)
								} else {
									text = fmt.Sprintf("<a href='%s' rel='nofollow' target='_blank'>%s</a>", template.HTMLEscapeString(href), text)
								}
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

func extractCommentIdFromURL(url string) string {
	re := regexp.MustCompile(`#comment-(\d+)$`)
	if match := re.FindString(url); match != "" {
		return match
	}
	return ""
}

func ExtractPlainTextFromADF(adf string) string {
	if adf == "" {
		return ""
	}
	var node map[string]any
	err := json.Unmarshal([]byte(adf), &node)
	if err != nil {
		return ""
	}
	return extractPlainTextFromADFNode(node)
}

func extractPlainTextFromADFNode(node map[string]any) string {
	typeStr, _ := node["type"].(string)
	switch typeStr {
	case "text":
		text, _ := node["text"].(string)
		return text
	case "hardBreak":
		return "\n"
	case "heading":
		return extractPlainTextFromADFChildren(node) + "\n"
	case "paragraph":
		return extractPlainTextFromADFChildren(node) + "\n"
	default:
		return extractPlainTextFromADFChildren(node)
	}
}

func extractPlainTextFromADFChildren(node map[string]any) string {
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, c := range content {
		if child, ok := c.(map[string]any); ok {
			sb.WriteString(extractPlainTextFromADFNode(child))
		}
	}
	return sb.String()
}

func IsEmptyADF(adf string) bool {
	var node map[string]any
	err := json.Unmarshal([]byte(adf), &node)
	if err != nil {
		return true
	}
	content, ok := node["content"].([]any)
	if !ok {
		return true
	}
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok || block["type"] != "paragraph" {
			return false
		}
		children, ok := block["content"].([]any)
		if ok && len(children) > 0 {
			return false
		}
	}
	return true
}
