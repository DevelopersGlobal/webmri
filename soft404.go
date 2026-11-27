package main

import (
	"fmt"
	"regexp"
	"strings"
)

// softNotFoundTitle matches page titles that strongly suggest an error page:
// "not found", a standalone 404, or "page does not exist"/"cannot be found".
var softNotFoundTitle = regexp.MustCompile(`(?i)\bnot found\b|\b404\b|\bpage (?:does not|doesn't|did not|didn't) exist\b|\b(?:cannot|can't|could not|couldn't) be found\b`)

// softNotFoundNote returns an inferred note when a 2xx page's title reads like
// an error page, or "" when it does not. Some sites answer missing pages with
// 200; the title is only a hint, so the note never claims the page is missing.
func softNotFoundNote(status int, title string) string {
	if status < 200 || status >= 300 {
		return ""
	}
	t := strings.TrimSpace(title)
	if t == "" || !softNotFoundTitle.MatchString(t) {
		return ""
	}
	return fmt.Sprintf("inferred: possible soft 404, status %d but title %q reads like an error page", status, t)
}
