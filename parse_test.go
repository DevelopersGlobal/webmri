package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestNormalizeURLAddsScheme(t *testing.T) {
	u, err := normalizeURL("example.com/path")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Host != "example.com" {
		t.Fatalf("got %s", u)
	}
}

func TestNormalizeURLStripsFragment(t *testing.T) {
	u, err := normalizeURL("https://example.com/p#frag")
	if err != nil {
		t.Fatal(err)
	}
	if u.Fragment != "" {
		t.Fatalf("fragment kept: %s", u)
	}
}

func TestNormalizeURLRejectsBadInput(t *testing.T) {
	for _, in := range []string{"", "ftp://example.com", "https://"} {
		if _, err := normalizeURL(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func TestSameHostWWW(t *testing.T) {
	base := mustURL(t, "https://www.example.com")
	if !sameHost(base, "https://example.com/a") {
		t.Fatal("www mismatch should still be same host")
	}
	if sameHost(base, "https://other.com/a") {
		t.Fatal("different host reported as same")
	}
	if sameHost(base, "not a url :://") {
		t.Fatal("invalid URL reported as same host")
	}
}

func TestCleanLink(t *testing.T) {
	base := mustURL(t, "https://example.com/dir/page")
	if got := cleanLink(base, "../other#x"); got != "https://example.com/other" {
		t.Fatalf("relative resolve: %s", got)
	}
	for _, bad := range []string{"", "mailto:a@b.c", "tel:123", "javascript:void(0)", "data:text/html,x"} {
		if got := cleanLink(base, bad); got != "" {
			t.Fatalf("expected empty for %q, got %q", bad, got)
		}
	}
}

const fixtureHTML = `<!doctype html>
<html><head>
<title>  Fixture App </title>
<meta name="generator" content="FixtureGen 1.0">
<meta name="description" content="A test app">
<link rel="stylesheet" href="/styles.css">
<link rel="alternate" type="application/rss+xml" href="feeds/rss.xml#latest">
<link rel="alternate" type="application/atom+xml; charset=utf-8" href="https://fixture.test/atom.xml">
<link rel="alternate" type="text/html" href="/not-a-feed">
<style>:root { --brand: #ff6600; }</style>
<script src="/_next/static/chunk.js"></script>
<script src="/_next/static/chunk.js"></script>
</head><body>
<a href="/about">About</a>
<a href="https://external.example/x">External</a>
<form action="/search" method="post">
  <input type="text" name="q">
  <input type="hidden" name="page">
  <input type="text" name="q">
  <select name="sort"></select>
</form>
</body></html>`

func TestParseHTML(t *testing.T) {
	base := mustURL(t, "https://fixture.test/")
	info := parseHTML([]byte(fixtureHTML), base, "https://fixture.test/")
	if info.title != "Fixture App" {
		t.Fatalf("title %q", info.title)
	}
	if info.generator != "FixtureGen 1.0" {
		t.Fatalf("generator %q", info.generator)
	}
	if info.description != "A test app" {
		t.Fatalf("description %q", info.description)
	}
	if len(info.scripts) != 1 || info.scripts[0] != "https://fixture.test/_next/static/chunk.js" {
		t.Fatalf("scripts %v", info.scripts)
	}
	if len(info.stylesheetURLs) != 1 || info.stylesheetURLs[0] != "https://fixture.test/styles.css" {
		t.Fatalf("stylesheets %v", info.stylesheetURLs)
	}
	if len(info.feeds) != 2 || info.feeds[0] != (FeedInfo{URL: "https://fixture.test/feeds/rss.xml", Page: "https://fixture.test/"}) || info.feeds[1] != (FeedInfo{URL: "https://fixture.test/atom.xml", Page: "https://fixture.test/"}) {
		t.Fatalf("feeds %v", info.feeds)
	}
	if len(info.styleBodies) != 1 || !strings.Contains(info.styleBodies[0], "--brand") {
		t.Fatalf("style bodies %v", info.styleBodies)
	}
	if len(info.links) != 2 {
		t.Fatalf("links %v", info.links)
	}
	if len(info.forms) != 1 {
		t.Fatalf("forms %v", info.forms)
	}
	f := info.forms[0]
	if f.Method != "POST" || f.Action != "https://fixture.test/search" {
		t.Fatalf("form %+v", f)
	}
	want := []string{"q (text)", "page (hidden)", "sort"}
	if len(f.Inputs) != len(want) {
		t.Fatalf("inputs %v", f.Inputs)
	}
	for i, w := range want {
		if f.Inputs[i] != w {
			t.Fatalf("inputs %v, want %v", f.Inputs, want)
		}
	}
}

func TestParseHTMLGarbage(t *testing.T) {
	info := parseHTML([]byte("not html at all \x00\x01"), mustURL(t, "https://x.test/"), "https://x.test/")
	if info == nil {
		t.Fatal("nil info on garbage input")
	}
}

func TestParseHTMLAccessibilityFacts(t *testing.T) {
	base := mustURL(t, "https://fixture.test/")
	info := parseHTML([]byte(`<html lang="en"><body><h1>Heading</h1><img><img alt=""><img alt="photo"></body></html>`), base, "https://fixture.test/")
	if info.images != 3 || info.imagesMissingAlt != 1 || info.imagesEmptyAlt != 1 || !info.hasLang || !info.hasH1 {
		t.Fatalf("accessibility facts: images=%d missing_alt=%d empty_alt=%d lang=%t h1=%t; want 3, 1, 1, true, true", info.images, info.imagesMissingAlt, info.imagesEmptyAlt, info.hasLang, info.hasH1)
	}
	without := parseHTML([]byte(`<html><body><img alt="decorative"></body></html>`), base, "https://fixture.test/other")
	if without.hasLang || without.hasH1 || without.images != 1 || without.imagesMissingAlt != 0 || without.imagesEmptyAlt != 0 {
		t.Fatalf("facts for langless page: images=%d missing_alt=%d empty_alt=%d lang=%t h1=%t; want 1, 0, 0, false, false", without.images, without.imagesMissingAlt, without.imagesEmptyAlt, without.hasLang, without.hasH1)
	}
	page := PageResult{URL: "https://fixture.test/", Images: info.images, ImagesMissingAlt: info.imagesMissingAlt, ImagesEmptyAlt: info.imagesEmptyAlt, HasLang: info.hasLang, HasH1: info.hasH1}
	rep := &Report{Pages: []PageResult{page}}
	data, err := WriteJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	jsonPage := decoded["pages"].([]interface{})[0].(map[string]interface{})
	if jsonPage["images"] != float64(3) || jsonPage["images_missing_alt"] != float64(1) || jsonPage["images_empty_alt"] != float64(1) || jsonPage["has_lang"] != true || jsonPage["has_h1"] != true {
		t.Fatalf("JSON page facts %v", jsonPage)
	}
	if got := WriteMarkdown(rep); !strings.Contains(got, fmt.Sprintf("| %s | 3 | 1 | 1 | true | true |", page.URL)) {
		t.Fatalf("markdown missing accessibility facts row: %s", got)
	}
}

func TestDefaultFormMethod(t *testing.T) {
	html := `<form><input name="a"></form>`
	info := parseHTML([]byte(html), mustURL(t, "https://x.test/"), "https://x.test/")
	if len(info.forms) != 1 || info.forms[0].Method != "GET" || info.forms[0].Action != "" {
		t.Fatalf("form %+v", info.forms)
	}
}

func TestPageLike(t *testing.T) {
	page := []string{"https://x.test/", "https://x.test/about", "https://x.test/a.b/c", "https://x.test/file"}
	for _, u := range page {
		if !pageLike(u) {
			t.Fatalf("pageLike(%s) = false", u)
		}
	}
	nonPage := []string{"https://x.test/logo.png", "https://x.test/a/report.PDF", "https://x.test/app.js?v=2", "https://x.test/f.woff2", "https://x.test/d.zip", "::bad"}
	for _, u := range nonPage {
		if pageLike(u) {
			t.Fatalf("pageLike(%s) = true", u)
		}
	}
}
