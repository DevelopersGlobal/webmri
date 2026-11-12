package main

import (
	"net/url"
	"regexp"
)

type headerRule struct {
	key string
	re  *regexp.Regexp
}

type stackRule struct {
	name       string
	category   string
	confidence string // observed | inferred
	headers    []headerRule
	body       []*regexp.Regexp // matched against raw HTML
	srcs       []*regexp.Regexp // matched against script/stylesheet URLs
	css        []*regexp.Regexp // matched against fetched CSS
	generator  *regexp.Regexp   // matched against meta[name=generator]
}

func re(p string) *regexp.Regexp { return regexp.MustCompile(p) }

var stackRules = []stackRule{
	// Frameworks
	{name: "Next.js", category: "framework", confidence: "observed",
		body:    []*regexp.Regexp{re(`__NEXT_DATA__`), re(`/_next/static/`)},
		headers: []headerRule{{"x-powered-by", re(`(?i)next\.js`)}}},
	{name: "Nuxt", category: "framework", confidence: "observed",
		body: []*regexp.Regexp{re(`__NUXT__`), re(`/_nuxt/`)}},
	{name: "Gatsby", category: "framework", confidence: "observed",
		body: []*regexp.Regexp{re(`___gatsby`), re(`/gatsby-`)}},
	{name: "React", category: "library", confidence: "inferred",
		body: []*regexp.Regexp{re(`data-reactroot`), re(`data-reactid`)},
		srcs: []*regexp.Regexp{re(`(?i)/react(\.production|\.development)?(\.min)?\.js`)}},
	{name: "Vue.js", category: "library", confidence: "inferred",
		srcs: []*regexp.Regexp{re(`(?i)/vue(\.runtime)?(\.global|\.min)?\.js`)}},
	{name: "Angular", category: "framework", confidence: "observed",
		body: []*regexp.Regexp{re(`ng-version=`), re(`\bng-app\b`)}},
	{name: "SvelteKit", category: "framework", confidence: "observed",
		body: []*regexp.Regexp{re(`__sveltekit`)}},
	{name: "Astro", category: "framework", confidence: "observed",
		body:      []*regexp.Regexp{re(`astro-island`)},
		generator: re(`(?i)\bAstro\b`)},
	// CMS / generators
	{name: "WordPress", category: "cms", confidence: "observed",
		body:      []*regexp.Regexp{re(`/wp-content/`), re(`/wp-includes/`)},
		generator: re(`(?i)WordPress`)},
	{name: "Drupal", category: "cms", confidence: "observed",
		body: []*regexp.Regexp{re(`/sites/default/files/`)},
		headers: []headerRule{
			{"x-generator", re(`(?i)\bDrupal\b`)},
			{"x-drupal-cache", re(`.`)},
			{"x-drupal-dynamic-cache", re(`.`)},
		}},
	{name: "Ghost", category: "cms", confidence: "observed",
		body:      []*regexp.Regexp{re(`/ghost/api/`), re(`/ghost/assets/`)},
		generator: re(`(?i)\bGhost\b`)},
	{name: "Shopify", category: "platform", confidence: "observed",
		body:    []*regexp.Regexp{re(`cdn\.shopify\.com`)},
		headers: []headerRule{{"x-shopify-stage", re(`.`)}, {"x-shopid", re(`.`)}}},
	{name: "Hugo", category: "generator", confidence: "observed", generator: re(`(?i)\bHugo\b`)},
	{name: "Jekyll", category: "generator", confidence: "observed", generator: re(`(?i)\bJekyll\b`)},
	{name: "Eleventy", category: "generator", confidence: "observed", generator: re(`(?i)\bEleventy\b`)},
	{name: "Docusaurus", category: "generator", confidence: "observed", generator: re(`(?i)\bDocusaurus\b`)},
	// Styling
	{name: "Tailwind CSS", category: "styling", confidence: "observed",
		css:  []*regexp.Regexp{re(`--tw-`)},
		srcs: []*regexp.Regexp{re(`(?i)tailwind`)}},
	{name: "Bootstrap", category: "styling", confidence: "observed",
		srcs: []*regexp.Regexp{re(`(?i)bootstrap(\.min)?\.(css|js)`)}},
	// Infrastructure
	{name: "Vercel", category: "hosting", confidence: "observed",
		headers: []headerRule{{"x-vercel-id", re(`.`)}, {"server", re(`(?i)^Vercel$`)}}},
	{name: "Netlify", category: "hosting", confidence: "observed",
		headers: []headerRule{{"x-nf-request-id", re(`.`)}, {"server", re(`(?i)^Netlify$`)}}},
	{name: "Cloudflare", category: "cdn", confidence: "observed",
		headers: []headerRule{{"cf-ray", re(`.`)}, {"server", re(`(?i)cloudflare`)}}},
	{name: "GitHub Pages", category: "hosting", confidence: "observed",
		headers: []headerRule{{"server", re(`(?i)^GitHub\.com$`)}}},
	// Libraries / services
	{name: "jQuery", category: "library", confidence: "observed",
		srcs: []*regexp.Regexp{re(`(?i)jquery[-.]`)}},
	{name: "Google Tag/Analytics", category: "analytics", confidence: "observed",
		body: []*regexp.Regexp{re(`googletagmanager\.com`), re(`www\.google-analytics\.com`), re(`\bG-[A-Z0-9]{6,}\b`)}},
	{name: "Plausible Analytics", category: "analytics", confidence: "observed",
		body: []*regexp.Regexp{re(`plausible\.io/js`)}},
	{name: "Stripe.js", category: "payments", confidence: "observed",
		body: []*regexp.Regexp{re(`js\.stripe\.com`)}},
	{name: "Sentry", category: "monitoring", confidence: "observed",
		body: []*regexp.Regexp{re(`sentry-cdn`), re(`@sentry/`)}},
	{name: "reCAPTCHA", category: "security", confidence: "observed",
		body: []*regexp.Regexp{re(`google\.com/recaptcha`)}},
	{name: "Vite (bundler)", category: "build", confidence: "inferred",
		body: []*regexp.Regexp{re(`type="module"[^>]*src="/assets/[A-Za-z0-9_.-]+\.js"`)}},
}

