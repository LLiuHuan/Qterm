package webfetch

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

var blockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true, "br": true,
	"dd": true, "div": true, "dl": true, "dt": true, "figcaption": true, "figure": true,
	"footer": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "hr": true, "li": true, "main": true, "nav": true, "ol": true,
	"p": true, "pre": true, "section": true, "table": true, "tbody": true, "td": true,
	"th": true, "thead": true, "tr": true, "ul": true,
}

var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true, "svg": true, "iframe": true,
}

func extractText(contentType string, data []byte) (string, string) {
	if strings.Contains(contentType, "html") || (contentType == "" && looksLikeHTML(data)) {
		text, title := htmlToText(data)
		return text, title
	}
	return normalizePlainText(string(data)), ""
}

func looksLikeHTML(data []byte) bool {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	lowered := strings.ToLower(strings.TrimSpace(string(head)))
	return strings.HasPrefix(lowered, "<!doctype html") || strings.HasPrefix(lowered, "<html")
}

func htmlToText(data []byte) (string, string) {
	tokenizer := html.NewTokenizer(bytes.NewReader(data))
	var builder strings.Builder
	var title strings.Builder
	skipDepth := 0
	inTitle := false
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {

			break
		}
		token := tokenizer.Token()
		switch tokenType {
		case html.StartTagToken:
			name := strings.ToLower(token.Data)
			if skipTags[name] {
				skipDepth++
				continue
			}
			if name == "title" {
				inTitle = true
			}
			if blockTags[name] {
				builder.WriteString("\n")
			}
		case html.EndTagToken:
			name := strings.ToLower(token.Data)
			if skipTags[name] {
				if skipDepth > 0 {
					skipDepth--
				}
				continue
			}
			if name == "title" {
				inTitle = false
			}
			if blockTags[name] {
				builder.WriteString("\n")
			}
		case html.TextToken:
			if skipDepth > 0 {
				continue
			}

			if inTitle {
				title.WriteString(token.Data)
				continue
			}
			builder.WriteString(token.Data)
		}
	}
	return normalizePlainText(builder.String()), strings.TrimSpace(title.String())
}

func normalizePlainText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	output := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		collapsed := strings.Join(strings.Fields(line), " ")
		if collapsed == "" {
			if blank {
				continue
			}
			blank = true
			output = append(output, "")
			continue
		}
		blank = false
		output = append(output, collapsed)
	}
	return strings.TrimSpace(strings.Join(output, "\n"))
}
