package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	index := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("cf-ray", "test-SIN")
		fmt.Fprint(w, `<!doctype html><html><head>
			<title>Fixture App</title>
			<meta name="generator" content="Hugo 0.120">
			<meta name="description" content="A test app">
			<link rel="stylesheet" href="/styles.css">
			<style>:root { --brand: #ff6600; }</style>
			<script id="__NEXT_DATA__" type="application/json">{}</script>
			</head><body>
			<a href="/about">About</a>
			<a href="/contact">Contact</a>
			<a href="/api/users">API</a>
			<a href="/logo.png">Logo</a>
			<a href="/files/report.PDF">Report</a>
			<a href="/app.js">Script</a>
			<form action="/search" method="post"><input type="text" name="q"></form>
			</body></html>`)
	}
	page := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><title>Inner</title></head><body>inner</body></html>`)
	}
	mux.HandleFunc("/", index)
	mux.HandleFunc("/about", page)
	mux.HandleFunc("/contact", page)
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[]}`)
	})
	mux.HandleFunc("/styles.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		fmt.Fprint(w, `:root { --accent: #00ff00; } body { font-family: Inter, sans-serif; }`)
	})
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s\n", "")
	})
	srv := httptest.NewServer(mux)
	// rewrite robots now that we know the URL
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/", index)
	mux2.HandleFunc("/about", page)
	mux2.HandleFunc("/contact", page)
	mux2.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[]}`)
	})
	mux2.HandleFunc("/styles.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		fmt.Fprint(w, `:root { --accent: #00ff00; } body { font-family: Inter, sans-serif; }`)
	})
	mux2.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", srv.URL)
	})
	mux2.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
			<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
			<url><loc>%s/</loc></url>
			<url><loc>%s/about</loc></url>
			</urlset>`, srv.URL, srv.URL)
	})
	srv.Config.Handler = mux2
	return srv
}

func testOpts() Options {
	return Options{MaxPages: 10, Concurrency: 4, Timeout: 5 * time.Second}.withDefaults()
}

func TestScanEndToEnd(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()

	rep, err := Scan(srv.URL+"/", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Tool != "webmri" || rep.Version == "" {
		t.Fatalf("identity %+v", rep)
	}
	if rep.WallMS < 0 {
		t.Fatalf("wall time %d", rep.WallMS)
	}
	if len(rep.Pages) != 4 {
		t.Fatalf("expected 4 pages (/ /about /contact /api/users), got %d: %v", len(rep.Pages), rep.Pages)
	}
	for _, p := range rep.Pages {
		if strings.Contains(p.URL, ".png") || strings.Contains(strings.ToLower(p.URL), ".pdf") || strings.HasSuffix(p.URL, ".js") {
			t.Fatalf("non-page URL crawled: %s", p.URL)
		}
	}
	routeSet := map[string]bool{}
	for _, r := range rep.Routes {
		routeSet[r] = true
	}
	for _, want := range []string{srv.URL + "/", srv.URL + "/about", srv.URL + "/contact", srv.URL + "/api/users"} {
		if !routeSet[want] {
			t.Fatalf("route %s missing from %v", want, rep.Routes)
		}
	}
	if f := findFinding(rep.Stack, "Next.js"); f == nil || f.Confidence != "observed" {
		t.Fatalf("Next.js %+v", f)
	}
	if f := findFinding(rep.Stack, "Cloudflare"); f == nil {
		t.Fatal("Cloudflare missing")
	}
	if f := findFinding(rep.Stack, "Hugo"); f == nil {
		t.Fatal("Hugo (generator) missing")
	}
	if rep.Design.CustomProperties["--brand"] != "#ff6600" {
		t.Fatalf("design props %v", rep.Design.CustomProperties)
	}
	if rep.Design.Stylesheets != 1 {
		t.Fatalf("stylesheets %d", rep.Design.Stylesheets)
	}
	if len(rep.Forms) != 1 || rep.Forms[0].Method != "POST" {
		t.Fatalf("forms %+v", rep.Forms)
	}
	foundAPI := false
	for _, a := range rep.APIs {
		if a.Kind == "api" && strings.HasSuffix(a.URL, "/api/users") {
			foundAPI = true
		}
	}
	if !foundAPI {
		t.Fatalf("api surface %v", rep.APIs)
	}
	if len(rep.Notes) == 0 {
		t.Fatal("notes missing")
	}
}

func TestScanNoCrawl(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	opt := testOpts()
	opt.NoCrawl = true
	rep, err := Scan(srv.URL+"/", opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Pages) != 1 {
		t.Fatalf("no-crawl should scan 1 page, got %d", len(rep.Pages))
	}
}

func TestScanRespectsPageCap(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	opt := testOpts()
	opt.MaxPages = 2
	rep, err := Scan(srv.URL+"/", opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Pages) != 2 {
		t.Fatalf("cap: got %d pages", len(rep.Pages))
	}
}

func TestScanUnreachableHost(t *testing.T) {
	srv := fixtureServer(t)
	url := srv.URL
	srv.Close() // now unreachable
	rep, err := Scan(url+"/", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Pages) != 1 || rep.Pages[0].Error == "" {
		t.Fatalf("expected one failed page, got %+v", rep.Pages)
	}
}

func TestWriteOutputs(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	rep, err := Scan(srv.URL+"/", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	md := WriteMarkdown(rep)
	for _, want := range []string{"# Anatomy of", "## Stack", "## Design tokens", "## Routes", "## Forms", "Next.js", "--brand", "Method and limits"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q", want)
		}
	}
	data, err := WriteJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"tool": "webmri"`) {
		t.Fatal("json missing tool field")
	}
}

