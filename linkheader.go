package main

import (
	"net/http"
	"net/url"
	"strings"
)

// linkHeaderSource names the HTTP Link response header as evidence.
const linkHeaderSource = "Link header"

// wpAPIRel is the link relation WordPress uses to announce its REST API.
const wpAPIRel = "https://api.w.org/"

// linkEntry is one link-value from an HTTP Link header (RFC 8288).
type linkEntry struct {
	target string   // resolved absolute URL
	rels   []string // lowercased relation types
	typ    string   // media type from type=, lowercased, without parameters
}

func (e linkEntry) hasRel(want string) bool {
	for _, r := range e.rels {
		if r == want {
			return true
		}
	}
	return false
}

// parseLinkHeader parses every Link header value into entries, resolving
// targets against base. Malformed link-values are skipped.
func parseLinkHeader(h http.Header, base *url.URL) []linkEntry {
	var out []linkEntry
	for _, value := range h.Values("Link") {
		rest := value
		for {
			open := strings.IndexByte(rest, '<')
			if open < 0 {
				break
			}
			end := strings.IndexByte(rest[open:], '>')
			if end < 0 {
				break
			}
			target := strings.TrimSpace(rest[open+1 : open+end])
			params, next := splitLinkParams(rest[open+end+1:])
			rest = next
			if target == "" {
				continue
			}
			u, err := base.Parse(target)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				continue
			}
			u.Fragment = ""
			e := linkEntry{target: u.String()}
			for _, p := range params {
				name, val, _ := strings.Cut(p, "=")
				val = strings.Trim(strings.TrimSpace(val), `"`)
				switch strings.ToLower(strings.TrimSpace(name)) {
				case "rel":
					e.rels = strings.Fields(strings.ToLower(val))
				case "type":
					e.typ = strings.ToLower(strings.TrimSpace(strings.Split(val, ";")[0]))
				}
			}
			out = append(out, e)
		}
	}
	return out
}

// splitLinkParams reads the ;-separated parameters after a link target up to
// the next comma outside quotes, and returns them with the remaining input.
func splitLinkParams(s string) ([]string, string) {
	var params []string
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case ';', ',':
			if inQuote {
				continue
			}
			if p := strings.TrimSpace(s[start:i]); p != "" {
				params = append(params, p)
			}
			if s[i] == ',' {
				return params, s[i+1:]
			}
			start = i + 1
		}
	}
	if p := strings.TrimSpace(s[start:]); p != "" {
		params = append(params, p)
	}
	return params, ""
}

// linkHeaderEntries returns the Link header entries of a successfully
// fetched page.
func linkHeaderEntries(pd pageData) []linkEntry {
	if pd.Error != "" || pd.Status < 200 || pd.Status >= 300 || pd.headers == nil {
		return nil
	}
	base, err := url.Parse(pd.URL)
	if err != nil {
		return nil
	}
	return parseLinkHeader(pd.headers, base)
}

func isFeedType(t string) bool {
	return t == "application/rss+xml" || t == "application/atom+xml"
}