// detectStack evaluates every rule against the fetched pages and merges
// evidence per finding.
func detectStack(pages []pageData, base *url.URL) []Finding {
	var findings []Finding
	seen := map[string]int{}
	add := func(rule stackRule, ev Evidence) {
		if i, ok := seen[rule.name]; ok {
			f := &findings[i]
			for _, e := range f.Evidence {
				if e.URL == ev.URL && e.Detail == ev.Detail {
					return
				}
			}
			if len(f.Evidence) < 6 {
				f.Evidence = append(f.Evidence, ev)
			}
			// observed beats inferred
			if rule.confidence == "observed" {
				f.Confidence = "observed"
			}
			return
		}
		seen[rule.name] = len(findings)
		findings = append(findings, Finding{
			Name: rule.name, Category: rule.category,
			Confidence: rule.confidence, Evidence: []Evidence{ev},
		})
	}

	for _, pd := range pages {
		for _, rule := range stackRules {
			page := pd.URL
			if pd.Error != "" {
				continue
			}
			if rule.generator != nil && pd.htmlInfo != nil && pd.htmlInfo.generator != "" &&
				rule.generator.MatchString(pd.htmlInfo.generator) {
				add(rule, Evidence{URL: page, Detail: "meta generator: " + pd.htmlInfo.generator})
			}
			for _, hr := range rule.headers {
				if pd.headers == nil {
					continue
				}
				if v := headerString(pd.headers, hr.key); v != "" && hr.re.MatchString(v) {
					add(rule, Evidence{URL: page, Detail: "header " + hr.key + ": " + v})
				}
			}
			if pd.htmlInfo == nil {
				continue
			}
			for _, pattern := range rule.body {
				if pattern.Match(pd.body) {
					add(rule, Evidence{URL: page, Detail: "HTML matches /" + pattern.String() + "/"})
					break
				}
			}
			srcs := append(append([]string{}, pd.htmlInfo.scripts...), pd.htmlInfo.stylesheetURLs...)
			for _, pattern := range rule.srcs {
				for _, s := range srcs {
					if pattern.MatchString(s) {
						add(rule, Evidence{URL: page, Detail: "asset URL " + s})
						break
					}
				}
			}
			for _, pattern := range rule.css {
				for _, body := range pd.htmlInfo.styleBodies {
					if pattern.MatchString(body) {
						add(rule, Evidence{URL: page, Detail: "inline CSS matches /" + pattern.String() + "/"})
						break
					}
				}
			}
		}
	}
	return findings
}

var apiPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"graphql", re(`(?i)/graphql\b`)},
	{"api", re(`(?i)(^|//|\.)api\.|/api/`)},
	{"docs", re(`(?i)/(docs?|redoc|swagger|openapi)(/|\.|$)`)},
}

// detectAPISurface collects URLs that look like programmatic surfaces from
// every link and asset reference. These are inferred candidates.
func detectAPISurface(pages []pageData, base *url.URL) []APIHit {
	seen := map[string]bool{}
	var out []APIHit
	consider := func(raw, source string) {
		u, err := url.Parse(raw)
		if err != nil {
			return
		}
		s := u.String()
		for _, p := range apiPatterns {
			if p.re.MatchString(s) && !seen[s] {
				seen[s] = true
				out = append(out, APIHit{URL: s, Kind: p.kind, Source: source})
			}
		}
	}
	for _, pd := range pages {
		if pd.htmlInfo == nil {
			continue
		}
		for _, l := range pd.htmlInfo.links {
			consider(l, pd.URL)
		}
		for _, s := range pd.htmlInfo.scripts {
			consider(s, pd.URL)
		}
	}
	if len(out) > 25 {
		out = out[:25]
	}
	return out
}
