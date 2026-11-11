package main

import (
	"net/http"
	"net/url"
	"testing"
)

func pd(url string, headers map[string]string, body string) pageData {
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}
	u, _ := urlparse(url)
	return pageData{
		PageResult: PageResult{URL: url, Status: 200},
		headers:    h,
		body:       []byte(body),
		htmlInfo:   parseHTML([]byte(body), u, url),
	}
}

func urlparse(raw string) (*url.URL, error) { return url.Parse(raw) }

func findFinding(fs []Finding, name string) *Finding {
	for i := range fs {
		if fs[i].Name == name {
			return &fs[i]
		}
	}
	return nil
}

func TestDetectStackNextAndCloudflare(t *testing.T) {
	base, _ := url.Parse("https://app.test/")
	pages := []pageData{
		pd("https://app.test/", map[string]string{"cf-ray": "abc123-SIN"}, `<script id="__NEXT_DATA__" type="application/json">{}</script>`),
	}
	fs := detectStack(pages, base)
	if f := findFinding(fs, "Next.js"); f == nil || f.Confidence != "observed" {
		t.Fatalf("Next.js finding %+v", f)
	}
	if f := findFinding(fs, "Cloudflare"); f == nil {
		t.Fatal("Cloudflare not detected from cf-ray header")
	}
}

func TestDetectStackGenerator(t *testing.T) {
	base, _ := url.Parse("https://blog.test/")
	pages := []pageData{
		pd("https://blog.test/", nil, `<meta name="generator" content="Hugo 0.120">`),
	}
	fs := detectStack(pages, base)
	f := findFinding(fs, "Hugo")
	if f == nil || len(f.Evidence) == 0 || f.Evidence[0].Detail != "meta generator: Hugo 0.120" {
		t.Fatalf("Hugo finding %+v", f)
	}
}

func TestDetectStackEvidenceMerges(t *testing.T) {
	base, _ := url.Parse("https://app.test/")
	pages := []pageData{
		pd("https://app.test/a", nil, `<script src="/_next/static/a.js"></script>`),
		pd("https://app.test/b", nil, `<script src="/_next/static/b.js"></script>`),
	}
	fs := detectStack(pages, base)
	f := findFinding(fs, "Next.js")
	if f == nil || len(f.Evidence) != 2 {
		t.Fatalf("expected merged evidence from both pages, got %+v", f)
	}
}

func TestDetectStackSkipsFailedPages(t *testing.T) {
	base, _ := url.Parse("https://app.test/")
	p := pd("https://app.test/", nil, `__NEXT_DATA__`)
	p.Error = "boom"
	if fs := detectStack([]pageData{p}, base); len(fs) != 0 {
		t.Fatalf("expected no findings, got %v", fs)
	}
}

func TestDetectAPISurface(t *testing.T) {
	base, _ := url.Parse("https://app.test/")
	p := pd("https://app.test/", nil, `
		<a href="/api/users">users</a>
		<a href="/api/users">users dup</a>
		<a href="/graphql">gql</a>
		<a href="/docs/">docs</a>
		<a href="/about">no</a>`)
	hits := detectAPISurface([]pageData{p}, base)
	kinds := map[string]string{}
	for _, h := range hits {
		kinds[h.URL] = h.Kind
	}
	if kinds["https://app.test/api/users"] != "api" {
		t.Fatalf("api hit missing: %v", hits)
	}
	if kinds["https://app.test/graphql"] != "graphql" {
		t.Fatalf("graphql hit missing: %v", hits)
	}
	if kinds["https://app.test/docs/"] != "docs" {
		t.Fatalf("docs hit missing: %v", hits)
	}
	if len(hits) != 3 {
		t.Fatalf("expected 3 deduped hits, got %v", hits)
	}
}

func TestDetectStackDrupal(t *testing.T) {
	base, _ := url.Parse("https://cms.test/")
	cases := []struct {
		name    string
		headers map[string]string
		body    string
		detail  string
	}{
		{"x-generator header", map[string]string{"X-Generator": "Drupal 10 (https://www.drupal.org)"}, "",
			"header x-generator: Drupal 10 (https://www.drupal.org)"},
		{"x-drupal-cache header", map[string]string{"X-Drupal-Cache": "HIT"}, "",
			"header x-drupal-cache: HIT"},
		{"x-drupal-dynamic-cache header", map[string]string{"X-Drupal-Dynamic-Cache": "MISS"}, "",
			"header x-drupal-dynamic-cache: MISS"},
		{"files path in body", nil, `<img src="/sites/default/files/logo.png">`,
			"HTML matches //sites/default/files//"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := detectStack([]pageData{pd("https://cms.test/", tc.headers, tc.body)}, base)
			f := findFinding(fs, "Drupal")
			if f == nil {
				t.Fatalf("Drupal not detected, findings %+v", fs)
			}
			if f.Category != "cms" || f.Confidence != "observed" {
				t.Fatalf("Drupal finding %+v", f)
			}
			if len(f.Evidence) != 1 || f.Evidence[0].Detail != tc.detail {
				t.Fatalf("evidence %+v, want detail %q", f.Evidence, tc.detail)
			}
		})
	}
}

func TestDetectStackDrupalIgnoresOtherGenerators(t *testing.T) {
	base, _ := url.Parse("https://cms.test/")
	p := pd("https://cms.test/", map[string]string{"X-Generator": "Some Other CMS 2.0"}, "<p>hi</p>")
	if f := findFinding(detectStack([]pageData{p}, base), "Drupal"); f != nil {
		t.Fatalf("unexpected Drupal finding %+v", f)
	}
}
