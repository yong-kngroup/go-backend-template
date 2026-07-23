package sitegen

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type TOCEntry struct {
	Level int
	ID    string
	Text  string
}

type RenderedMarkdown struct {
	HTML           template.HTML
	TOC            []TOCEntry
	ReadingMinutes int
}

type MarkdownRenderer struct {
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{
		markdown: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		),
		policy: articleHTMLPolicy(),
	}
}

func (r *MarkdownRenderer) Render(source string) (RenderedMarkdown, error) {
	sourceBytes := []byte(source)
	root := r.markdown.Parser().Parse(text.NewReader(sourceBytes))
	toc := make([]TOCEntry, 0)
	if err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Link:
			if !isAllowedLinkURL(string(n.Destination)) {
				return ast.WalkStop, fmt.Errorf("Markdown link URL %q uses an unsupported scheme", n.Destination)
			}
		case *ast.Image:
			if !isAllowedImageURL(string(n.Destination)) {
				return ast.WalkStop, fmt.Errorf("Markdown image URL %q must be an absolute HTTP(S) URL or a site-root path", n.Destination)
			}
		case *ast.Heading:
			if n.Level < 2 || n.Level > 4 {
				return ast.WalkContinue, nil
			}
			id, ok := n.AttributeString("id")
			if !ok {
				return ast.WalkStop, fmt.Errorf("Markdown heading is missing an ID")
			}
			idBytes, ok := id.([]byte)
			if !ok {
				return ast.WalkStop, fmt.Errorf("Markdown heading has an invalid ID")
			}
			toc = append(toc, TOCEntry{Level: n.Level, ID: string(idBytes), Text: normalizeText(string(n.Text(sourceBytes)))})
		}
		return ast.WalkContinue, nil
	}); err != nil {
		return RenderedMarkdown{}, err
	}

	var rendered bytes.Buffer
	if err := r.markdown.Convert(sourceBytes, &rendered); err != nil {
		return RenderedMarkdown{}, fmt.Errorf("render Markdown: %w", err)
	}
	safeHTML := r.policy.SanitizeBytes(rendered.Bytes())
	return RenderedMarkdown{
		HTML:           template.HTML(safeHTML), // Sanitized by the fixed article policy above.
		TOC:            toc,
		ReadingMinutes: readingMinutes(source),
	}, nil
}

func articleHTMLPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "del", "blockquote", "hr", "ul", "ol", "li", "pre", "code", "table", "thead", "tbody", "tr", "th", "td")
	p.AllowElements("h2", "h3", "h4", "a", "img", "input")
	p.AllowAttrs("id").OnElements("h2", "h3", "h4")
	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[a-zA-Z0-9+_-]+$`)).OnElements("code")
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)
	p.RequireNoReferrerOnLinks(true)
	return p
}

func isAllowedImageURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Scheme == "" && u.Host != "" {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return !strings.HasPrefix(value, "//")
	}
	return u.IsAbs() && (u.Scheme == "https" || u.Scheme == "http")
}

func isAllowedLinkURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (u.Scheme == "" && u.Host != "") {
		return false
	}
	if u.Scheme == "" {
		return true
	}
	return u.Scheme == "https" || u.Scheme == "http" || u.Scheme == "mailto"
}

func readingMinutes(source string) int {
	chineseChars, words := 0, 0
	inWord := false
	for _, r := range source {
		if unicode.Is(unicode.Han, r) {
			chineseChars++
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				words++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	minutesByChinese := (chineseChars + 399) / 400
	minutesByWords := (words + 199) / 200
	if minutesByChinese > minutesByWords {
		return max(1, minutesByChinese)
	}
	return max(1, minutesByWords)
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func summaryDescription(value string) string {
	value = normalizeText(value)
	runes := []rune(value)
	if len(runes) <= 160 {
		return value
	}
	return string(runes[:157]) + "..."
}
