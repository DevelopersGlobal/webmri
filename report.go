package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// WriteJSON renders the report as indented JSON.
func WriteJSON(rep *Report) ([]byte, error) {
	return json.MarshalIndent(rep, "", "  ")
}

// perfStats returns fastest/slowest/median fetch times over successful pages.
func perfStats(pages []PageResult) (fastest, slowest, median PageResult, ok bool) {
	var good []PageResult
	for _, p := range pages {
		if p.Error == "" {
			good = append(good, p)
		}
	}
	if len(good) == 0 {
		return PageResult{}, PageResult{}, PageResult{}, false
	}
	sort.Slice(good, func(i, j int) bool { return good[i].DurationMS < good[j].DurationMS })
	return good[0], good[len(good)-1], good[len(good)/2], true
}

// WriteMarkdown renders the human-readable anatomy report.
func WriteMarkdown(rep *Report) string {
	var b strings.Builder
	w := func(format string, args ...interface{}) { fmt.Fprintf(&b, format+"\n", args...) }

	w("# Anatomy of %s", rep.Target)
	w("")
	w("Scanned by webmri %s on %s. Every claim below carries the evidence it stands on.", rep.Version, rep.ScannedAt)
	w("")
	w("## Summary")
	w("")
	w("| metric | value |")
	w("| --- | --- |")
	w("| Pages scanned | %d |", len(rep.Pages))
	w("| Wall time | %d ms |", rep.WallMS)
	total := 0
	errs := 0
	for _, p := range rep.Pages {
		total += p.Bytes
		if p.Error != "" {
			errs++
		}
	}
	w("| Total bytes fetched | %s |", humanBytes(total))
	if errs > 0 {
		w("| Failed fetches | %d |", errs)
	}
	if fast, slow, med, ok := perfStats(rep.Pages); ok {
		w("| Fastest page | %s (%d ms) |", fast.URL, fast.DurationMS)
		w("| Slowest page | %s (%d ms) |", slow.URL, slow.DurationMS)
		w("| Median page fetch | %d ms |", med.DurationMS)
	}
	w("")

	w("## Stack")
	w("")
	if len(rep.Stack) == 0 {
		w("No stack signals detected.")
	} else {
		w("| name | category | confidence | evidence |")
		w("| --- | --- | --- | --- |")
		for _, f := range rep.Stack {
			ev := f.Evidence[0]
			extra := ""
			if len(f.Evidence) > 1 {
				extra = fmt.Sprintf(" (+%d more)", len(f.Evidence)-1)
			}
			w("| %s | %s | %s | %s — %s%s |", f.Name, f.Category, f.Confidence, ev.Detail, ev.URL, extra)
		}
	}
	w("")

	w("## Design tokens")
	w("")
	w("Stylesheets scanned: %d", rep.Design.Stylesheets)
	w("")
	if len(rep.Design.CustomProperties) > 0 {
		w("CSS custom properties (`:root`):")
		w("")
		w("```css")
		keys := make([]string, 0, len(rep.Design.CustomProperties))
		for k := range rep.Design.CustomProperties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		limit := len(keys)
		if limit > 30 {
			limit = 30
		}
		for _, k := range keys[:limit] {
			w("%s: %s;", k, rep.Design.CustomProperties[k])
		}
		if len(keys) > limit {
			w("/* … %d more */", len(keys)-limit)
		}
		w("```")
		w("")
	}
	if len(rep.Design.TopColors) > 0 {
		w("Most-used colors:")
		w("")
		for _, c := range rep.Design.TopColors {
			w("- `%s` (%d uses)", c.Token, c.Count)
		}
		w("")
	}
	if len(rep.Design.TopFonts) > 0 {
		w("Font families:")
		w("")
		for _, f := range rep.Design.TopFonts {
			w("- %s (%d uses)", f.Token, f.Count)
		}
		w("")
	}

	w("## Routes")
	w("")
	if len(rep.Routes) == 0 {
		w("No routes captured.")
	} else {
		limit := len(rep.Routes)
		if limit > 60 {
			limit = 60
		}
		for _, r := range rep.Routes[:limit] {
			w("- %s", r)
		}
		if len(rep.Routes) > limit {
			w("- … %d more (see JSON output)", len(rep.Routes)-limit)
		}
	}
	w("")

	w("## Forms")
	w("")
	if len(rep.Forms) == 0 {
		w("No forms found on scanned pages.")
	} else {
		for _, f := range rep.Forms {
			target := f.Action
			if target == "" {
				target = "(same page)"
			}
			w("- `%s %s` on %s — fields: %s", f.Method, target, f.Page, strings.Join(f.Inputs, ", "))
		}
	}
	w("")

	w("## Programmatic surface (inferred)")
	w("")
	if len(rep.APIs) == 0 {
		w("No API, GraphQL or docs endpoints detected in links or assets.")
	} else {
		for _, a := range rep.APIs {
			w("- [%s] %s (seen on %s)", a.Kind, a.URL, a.Source)
		}
	}
	w("")

	w("## Method and limits")
	w("")
	for _, n := range rep.Notes {
		w("- %s", n)
	}
	w("")
	return b.String()
}

func humanBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
