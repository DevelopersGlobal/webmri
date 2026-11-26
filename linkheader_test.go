package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseLinkHeader(t *testing.T) {
	base, _ := url.Parse("https://site.test/blog/")
	h := http.Header{}
	h.Add("Link", `<https://site.test/wp-json/>; rel="https://api.w.org/", </feed/>; rel="alternate"; type="application/rss+xml"; title="Posts, all"`)
	h.Add("Link", `<atom.xml#top>; rel="Alternate Feed"; type="application/atom+xml; charset=utf-8"`)
	h.Add("Link", `no target here, <javascript:alert(1)>; rel=alternate`)
	got := parseLinkHeader(h, base)
	if len(got) != 3 {
		t.Fatalf("entries %+v", got)
	}
	if got[0].target != "https://site.test/wp-json/" || !got[0].hasRel(wpAPIRel) {
		t.Fatalf("api entry %+v", got[0])
	}
	if got[1].target != "https://site.test/feed/" || !got[1].hasRel("alternate") || got[1].typ != "application/rss+xml" {
		t.Fatalf("rss entry %+v", got[1])
	}
	if got[2].target != "https://site.test/blog/atom.xml" || !got[2].hasRel("alternate") || !got[2].hasRel("feed") || got[2].typ != "application/atom+xml" {
		t.Fatalf("atom entry %+v", got[2])
	}
}

func TestLinkHeaderHintsInReport(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Add("Link", `</wp-json/>; rel="https://api.w.org/"`)
		w.Header().Add("Link", `</feed/>; rel="alternate"; type="application/rss+xml", </html-feed.xml>; rel="alternate"; type="application/rss+xml"`)
		fmt.Fprint(w, `<!doctype html><html><head><title>WP</title>
			<link rel="alternate" type="application/rss+xml" href="/html-feed.xml">
			</head><body><a href="/about">About</a></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rep, err := Scan(srv.URL+"/", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	page := srv.URL + "/"
	wantFeeds := []FeedInfo{
		{URL: srv.URL + "/html-feed.xml", Page: page},
		{URL: srv.URL + "/feed/", Page: page, Via: linkHeaderSource},
	}
	if len(rep.Feeds) != len(wantFeeds) || rep.Feeds[0] != wantFeeds[0] || rep.Feeds[1] != wantFeeds[1] {
		t.Fatalf("feeds %+v", rep.Feeds)
	}
	var api *APIHit
	for i := range rep.APIs {
		if rep.APIs[i].URL == srv.URL+"/wp-json/" {
			api = &rep.APIs[i]
		}
	}
	if api == nil || api.Kind != "api" || api.Source != page || api.Via != linkHeaderSource {
		t.Fatalf("api surface %+v", rep.APIs)
	}

	data, err := WriteJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Feeds []map[string]any `json:"feeds"`
		APIs  []map[string]any `json:"api_surface"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded.Feeds[0]["via"]; ok {
		t.Fatalf("HTML feed should not carry via: %v", decoded.Feeds[0])
	}
	if decoded.Feeds[1]["via"] != linkHeaderSource {
		t.Fatalf("header feed json %v", decoded.Feeds[1])
	}

	md := WriteMarkdown(rep)
	for _, want := range []string{
		"- " + srv.URL + "/html-feed.xml (found on " + page + ")\n",
		"- " + srv.URL + "/feed/ (found on " + page + " via Link header)\n",
		"- [api] " + srv.URL + "/wp-json/ (seen on " + page + " via Link header)\n",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestLinkHeaderIgnoredOnErrorPages(t *testing.T) {
	base, _ := url.Parse("https://app.test/")
	p := pd("https://app.test/", map[string]string{"Link": `</wp-json/>; rel="https://api.w.org/"`}, "")
	p.Status = 404
	if hits := detectAPISurface([]pageData{p}, base); len(hits) != 0 {
		t.Fatalf("expected no hits, got %v", hits)
	}
	if feeds := collectFeeds([]pageData{p}); len(feeds) != 0 {
		t.Fatalf("expected no feeds, got %v", feeds)
	}
}
