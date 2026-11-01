package main

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	reCustomProp = regexp.MustCompile(`(--[A-Za-z0-9-]+)\s*:\s*([^;}]+)`)
	reHexColor   = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	reFuncColor  = regexp.MustCompile(`(?i)\b(?:rgba?|hsla?)\([^)]{3,40}\)`)
	reFontFamily = regexp.MustCompile(`(?i)font-family\s*:\s*([^;}]+)`)
	reRootBlock  = regexp.MustCompile(`(?s):root\s*\{([^}]*)\}`)
)

// extractDesign gathers inline styles plus same-origin stylesheets and pulls
// design tokens out of the combined CSS.
func extractDesign(client *http.Client, pages []pageData, base *url.URL, opt Options) DesignTokens {
	var css strings.Builder
	scanned := 0
	for _, pd := range pages {
		if pd.htmlInfo == nil {
			continue
		}
		for _, body := range pd.htmlInfo.styleBodies {
			css.WriteString(body)
			css.WriteByte('\n')
		}
	}
	seen := map[string]bool{}
	var sheetURLs []string
	for _, pd := range pages {
		if pd.htmlInfo == nil {
			continue
		}
		for _, u := range pd.htmlInfo.stylesheetURLs {
			if sameHost(base, u) && !seen[u] {
				seen[u] = true
				sheetURLs = append(sheetURLs, u)
			}
		}
	}
	if len(sheetURLs) > opt.MaxCSSFiles {
		sheetURLs = sheetURLs[:opt.MaxCSSFiles]
	}
	for _, u := range sheetURLs {
		_, data, _, err := get(client, u, opt.UserAgent)
		if err != nil {
			continue
		}
		if len(data) > maxCSSBytes {
			data = data[:maxCSSBytes]
		}
		css.Write(data)
		css.WriteByte('\n')
		scanned++
	}

	text := css.String()
	dt := DesignTokens{Stylesheets: scanned}

	props := map[string]string{}
	for _, block := range reRootBlock.FindAllStringSubmatch(text, -1) {
		for _, m := range reCustomProp.FindAllStringSubmatch(block[1], -1) {
			name := strings.TrimSpace(m[1])
			if _, ok := props[name]; !ok {
				props[name] = strings.TrimSpace(m[2])
			}
		}
	}
	if len(props) > 0 {
		dt.CustomProperties = props
	}

	colors := map[string]int{}
	for _, m := range reHexColor.FindAllString(text, -1) {
		colors[strings.ToLower(m)]++
	}
	for _, m := range reFuncColor.FindAllString(text, -1) {
		colors[strings.ToLower(strings.Join(strings.Fields(m), " "))]++
	}
	dt.TopColors = topCounts(colors, 12)

	fonts := map[string]int{}
	for _, m := range reFontFamily.FindAllStringSubmatch(text, -1) {
		for _, fam := range strings.Split(m[1], ",") {
			fam = strings.Trim(strings.TrimSpace(fam), `"'`)
			if fam != "" {
				fonts[fam]++
			}
		}
	}
	dt.TopFonts = topCounts(fonts, 10)
	return dt
}

func topCounts(m map[string]int, limit int) []TokenCount {
	out := make([]TokenCount, 0, len(m))
	for k, v := range m {
		out = append(out, TokenCount{Token: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Token < out[j].Token
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
