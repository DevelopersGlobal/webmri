package main

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	version       = "0.1.0"
	maxBodyBytes  = 4 << 20
	maxCSSBytes   = 1 << 20
	defaultUA     = "webmri/0.1 (+https://github.com/DevelopersGlobal/webmri)"
	defaultMaxCSS = 8
)

// Options tunes one scan.
type Options struct {
	MaxPages    int
	Concurrency int
	Timeout     time.Duration
	NoCrawl     bool
	UserAgent   string
	MaxCSSFiles int
}

func (o Options) withDefaults() Options {
	if o.MaxPages <= 0 {
		o.MaxPages = 25
	}
	if o.MaxPages > 500 {
		o.MaxPages = 500
	}
	if o.Concurrency <= 0 {
		o.Concurrency = 8
	}
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Second
	}
	if o.UserAgent == "" {
		o.UserAgent = defaultUA
	}
	if o.MaxCSSFiles <= 0 {
		o.MaxCSSFiles = defaultMaxCSS
	}
	return o
}

// normalizeURL parses target and adds https:// when no scheme is given.
func normalizeURL(target string) (*url.URL, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errUsage("empty target URL")
	}
	if !strings.Contains(target, "://") {
		target = "https://" + target
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errUsage("unsupported scheme %q (http or https only)", u.Scheme)
	}
	if u.Host == "" {
		return nil, errUsage("missing host in %q", target)
	}
	u.Fragment = ""
	return u, nil
}

type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func errUsage(format string, args ...interface{}) error {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

// sameHost reports whether link is on the same host as base, ignoring a
// leading "www." on either side.
func sameHost(base *url.URL, link string) bool {
	l, err := url.Parse(link)
	if err != nil {
		return false
	}
	strip := func(h string) string {
		return strings.TrimPrefix(strings.ToLower(h), "www.")
	}
	return strip(l.Hostname()) == strip(base.Hostname())
}

// nonPageExts are URL path suffixes that never render as pages, so the
// crawler does not spend its page budget on them.
var nonPageExts = map[string]bool{
	".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".tgz": true,
	".rar": true, ".7z": true, ".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".svg": true, ".ico": true, ".avif": true,
	".mp4": true, ".webm": true, ".mov": true, ".mp3": true, ".wav": true,
	".ogg": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true,
	".eot": true, ".css": true, ".js": true, ".mjs": true, ".map": true,
	".wasm": true, ".exe": true, ".dmg": true, ".apk": true,
}

// pageLike reports whether a URL is worth crawling as a page.
func pageLike(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(u.Path))
	return !nonPageExts[ext]
}

