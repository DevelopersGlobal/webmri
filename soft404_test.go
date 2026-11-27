package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSoftNotFoundNote(t *testing.T) {
	flagged := []string{
		"Page not found",
		"404 | Example",
		"Error 404 - Example Shop",
		"Not Found",
		"Sorry, this page doesn't exist",
		"The page could not be found",
	}
	for _, title := range flagged {
		if got := softNotFoundNote(200, title); !strings.HasPrefix(got, "inferred: ") || !strings.Contains(got, title) {
			t.Errorf("title %q: note %q, want an inferred note quoting the title", title, got)
		}
	}
	plain := []string{"", "Home", "Pricing", "Top 4040 deals", "Found footage films", "Model X404"}
	for _, title := range plain {
		if got := softNotFoundNote(200, title); got != "" {
			t.Errorf("title %q flagged: %q", title, got)
		}
	}
	// A real 404 is reported by its status code, not as a soft 404.
	if got := softNotFoundNote(404, "Page not found"); got != "" {
		t.Errorf("404 status flagged as soft 404: %q", got)
	}
}

func TestScanFlagsSoft404Pages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/missing" {
			fmt.Fprint(w, `<!doctype html><html><head><title>Page not found</title></head><body>nothing here</body></html>`)
			return
		}
		fmt.Fprint(w, `<!doctype html><html><head><title>Home</title></head><body><a href="/missing">gone</a></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	rep, err := Scan(srv.URL+"/", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	notes := map[string]string{}
	for _, p := range rep.Pages {
		notes[p.URL] = p.SoftNotFound
	}
	if got := notes[srv.URL+"/missing"]; !strings.HasPrefix(got, "inferred: possible soft 404") {
		t.Fatalf("missing page note %q, want an inferred soft 404 note; pages %v", got, notes)
	}
	if got, ok := notes[srv.URL+"/"]; !ok || got != "" {
		t.Fatalf("home page note %q (found %t), want none", got, ok)
	}

	data, err := WriteJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Pages []map[string]interface{} `json:"pages"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, p := range decoded.Pages {
		_, has := p["soft_404"]
		if want := p["url"] == srv.URL+"/missing"; has != want {
			t.Fatalf("JSON page %v: soft_404 present=%t, want %t", p, has, want)
		}
	}

	md := WriteMarkdown(rep)
	for _, want := range []string{
		"## Possible soft 404s\n\n| page | status | inferred note |\n| --- | ---: | --- |\n",
		fmt.Sprintf("| %s/missing | 200 | inferred: possible soft 404", srv.URL),
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

// Reports with no suspicious titles render exactly as before: no section.
func TestWriteMarkdownOmitsSoft404SectionWhenNoneFlagged(t *testing.T) {
	rep := &Report{
		Tool: "webmri", Version: "test", Target: "https://example.com",
		Pages: []PageResult{{URL: "https://example.com/", Title: "Home", Status: 200}},
	}
	if md := WriteMarkdown(rep); strings.Contains(md, "soft 404") {
		t.Fatalf("unexpected soft 404 section:\n%s", md)
	}
}
