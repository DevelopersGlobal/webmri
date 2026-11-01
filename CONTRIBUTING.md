# Contributing to webmri

Thanks for helping make webmri better. It stays free and MIT-licensed.

## Ways to help

- **New fingerprint rules** — the highest-value contribution. Each rule lives in `stack.go` and needs a test in `stack_test.go`. Rules must be pattern-based and honest: if the signal is weak, mark the finding `inferred`.
- **Bug reports** — open an issue with the target URL (if public), the command you ran, and what you expected vs saw.
- **New extractors** — routes, forms, design tokens, API surface: same pattern as the existing analyzers, with tests.
- **Docs** — clearer README sections, more examples, translations.

## Ground rules

1. **Evidence or it does not ship.** Every claim webmri prints must name where it was seen. Never add a finding without evidence.
2. **Honesty over coverage.** A missed detection is fine; an invented one is not.
3. **Stay polite.** Same host only, bounded page counts, bounded fetch sizes. No auth bypassing, no JS execution, no scraping of private surfaces.
4. **Tests pass.** `go test ./...` must be green. Add tests for every new rule and extractor.
5. **No new dependencies without discussion.** The tool is one binary with a single module dependency (`golang.org/x/net`). Open an issue before adding another.

## Development

```bash
git clone https://github.com/DevelopersGlobal/webmri.git
cd webmri
go test ./...
go build -o webmri .
./webmri scan https://example.com
```

- Go 1.23+ required.
- Run `gofmt -w .` before committing.
- Keep report sections stable; the JSON shape is consumed by other tools.

## Pull requests

- One change per PR, with a clear title.
- Describe what you tested and against which sites (public ones only).
- If you change output shapes, say so explicitly — it is a breaking change.