func TestPerfStats(t *testing.T) {
	pages := []PageResult{
		{URL: "a", DurationMS: 30},
		{URL: "b", DurationMS: 10},
		{URL: "c", DurationMS: 20},
		{URL: "d", Error: "x"},
	}
	fast, slow, med, ok := perfStats(pages)
	if !ok || fast.URL != "b" || slow.URL != "a" || med.URL != "c" {
		t.Fatalf("perfStats %v %v %v", fast, slow, med)
	}
	if _, _, _, ok := perfStats([]PageResult{{Error: "x"}}); ok {
		t.Fatal("perfStats should report not-ok with all failures")
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int]string{500: "500 B", 2048: "2.0 KB", 3 << 20: "3.0 MB"}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Fatalf("humanBytes(%d) = %s, want %s", in, got, want)
		}
	}
}

func TestFetchDecodesGzipHTML(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	fmt.Fprint(gz, `<!doctype html><html><head><title>Zipped</title></head><body>compressed</body></html>`)
	gz.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept-Encoding"); got != "gzip" {
			t.Errorf("Accept-Encoding = %q, want exactly gzip", got)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()
	pd := fetchPage(newClient(Options{Timeout: 5 * time.Second}.withDefaults()), srv.URL, Options{Timeout: 5 * time.Second}.withDefaults())
	if pd.Error != "" {
		t.Fatalf("unexpected error: %s", pd.Error)
	}
	if pd.Title != "Zipped" {
		t.Fatalf("title = %q, want Zipped (gzip not decoded)", pd.Title)
	}
}

func TestFetchUnsupportedEncodingIsVisible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "br")
		w.Write([]byte{0x1b, 0x03, 0x00})
	}))
	defer srv.Close()
	pd := fetchPage(newClient(Options{Timeout: 5 * time.Second}.withDefaults()), srv.URL, Options{Timeout: 5 * time.Second}.withDefaults())
	if !strings.Contains(pd.Error, `unsupported Content-Encoding "br"`) {
		t.Fatalf("error = %q, want unsupported Content-Encoding", pd.Error)
	}
}

func TestFetchCorruptGzipIsVisible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		w.Write([]byte("not actually gzip"))
	}))
	defer srv.Close()
	pd := fetchPage(newClient(Options{Timeout: 5 * time.Second}.withDefaults()), srv.URL, Options{Timeout: 5 * time.Second}.withDefaults())
	if !strings.Contains(pd.Error, "invalid gzip response") {
		t.Fatalf("error = %q, want invalid gzip response", pd.Error)
	}
}
