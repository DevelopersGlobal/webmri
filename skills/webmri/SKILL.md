---
name: webmri
description: >-
  Read what a website publicly serves in seconds: its HTTP-visible stack,
  design tokens, routes, forms and programmatic surface, with evidence for
  every claim.
  Use when the user asks how a site or web app is built, what powers it,
  what its design system is, what routes or endpoints it has, or wants a
  technical anatomy of a URL before rebuilding or competing with it.
---

# webmri

Run the tool, read the JSON, answer from the evidence. Never invent findings
the report does not contain.

## Setup

```bash
git clone https://github.com/DevelopersGlobal/webmri.git
cd webmri && go build -o webmri .
```

Requires Go 1.23+. The binary is self-contained; run it as `./webmri` from
the repo directory or move it onto your PATH.

## Workflow

1. Scan with JSON output:

   ```bash
   ./webmri scan <url> --out /tmp/anatomy.md --json /tmp/anatomy.json
   ```

   Defaults (25 pages, 8 parallel fetches) fit most sites. Raise `--pages`
   for big sites; use `--no-crawl` for a single-page check.

2. Read `/tmp/anatomy.json`. Every finding carries `confidence`
   (`observed` or `inferred`) and an `evidence` list naming the URL and the
   exact signal. Prefer observed findings; present inferred ones as
   candidates.

3. Answer from the report only. If a section is empty, say the signal was
   not found — do not guess.

## Rules

- Public pages only. webmri never logs in, executes JavaScript, or reads
  private APIs, and neither should you when using its output.
- Cite the evidence entries when a claim matters.
- The report's "Method and limits" section is part of the answer: static
  analysis can miss client-rendered content. Say so when it matters.
