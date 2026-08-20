# KitQA — Claude Code Context

## Project Overview

KitQA is a Go TUI/CLI tool for the QA team at BRISurf. It has four use cases:

1. **Download TC to Excel** — fetch test cases from a Confluence Scenario Detail page and export to `.xlsx` format ready for upload to CBAgnito.
2. **Update Scenario Detail** — after uploading the Excel to CBAgnito (which creates Jira tickets), inject the resulting Jira ticket links into the "No. Test Case" cells of the original Confluence page.
3. **Mark Tickets as Done** — transition every Jira ticket from a CBAgnito job to Done.
4. **Fill Scenario & Function** (shown as menu item `0`) — read a spreadsheet (`.xlsx`/`.csv`) and fill the Scenario, Function, Input Data, Steps and Expected Result rows of the TC tables on a Confluence page, creating tables for test cases the page doesn't have yet. Runs *before* UC1 in the workflow.

## Module

```
github.com/wirsal/kitqa
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
  domain/testcase.go                — TestCase + ScenarioRow structs (shared models)
  usecase/
    port.go                         — interfaces: ConfluenceRepository, ExcelExporter, ConfluenceUpdater, ScenarioSource
    downloader.go                   — DownloadUseCase
    updater.go                      — UpdateScenarioUseCase
    set_done.go                     — SetDoneUseCase
    fill_scenario.go                — FillScenarioUseCase (UC4)
  adapter/
    confluence/
      client.go                     — HTTP client: GetPage, UpdatePage, ResolvePageID
      parser.go                     — HTML/storage-format parser + Jira link injection
      htmlscan.go                   — balanced element scanner + ValidateStorageBalance
      scenario.go                   — UC4: InjectScenarioFields, BuildTCTable, ExtractTCTableTemplate
      repository.go                 — Repository: FetchTestCases, GetPageRaw, UpdatePageContent
    sheet/
      file_source.go                — ScenarioSource reading local .xlsx/.xlsm/.csv
      header.go                     — header-row detection + column aliases
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
fieldUsername   = 0
fieldPassword   = 1
fieldPageURL    = 2
fieldRepoPath   = 3  (UC1 only)
fieldProject    = 4  (UC1 only)
fieldOutput     = 5  (UC1 only)
fieldJobURL     = 3  (UC2 only — reuses slot 3)
fieldJobURLUC3  = 2  (UC3 only — reuses slot 2, no Page URL)
fieldSourceFile = 3  (UC4 only — reuses slot 3)

fieldCountUC1 = 6
fieldCountUC2 = 4
fieldCountUC3 = 3
fieldCountUC4 = 4
fieldCount    = 6  (max allocation)
```

All use cases share the same `inputs` slice (length 6) and overwrite the slots they need — UC2/UC4 write slot 3, UC3 writes slot 2. Every use case must use `activeFieldCount()` and `activeFieldButton()` instead of a fixed constant. Menu order follows the real workflow, not the use-case numbering: `0. Fill Scenario & Function`, `1. Download TC to Excel`, `2. Update Scenario Detail`, `3. Mark Tickets as Done`, `4. Exit`. UC4 is labelled `0` because it runs before step 1, which keeps the numbers the QA team already knows attached to UC1-UC3.

Each `menuItem` carries the `activeUseCase` it launches (and `isExit` for the last entry), so `updateMenu` reads `m.items[m.cursor].uc` instead of switching on literal indices — reordering or inserting a menu entry needs no other change. `TestMenuSelectionMatchesItem` in `internal/tui/menu_test.go` guards this.

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

## Scenario Field Injection (UC4)

`scenario.go` works at **`<table>` scope**, not `<tr>` scope: a `Scenario | (empty)` row carries no TC identity, so the TC number must first be read from the table's "No. Test Case" row before its other rows can be filled.

- `InjectScenarioFields` — fills empty value cells only. A cell counts as empty when its tag-stripped text is blank or a placeholder (`-`, `–`, `—`, `.`, `n/a`, `na`); any cell containing `<ac:`/`<ri:` (macro, image, Jira link) counts as filled. Values are written via `storageParagraphs`, which HTML-escapes each line and wraps it in `<p>` — spreadsheet content can never inject markup.
- `ExtractTCTableTemplate` — returns the page's **last** TC table, cloned as the template for new test cases so the page's own colgroups/styling/macros are preserved.
- `BuildTCTable` — blanks the clone (`blankTCTable`), then reuses `fillTableFields` so filling and creating share one code path. `sanitizeTemplateCell` strips `ac:image` attachments and empties `ac:rich-text-body` so the cloned TC's screenshots aren't carried over; static template text (Link Defect `-`, `PASS | FAILED`) is kept.

Idempotent: a second run fills nothing and skips the PUT, so no empty page version is created.

### Never use non-greedy regex for element boundaries

Storage format nests: a TC table's Screen Capture cell holds an `expand` macro that can wrap **another table**. A pattern like `<table\b[^>]*>.*?</table>` stops at the *first* `</table>` — the inner one — and returns a TC table cut in half, with `<ac:structured-macro>` left open. Cloning that as a template and PUTting it makes Confluence answer:

```
400 Error parsing xhtml: Unexpected EOF; was expecting a close tag for element <ac:structured-macro>
```

`htmlscan.go` exists for this. Use `topLevelElements(s, "table")` / `("tr")` / `("td", "th")` — a depth-counting scanner that skips comments, CDATA and quoted attributes, returns balanced spans, and leaves untouched bytes byte-identical. Elements of a *nested* table are correctly excluded because they sit inside an outer `<td>`.

`ValidateStorageBalance` checks container nesting (`table`, `tr`, `td`, `ac:structured-macro`, `ac:rich-text-body`, …). UC4 runs it before every PUT and refuses to write markup it has broken — but only when the original page was well-formed, so an already-messy page can still be updated.

Note: `InjectJiraLinks` (UC2) still uses the older `<tr>` regexes. It doesn't corrupt anything, because a mis-matched row fails the label check and is returned unchanged — but port it to `topLevelElements` if it is ever extended.

## Known Behaviours

- Output file path: `"` characters are stripped automatically (Windows copy-paste artefact).
- Project and Output fields are pre-filled with defaults (`SPBRISURF`, `test_cases.xlsx`) as actual values, not placeholders.
- If output path is empty after stripping quotes, defaults back to `test_cases.xlsx`.
- `ExpectedResult` is read from KV keys: `"expected result"`, `"expected"`, `"hasil yang diharapkan"`.
- UC4 spreadsheet input: the header row is auto-detected within the first 10 rows (a row qualifies with ≥2 recognised columns), so a file exported by UC1 — header on rows 1-2, data from row 3 — can be fed straight back in. For `.xlsx` the `Create Test` sheet is preferred, otherwise the first sheet. A missing/blank TC column falls back to row position.
- UC4's `ScenarioSource` interface exists so a Google Sheets source can be added later without touching the parser or use case; only the local-file source is implemented today.