// cleanLink resolves ref against base and drops the fragment. Empty on error.
func cleanLink(base *url.URL, ref string) string {
	if ref == "" || strings.HasPrefix(ref, "mailto:") || strings.HasPrefix(ref, "tel:") || strings.HasPrefix(ref, "javascript:") {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

func newClient(opt Options) *http.Client {
	return &http.Client{
		Timeout: opt.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: opt.Concurrency,
			IdleConnTimeout:     30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			req.Header.Set("User-Agent", opt.UserAgent)
			return nil
		},
	}
}

func get(client *http.Client, raw, ua string) (*http.Response, []byte, int64, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, nil, 0, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/css,*/*;q=0.5")
	// Advertise gzip only. Setting Accept-Encoding explicitly disables the
	// transport's transparent decompression, so decoding below is
	// deterministic: gzip is decoded, anything else becomes a visible
	// error instead of silently stored compressed bytes.
	req.Header.Set("Accept-Encoding", "gzip")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()
	data, err := decodeBody(resp, maxBodyBytes)
	return resp, data, time.Since(start).Milliseconds(), err
}

// decodeBody reads up to limit bytes of the response's decoded content.
// Only gzip is supported, matching the Accept-Encoding header sent in
// get; any other Content-Encoding is an explicit error so a compressed
// page is never silently misreported as empty or unparseable.
func decodeBody(resp *http.Response, limit int64) ([]byte, error) {
	switch enc := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))); enc {
	case "", "identity":
		return io.ReadAll(io.LimitReader(resp.Body, limit))
	case "gzip", "x-gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("invalid gzip response: %w", err)
		}
		defer gz.Close()
		return io.ReadAll(io.LimitReader(gz, limit))
	default:
		return nil, fmt.Errorf("unsupported Content-Encoding %q (webmri only requests gzip)", enc)
	}
}

// fetchSitemapURLs discovers sitemap locations from robots.txt and the
// default /sitemap.xml, then returns page URLs (one level of index nesting).
func fetchSitemapURLs(client *http.Client, base *url.URL, opt Options) []string {
	root := &url.URL{Scheme: base.Scheme, Host: base.Host}
	candidates := []string{root.String() + "/sitemap.xml"}
	if _, data, _, err := get(client, root.String()+"/robots.txt", opt.UserAgent); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
				loc := strings.TrimSpace(line[len("sitemap:"):])
				if loc != "" {
					candidates = append([]string{loc}, candidates...)
				}
			}
		}
	}
	seen := map[string]bool{}
	var out []string
	var parse func(smURL string, depth int)
	parse = func(smURL string, depth int) {
		if depth > 1 || seen[smURL] {
			return
		}
		seen[smURL] = true
		_, data, _, err := get(client, smURL, opt.UserAgent)
		if err != nil {
			return
		}
		var urlset struct {
			URLs []struct {
				Loc string `xml:"loc"`
			} `xml:"url"`
			Sitemaps []struct {
				Loc string `xml:"loc"`
			} `xml:"sitemap"`
		}
		if xml.Unmarshal(data, &urlset) != nil {
			return
		}
		for _, entry := range urlset.URLs {
			loc := strings.TrimSpace(entry.Loc)
			if loc != "" && sameHost(base, loc) {
				out = append(out, loc)
			}
		}
		for _, child := range urlset.Sitemaps {
			parse(strings.TrimSpace(child.Loc), depth+1)
		}
	}
	for _, c := range candidates {
		parse(c, 0)
	}
	return out
}

// pageData is one fetched page plus its parsed HTML facts.
type pageData struct {
	PageResult
	headers  http.Header
	body     []byte
	htmlInfo *htmlInfo
}

// fetchAll fetches urls with bounded concurrency, preserving order.
func fetchAll(client *http.Client, urls []string, opt Options) []pageData {
	sem := make(chan struct{}, opt.Concurrency)
	var wg sync.WaitGroup
	out := make([]pageData, len(urls))
	for i, raw := range urls {
		wg.Add(1)
		go func(i int, raw string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = fetchPage(client, raw, opt)
		}(i, raw)
	}
	wg.Wait()
	return out
}

func fetchPage(client *http.Client, raw string, opt Options) pageData {
	pd := pageData{PageResult: PageResult{URL: raw}}
	resp, data, ms, err := get(client, raw, opt.UserAgent)
	pd.DurationMS = ms
	if err != nil {
		pd.Error = err.Error()
		return pd
	}
	pd.Status = resp.StatusCode
	pd.Bytes = len(data)
	pd.headers = resp.Header
	pd.body = data
	finalURL := resp.Request.URL
	pd.URL = finalURL.String()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && isHTML(resp.Header, data) {
		info := parseHTML(data, finalURL, pd.URL)
		pd.Title = info.title
		pd.htmlInfo = info
	}
	return pd
}

func isHTML(h http.Header, data []byte) bool {
	ct := strings.ToLower(h.Get("Content-Type"))
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return true
	}
	if ct != "" && !strings.Contains(ct, "text/") {
		return false
	}
	return bytes.Contains(bytes.ToLower(data[:min(len(data), 512)]), []byte("<html"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Scan runs the full anatomy scan of target and returns the report.
func Scan(target string, opt Options) (*Report, error) {
	opt = opt.withDefaults()
	start := time.Now()
	base, err := normalizeURL(target)
	if err != nil {
		return nil, err
	}
	client := newClient(opt)

	rep := &Report{
		Tool:      "webmri",
		Version:   version,
		Target:    base.String(),
		ScannedAt: start.UTC().Format(time.RFC3339),
		Notes: []string{
			"Static analysis of public pages only: no JavaScript is executed, no logins, no private APIs.",
			"Stack fingerprints are heuristic; 'observed' means the pattern was directly seen in a response, 'inferred' means a weaker pattern suggested it.",
			"Evidence entries name the URL and the exact signal behind every claim.",
		},
	}

	seeds := []string{base.String()}
	if !opt.NoCrawl {
		for _, s := range fetchSitemapURLs(client, base, opt) {
			if pageLike(s) {
				seeds = append(seeds, s)
			}
		}
	}
	visited := map[string]bool{}
	var queue []string
	for _, s := range dedupeStrings(seeds) {
		if !visited[s] {
			visited[s] = true
			queue = append(queue, s)
		}
	}
	var pages []pageData

	for len(pages) < opt.MaxPages && len(queue) > 0 {
		var batch []string
		for len(queue) > 0 && len(pages)+len(batch) < opt.MaxPages {
			batch = append(batch, queue[0])
			queue = queue[1:]
		}
		if len(batch) == 0 {
			break
		}
		for _, pd := range fetchAll(client, batch, opt) {
			pages = append(pages, pd)
			if opt.NoCrawl || pd.htmlInfo == nil {
				continue
			}
			for _, link := range pd.htmlInfo.links {
				if sameHost(base, link) && pageLike(link) && !visited[link] {
					visited[link] = true
					queue = append(queue, link)
				}
			}
		}
	}

	rep.Pages = make([]PageResult, 0, len(pages))
	routeSet := map[string]bool{}
	for _, pd := range pages {
		rep.Pages = append(rep.Pages, pd.PageResult)
		if pd.Error == "" {
			routeSet[pd.URL] = true
		}
	}
	rep.Routes = sortedKeys(routeSet)

	rep.Stack = detectStack(pages, base)
	rep.Forms = collectForms(pages)
	rep.APIs = detectAPISurface(pages, base)
	rep.Design = extractDesign(client, pages, base, opt)
	rep.WallMS = time.Since(start).Milliseconds()
	return rep, nil
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectForms(pages []pageData) []FormInfo {
	var out []FormInfo
	for _, pd := range pages {
		if pd.htmlInfo != nil {
			out = append(out, pd.htmlInfo.forms...)
		}
	}
	return out
}
