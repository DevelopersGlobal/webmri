# webmri

**An MRI for any web app.** One command shows what is inside: the stack it runs on, the design tokens it uses, its routes, forms and programmatic surface — and the evidence behind every claim. Seconds, no setup, no API keys.

```bash
webmri scan https://tailwindcss.com
```

```
webmri: 12 pages in 1235 ms, 5 stack findings
```

## What you get

A Markdown (and JSON) anatomy of the target:

- **Stack** — frameworks, CMSs, hosting, CDNs, analytics, payment and monitoring services. Each finding is labelled `observed` (directly seen in a response) or `inferred` (a weaker pattern suggested it), with the URL and the exact signal as evidence.
- **Design tokens** — CSS custom properties from `:root`, most-used colors, font families, from inline styles and real stylesheets.
- **Routes** — every same-host page found via the sitemap and link crawling, capped and polite.
- **Forms** — method, target and fields of every HTML form, with the page it lives on.
- **Programmatic surface** — links and assets that look like APIs, GraphQL endpoints or API docs (inferred candidates to investigate, not confirmed endpoints).
- **Performance snapshot** — per-page fetch times: fastest, slowest, median, total bytes.

## Install

Requires [Go 1.23+](https://go.dev/dl/) to build. The result is a single static binary with one module dependency (`golang.org/x/net`).

```bash
go install github.com/DevelopersGlobal/webmri@latest
```

Or build from source:

```bash
git clone https://github.com/DevelopersGlobal/webmri.git
cd webmri
go build -o webmri .
```

## Quick start

```bash
# Scan a site (default: up to 25 pages, 8 parallel fetches)
webmri scan https://example.com

# Save the report, and get machine-readable JSON too
webmri scan https://example.com --out report.md --json report.json

# Just the landing page
webmri scan https://example.com --no-crawl

# Tune the crawl
webmri scan https://example.com --pages 100 --concurrency 16 --timeout 8s
```

Flags after the URL work too. `webmri --h` lists everything.

## Sample output

Real excerpt from `webmri scan https://tailwindcss.com --pages 12`:

```markdown
| name | category | confidence | evidence |
| --- | --- | --- | --- |
| Next.js | framework | observed | header x-powered-by: Next.js — https://tailwindcss.com |
| Tailwind CSS | styling | observed | asset URL .../css/... — https://tailwindcss.com |
| Vercel | hosting | observed | header x-vercel-id: sfo1::... — https://tailwindcss.com |
| Cloudflare | cdn | observed | header cf-ray: a537...-PDX — https://tailwindcss.com |

CSS custom properties (`:root`): 80 found, from --color-brand-primary to --font-plex-mono
Forms: POST https://app.convertkit.com/... on /blog — fields: email_address (email)
```

## Measured speed

Wall-clock times from real runs on 2026-11-01 (default flags unless noted). These are this tool's own measured numbers — not a comparison with anything else.

| target | pages | wall time |
| --- | --- | --- |
| example.com | 1 | 73 ms |
| go.dev | 12 | 777 ms (1.1 MB fetched) |
| tailwindcss.com | 12 | 1.2 s |
| synthetic 200-page local site | 200 | 26 ms (16 workers) |

The whole anatomy costs one pass: every fetched page feeds stack detection, design tokens, routes, forms and API-surface extraction at once, and fetches run in parallel.

## Use it with an AI agent

`skills/webmri/SKILL.md` is a ready-made agent skill: drop it into your agent's skills folder and the agent can run webmri, read the JSON and explain or rebuild what it finds. The JSON output is stable and self-describing, so any tool can consume it without scraping the Markdown.

## Honest limits

- Static analysis of public pages only. No JavaScript execution, no logins, no private APIs, no binary analysis.
- Fingerprints are heuristic. `observed` means the pattern was directly seen; it still is not a guarantee. Evidence is always attached so you can check.
- Crawling is deliberately polite and bounded: same host only, capped page count, capped stylesheet and body sizes, your user-agent is identifiable by default.
- Sites rendered entirely client-side will show thin routes; their JS bundle contents are out of scope.

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues are labelled `good first issue`.

## License

[MIT](LICENSE) — free for anyone, for anything.
