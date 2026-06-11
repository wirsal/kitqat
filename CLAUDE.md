# KitQA — Claude Code Context

## Project Overview

KitQA is a Go TUI/CLI tool for the QA team at BRISurf. It has two use cases:

1. **Download TC to Excel** — fetch test cases from a Confluence Scenario Detail page and export to `.xlsx` format ready for upload to CBAgnito.
2. **Update Scenario Detail** — after uploading the Excel to CBAgnito (which creates Jira tickets), inject the resulting Jira ticket links into the "No. Test Case" cells of the original Confluence page.

## Module

```
github.com/brisurf/kitqa
```

## Key External Services

| Service | Base URL | Auth |
|---|---|---|
| Confluence | `https://confluence.bri.co.id` | Basic Auth |
| Jira | `https://jira.bri.co.id` | Basic Auth |
| CBAgnito | `http://172.18.249.109:1014` | Flask session + CSRF form login |

## Architecture

```
cmd/main.go                         — entrypoint, starts Bubble Tea app
internal/
  domain/testcase.go                — TestCase struct (shared model)
  usecase/
    port.go                         — interfaces: ConfluenceRepository, ExcelExporter, ConfluenceUpdater
    downloader.go                   — DownloadUseCase
    updater.go                      — UpdateScenarioUseCase
  adapter/
    confluence/
      client.go                     — HTTP client: GetPage, UpdatePage, ResolvePageID
      parser.go                     — HTML/storage-format parser + Jira link injection
      repository.go                 — Repository: FetchTestCases, GetPageRaw, UpdatePageContent
    jira/
      client.go                     — SearchIssues, GetFields
      mapper.go                     — MapIssue (Jira issue → domain.TestCase)
    cbagnito/
      client.go                     — Flask login + HTML table parser for ticket entries
    excel/
      exporter.go                   — excelize-based XLSX generator
  tui/
    model.go                        — Bubble Tea Model, state machine, async commands
    view.go                         — View rendering, form labels, menu descriptions
    styles.go                       — lipgloss styles
```

## Build

```bash
# local
go build ./...

# Windows x64 binary (target platform)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/KitQA_windows_x64.exe ./cmd/main.go
```

Always set `CGO_ENABLED=0` for Windows cross-compilation.

## TUI Field Constants (model.go)

```
fieldUsername = 0
fieldPassword = 1
fieldPageURL  = 2
fieldRepoPath = 3  (UC1 only)
fieldProject  = 4  (UC1 only)
fieldOutput   = 5  (UC1 only)
fieldJobURL   = 3  (UC2 only — reuses slot 3)

fieldCountUC1 = 6
fieldCountUC2 = 4
fieldCount    = 6  (max allocation)
```

UC1 and UC2 share the same `inputs` slice (length 6). UC2 writes `fieldJobURL` into slot 3, overwriting the UC1 Repo Path slot. Both use `activeFieldCount()` and `activeFieldButton()` instead of a fixed constant.

## Confluence Parser — Page Format Support

`parser.go` supports three page formats tried in order:

1. **Vertical two-column table** (`ParseVerticalTestCases`) — Scenario Detail pages. Each TC is a separate table with label/value pairs. TC number derived from "No. Test Case" row; Test ID built as `{codePrefix}_{tcNumber}` where `codePrefix` comes from the page title (e.g. `BRSF02-39` → `BRSF02_39`).
2. **Horizontal table** (`ParseTestCasesFromHTML`) — standard table with header row.
3. **Jira macros** (`ParseJiraMacros` + Jira API) — only when a `jiraClient` is attached.

## CBAgnito Login Flow

CBAgnito is a Flask SSR app (no JSON API). Login requires:
1. GET `/login` → extract `csrf_token` from form HTML
2. POST `/login` with `application/x-www-form-urlencoded` body (`csrf_token`, `username`, `password`, `login`)
3. Use `noRedirectClient` (`CheckRedirect: http.ErrUseLastResponse`) — Flask returns 302 on success, 200 on failure. Do NOT follow the redirect (the chain hits `/index` which is POST-only and returns 405 on GET).

Ticket data is parsed from the HTML table on the job page using `golang.org/x/net/html` DOM walker (not regex — multi-line HTML breaks regex).

## Jira Link Injection

`InjectJiraLinks` in `parser.go` uses regex to find `<tr>` blocks where the first `<td>` contains "No. Test Case", then appends/replaces a Confluence storage-format Jira macro in the second `<td>`. Idempotent: same key → skip, different key → replace.

## Known Behaviours

- Output file path: `"` characters are stripped automatically (Windows copy-paste artefact).
- Project and Output fields are pre-filled with defaults (`SPBRISURF`, `test_cases.xlsx`) as actual values, not placeholders.
- If output path is empty after stripping quotes, defaults back to `test_cases.xlsx`.
- `ExpectedResult` is read from KV keys: `"expected result"`, `"expected"`, `"hasil yang diharapkan"`.
