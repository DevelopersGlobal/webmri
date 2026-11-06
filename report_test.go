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
		// Accessibility rows have 7 columns, stack rows 5; either way the
		// unescaped pipes must match the columns the row was written with,
		// never the ones the page text smuggled in.
		if n := countUnescapedPipes(line); n != 7 && n != 5 {
			t.Errorf("row carries %d unescaped pipes, want 7 (accessibility) or 5 (stack): %q", n, line)
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
