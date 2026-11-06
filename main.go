package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func usage() {
	fmt.Fprintf(os.Stderr, `webmri %s — an MRI for websites and web apps

One command shows what a site publicly serves over HTTP: stack fingerprints,
design tokens, routes, forms and programmatic surface, with evidence for
every claim.

Usage:
  webmri [scan] <url> [flags]
  webmri -version

Flags:
`, version)
	flag.CommandLine.PrintDefaults()
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	var (
		pages       = flag.Int("pages", 25, "maximum same-host pages to scan (1-500)")
		conc        = flag.Int("concurrency", 8, "parallel fetches")
		timeout     = flag.Duration("timeout", 10*time.Second, "per-request timeout")
		outMD       = flag.String("out", "", "write the Markdown report here (default: stdout)")
		outJSON     = flag.String("json", "", "also write the full report as JSON to this path")
		noCrawl     = flag.Bool("no-crawl", false, "scan only the target page, no sitemap or link crawling")
		maxCSS      = flag.Int("css", 8, "maximum stylesheets to fetch for design tokens")
		showHelp    = flag.Bool("h", false, "show help")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = usage
	// Accept flags before or after the URL: parse repeatedly, pulling one
	// positional argument out per round.
	if len(args) > 0 && args[0] == "scan" {
		args = args[1:]
	}
	var rest []string
	for len(args) > 0 {
		if err := flag.CommandLine.Parse(args); err != nil {
			return 1
		}
		args = flag.Args()
		if len(args) > 0 {
			rest = append(rest, args[0])
			args = args[1:]
		}
	}
	if *showHelp {
		usage()
		return 0
	}
	if *showVersion {
		fmt.Printf("webmri %s\n", version)
		return 0
	}
	if len(rest) != 1 {
		usage()
		return 1
	}

	rep, err := Scan(rest[0], Options{
		MaxPages: *pages, Concurrency: *conc, Timeout: *timeout,
		NoCrawl: *noCrawl, MaxCSSFiles: *maxCSS,
	})
	if err != nil {
		var ue *usageError
		if errors.As(err, &ue) {
			fmt.Fprintln(os.Stderr, "webmri:", err)
			usage()
			return 1
		}
		fmt.Fprintln(os.Stderr, "webmri: scan failed:", err)
		return 1
	}

	md := WriteMarkdown(rep)
	if *outMD == "" {
		fmt.Print(md)
	} else if err := os.WriteFile(*outMD, []byte(md), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "webmri: writing markdown:", err)
		return 1
	}
	if *outJSON != "" {
		data, err := WriteJSON(rep)
		if err != nil {
			fmt.Fprintln(os.Stderr, "webmri: encoding json:", err)
			return 1
		}
		if err := os.WriteFile(*outJSON, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "webmri: writing json:", err)
			return 1
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "webmri: %d pages in %d ms", len(rep.Pages), rep.WallMS)
	if len(rep.Stack) > 0 {
		fmt.Fprintf(&sb, ", %d stack findings", len(rep.Stack))
	}
	if *outMD != "" {
		fmt.Fprintf(&sb, " — report: %s", *outMD)
	}
	if *outJSON != "" {
		fmt.Fprintf(&sb, " — json: %s", *outJSON)
	}
	fmt.Fprintln(os.Stderr, sb.String())
	return 0
}
