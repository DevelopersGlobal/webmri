package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTopCounts(t *testing.T) {
	got := topCounts(map[string]int{"b": 5, "a": 5, "c": 1}, 2)
	if len(got) != 2 || got[0].Token != "a" || got[1].Token != "b" {
		t.Fatalf("topCounts %v", got)
	}
}

func TestExtractDesignInlineAndSheet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(`:root { --accent: #00ff00; --space: 8px }
			body { font-family: Inter, system-ui, sans-serif; color: #111111; }
			.btn { color: rgb(1, 2, 3); }`))
	}))
	defer srv.Close()

	p := pageData{
		PageResult: PageResult{URL: srv.URL + "/"},
		htmlInfo: &htmlInfo{
			styleBodies:    []string{`:root { --brand: #FF6600; } .x { color: #ff6600; }`},
			stylesheetURLs: []string{srv.URL + "/styles.css"},
		},
	}
	base := mustURL(t, srv.URL)
	opt := Options{Timeout: 5 * time.Second}.withDefaults()
	dt := extractDesign(newClient(opt), []pageData{p}, base, opt)

	if dt.Stylesheets != 1 {
		t.Fatalf("stylesheets scanned %d", dt.Stylesheets)
	}
	if dt.CustomProperties["--brand"] != "#FF6600" {
		t.Fatalf("props %v", dt.CustomProperties)
	}
	if dt.CustomProperties["--accent"] != "#00ff00" {
		t.Fatalf("props %v", dt.CustomProperties)
	}
	foundColor := false
	for _, c := range dt.TopColors {
		if c.Token == "#ff6600" && c.Count == 2 {
			foundColor = true
		}
	}
	if !foundColor {
		t.Fatalf("colors %v", dt.TopColors)
	}
	if len(dt.TopFonts) == 0 || dt.TopFonts[0].Token != "Inter" {
		t.Fatalf("fonts %v", dt.TopFonts)
	}
}
