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

func RenderADF(adf string, issue *Issue) template.HTML {
	if adf == "" {
		return ""
	}
	var node map[string]any
	if err := json.Unmarshal([]byte(adf), &node); err != nil {
		return template.HTML(template.HTMLEscapeString(adf))
	}
	return template.HTML(renderADFNode(node, issue))
}

func renderADFNode(node map[string]any, issue *Issue) string {
	typeStr, _ := node["type"].(string)
	switch typeStr {
	case "doc":
		return renderADFChildren(node, issue)
	case "paragraph":
		return "<p>" + renderADFChildren(node, issue) + "</p>"
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
		return fmt.Sprintf("<h%d>%s</h%d>", lvl, renderADFChildren(node, issue), lvl)
	case "blockquote":
		return "<blockquote>" + renderADFChildren(node, issue) + "</blockquote>"
	case "bulletList":
		return "<ul>" + renderADFChildren(node, issue) + "</ul>"
	case "orderedList":
		return "<ol>" + renderADFChildren(node, issue) + "</ol>"
	case "listItem":
		return "<li>" + renderADFChildren(node, issue) + "</li>"
	case "codeBlock":
		text := extractPlainTextFromADFChildren(node)
		lang := "mcfunction"
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if p, ok := attrs["language"].(string); ok {
				lang = p
			}
		}

		// Return both versions wrapped in theme-specific divs
		var lightCode = highlightCodeBlock(text, lang, "catppuccin-latte") // abap
		var darkCode = highlightCodeBlock(text, lang, "catppuccin-mocha")  // onedark
		return fmt.Sprintf("<div class='code-light'>%s</div><div class='code-dark'>%s</div>", lightCode, darkCode)
	case "rule":
		return "<hr>"
	case "panel":
		panelType := "info"
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if p, ok := attrs["panelType"].(string); ok {
				panelType = p
			}
		}
		return fmt.Sprintf("<div class='panel panel-%s'><img src='/static/icons/%s.svg' alt=''><div>", panelType, panelType) + renderADFChildren(node, issue) + "</div></div>"
	case "table":
		return "<table>" + renderADFChildren(node, issue) + "</table>"
	case "tableRow":
		return "<tr>" + renderADFChildren(node, issue) + "</tr>"
	case "tableCell":
		return "<td>" + renderADFChildren(node, issue) + "</td>"
	case "tableHeader":
		return "<th>" + renderADFChildren(node, issue) + "</th>"
	case "text":
		text, _ := node["text"].(string)
		text = template.HTMLEscapeString(text)
		hasLink := false
		if marks, ok := node["marks"].([]any); ok {
			for _, m := range marks {
				if mark, ok := m.(map[string]any); ok {
					typeMark, _ := mark["type"].(string)
					if typeMark == "link" {
						hasLink = true
					}
				}
			}
		}
		if !hasLink {
			text = linkifyIssueKeys(text)
		}
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
		return fmt.Sprintf("<div class='media-single'>%s</div>", renderADFChildren(node, issue))
	case "mediaGroup":
		return fmt.Sprintf("<div class='media-group'>%s</div>", renderADFChildren(node, issue))
	case "media":
		if attrs, ok := node["attrs"].(map[string]any); ok {
			typeStr, _ := attrs["type"].(string)
			switch typeStr {
			case "file":
				if alt, ok := attrs["alt"].(string); ok {
					if issue != nil {
						for _, att := range issue.Attachments {
							if att.Filename == alt {
								width, _ := attrs["width"].(float64)
								height, _ := attrs["height"].(float64)
								if att.IsImage() {
									return fmt.Sprintf("<img class='media' src='%s' alt='%s' width='%.0f' height='%.0f'>", att.GetUrl(), template.HTMLEscapeString(alt), width, height)
								} else if att.IsVideo() {
									return fmt.Sprintf("<video class='media' src='%s' alt='%s' width='%.0f' height='%.0f' controls></video>", att.GetUrl(), template.HTMLEscapeString(alt), width, height)
								}
							}
						}
					}
					return fmt.Sprintf("<span class='placeholder'>[media: %s]</span>", template.HTMLEscapeString(alt))
				}
			default:
				return "<span class='placeholder'>[media]</span>"
			}
		}
		return "[media]"
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
		return fmt.Sprintf("<span style='color:red;font-weight:bold;'>[%s]</span>", template.HTMLEscapeString(typeStr)) + renderADFChildren(node, issue)
	}
}

func highlightCodeBlock(text string, lang string, style string) string {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	formatter := html.New(
		html.PreventSurroundingPre(true),
		html.TabWidth(4),
	)

	iterator, err := lexer.Tokenise(nil, text)
	if err != nil {
		log.Printf("[WARNING] Error during code tokenizing: %s", err)
		return fmt.Sprintf("<pre><code>%s</code></pre>", text)
	}

	var buffer bytes.Buffer
	err = formatter.Format(&buffer, styles.Get(style), iterator)
	if err != nil {
		log.Printf("[WARNING] Error during code highlighting: %s", err)
		return fmt.Sprintf("<pre><code>%s</code></pre>", text)
	}

	return fmt.Sprintf("<pre><code lang=\"%s\">%s</code></pre>", lang, buffer.String())
}

func renderADFChildren(node map[string]any, issue *Issue) string {
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, c := range content {
		if child, ok := c.(map[string]any); ok {
			sb.WriteString(renderADFNode(child, issue))
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

func IsOnlyMediaADF(adf string) bool {
	var node map[string]any
	err := json.Unmarshal([]byte(adf), &node)
	if err != nil {
		return false
	}
	content, ok := node["content"].([]any)
	if !ok {
		return false
	}
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			return false
		}
		switch block["type"] {
		case "media", "mediaSingle", "mediaGroup":
			continue
		default:
			return false
		}
	}
	return true
}
