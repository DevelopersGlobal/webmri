package main

import (
	"strings"
	"testing"
)

// countUnescapedPipes counts table column separators: every | that a backslash
// does not escape.
func countUnescapedPipes(s string) int {
	n := 0
	for i, r := range s {
		if r == '|' && (i == 0 || rune(s[i-1]) != '\\') {
			n++
		}
	}
	return n
}

// Free text that lands in a table cell — a page whose title or URL carries a
// pipe, an evidence detail quoting a header value — must not add columns or
// rows to the pipe-delimited tables (#12).
func TestWriteMarkdownEscapesTableCells(t *testing.T) {
	rep := &Report{
		Tool: "webmri", Version: "test", Target: "https://example.com",
		Pages: []PageResult{{
			URL: "https://example.com/a|b?next=/c|d", Title: "a | b",
			Status: 200, Images: 3, ImagesMissingAlt: 1, HasLang: true, HasH1: true,
		}},
		Stack: []Finding{{
			Name: "nginx", Category: "web server", Confidence: "observed",
			Evidence: []Evidence{{
				URL:    "https://example.com/a|b",
				Detail: "header server: nginx | cdn\r\nsecond line",
			}},
		}},
	}
	md := WriteMarkdown(rep)

	wantRow := "| https://example.com/a\\|b?next=/c\\|d | 3 | 1 | 0 | true | true |"
	if !strings.Contains(md, wantRow) {
		t.Errorf("accessibility row is not one intact escaped row, want:\n%s\nin:\n%s", wantRow, md)
	}
	if !strings.Contains(md, "header server: nginx \\| cdn second line") {
		t.Errorf("evidence detail pipe not escaped or newline not collapsed:\n%s", md)
	}
	for _, line := range strings.Split(md, "\n") {
		if !strings.HasPrefix(line, "| https://") {
			continue
		}
		// Accessibility rows have 7 pipes, stack rows 5, asset rows 4; either
		// way the unescaped pipes must match the columns the row was written
		// with, never the ones the page text smuggled in.
		if n := countUnescapedPipes(line); n != 7 && n != 5 && n != 4 {
			t.Errorf("row carries %d unescaped pipes, want 7 (accessibility), 5 (stack) or 4 (assets): %q", n, line)
		}
	}
}

// A page titled a | b with ordinary URLs and evidence still renders the same
// tables as before: escaping only changes cells that need it.
func TestWriteMarkdownPlainRowsUnchanged(t *testing.T) {
	rep := &Report{
		Tool: "webmri", Version: "test", Target: "https://example.com",
		Pages: []PageResult{{
			URL: "https://example.com/", Title: "a | b",
			Status: 200, HasLang: true,
		}},
	}
	md := WriteMarkdown(rep)
	if got := "| https://example.com/ | 0 | 0 | 0 | true | false |"; !strings.Contains(md, got) {
		t.Errorf("plain accessibility row changed, want:\n%s\nin:\n%s", got, md)
	}
}

// A page's <meta name="description"> reaches both outputs (#9); a page
// without one adds nothing, so its report renders exactly as before.
func TestPageDescription(t *testing.T) {
	plain := PageResult{URL: "https://example.com/", Status: 200, HasLang: true}
	described := PageResult{URL: "https://example.com/about", Status: 200, Description: "Who we are | what we do\nand why"}

	without := &Report{Tool: "webmri", Version: "test", Target: "https://example.com", Pages: []PageResult{plain}}
	md := WriteMarkdown(without)
	if strings.Contains(md, "## Page descriptions") || strings.Contains(md, "meta description") {
		t.Errorf("report without descriptions gained a section:\n%s", md)
	}
	data, err := WriteJSON(without)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"description"`) {
		t.Errorf("json without descriptions carries the field:\n%s", data)
	}

	with := &Report{Tool: "webmri", Version: "test", Target: "https://example.com", Pages: []PageResult{plain, described}}
	md = WriteMarkdown(with)
	want := "## Page descriptions\n\n| page | meta description |\n| --- | --- |\n| https://example.com/about | Who we are \\| what we do and why |\n\n"
	if !strings.Contains(md, want) {
		t.Errorf("description section missing or wrong, want:\n%s\nin:\n%s", want, md)
	}
	if strings.Contains(md, "| https://example.com/ | |") {
		t.Errorf("page without a description got a row:\n%s", md)
	}
	data, err = WriteJSON(with)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"description": "Who we are | what we do\nand why"`) {
		t.Errorf("json missing description:\n%s", data)
	}
	if strings.Count(string(data), `"description"`) != 1 {
		t.Errorf("description should appear only on the page that has one:\n%s", data)
	}
}

// Theme color and Open Graph values show in the design section with the page
// they came from, and are left out of both outputs when there are none (#5).
func TestDesignMetaOutput(t *testing.T) {
	without := &Report{Tool: "webmri", Version: "test", Target: "https://example.com"}
	if md := WriteMarkdown(without); strings.Contains(md, "Open Graph") {
		t.Errorf("report without design meta gained a list:\n%s", md)
	}
	data, err := WriteJSON(without)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"meta"`) {
		t.Errorf("json without design meta carries the field:\n%s", data)
	}

	with := &Report{Tool: "webmri", Version: "test", Target: "https://example.com", Design: DesignTokens{Meta: []MetaToken{
		{Property: "theme-color", Value: "#0a0a0a", Page: "https://example.com/"},
		{Property: "og:title", Value: "Docs\nhome", Page: "https://example.com/docs"},
	}}}
	md := WriteMarkdown(with)
	want := "Theme color and Open Graph metadata:\n\n- theme-color: #0a0a0a — https://example.com/\n- og:title: Docs home — https://example.com/docs\n\n"
	if !strings.Contains(md, want) {
		t.Errorf("design meta list missing or wrong, want:\n%s\nin:\n%s", want, md)
	}
	if strings.Index(md, "## Design tokens") > strings.Index(md, want) || strings.Index(md, want) > strings.Index(md, "## Routes") {
		t.Errorf("design meta should sit in the design section:\n%s", md)
	}
	data, err = WriteJSON(with)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"meta": [`, `"property": "theme-color"`, `"value": "#0a0a0a"`, `"page": "https://example.com/docs"`} {
		if !strings.Contains(string(data), s) {
			t.Errorf("json missing %s:\n%s", s, data)
		}
	}
}
