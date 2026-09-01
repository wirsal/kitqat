# KitQAt

Internal QA productivity tool for the **BRISurf** team. A single terminal app (TUI)
that automates the repetitive Confluence / Jira / Xray steps around UAT test-case
documentation — from generating scenarios to exporting test cases, wiring Jira links
back into Confluence, and closing out a Test Execution.

> ⚠️ **Internal use only.** KitQAt talks to BRI's internal Confluence, Jira/Xray and
> CBAgnito services and must be run from inside the BRI network (VPN). It is not a
> general-purpose tool.

---

## Features

The menu is ordered to match the real QA workflow:

### 0. Fill Scenario & Function
Read a spreadsheet (`.xlsx` / `.csv`) and fill the **Scenario, Function, Input Data,
Steps and Expected Result** rows of the TC tables on a Confluence Scenario Detail
page — creating tables for test cases the page doesn't have yet. Never overwrites
cells that already have content. Runs before step 1.

### 1. Download TC to Excel
Fetch test cases from a Confluence Scenario Detail page and export a formatted
`.xlsx` ready to upload to **CBAgnito**. An optional **Test Execution** value can be
stamped into every row.

**Output columns:** No · Project · Issue Type · Test ID · Summary · Description ·
Assignee · Test Repository Path · Expected Result · Manual Test Steps (Action / Data /
Expected Result × 3) · Test Plan · Test Execution · Labels

### 2. Update Scenario Detail
Inject the Jira test-issue links into the **"No. Test Case"** cells of a Confluence
page. The tests are read from a **Jira Test Execution** (e.g. `BRSF-16163`) and matched
to each TC table by comparing the test **summary** to the page's Scenario text — so a
mismatched execution writes nothing.

### 3. Add Test Execution Table
Insert a **Jira filter table** (`issue in testExecutionTests("BRSF-16163")`) at the top
of a Scenario Detail page, showing Key / Summary / Status / TestRunStatus. Idempotent.

### 4. Mark Tickets as Done
For every test in a Jira Test Execution: set the **Xray test run status to PASS** and
transition the **test issue** to done; then transition the **Test Execution issue
itself**. The done-transition prefers `CLOSED`, then `SELESAI` / `Done` / `Resolved`.

---

## Usage

Download the binary for your OS from [Releases](../../releases) and run it directly —
no installation required.

| OS | Binary |
|---|---|
| Windows x64 | `KitQAt_windows_x64.exe` |
| macOS (Apple Silicon) | `KitQAt_mac_arm64` |
| macOS (Intel) | `KitQAt_mac_x64` |
| Linux x64 | `KitQAt_linux_x64` |

On macOS/Linux, mark it executable first: `chmod +x KitQAt_mac_arm64`.

### Navigation
| Key | Action |
|---|---|
| `j` / `k` or `↑` / `↓` | Navigate menu |
| `Enter` | Select / confirm |
| `Tab` / `Shift+Tab` | Move between fields |
| `Esc` | Back |
| `q` | Quit |

### Credentials
Use your BRI corporate username/password for Confluence and Jira. CBAgnito upload
(the manual step between 1 and 2) uses your **NIP** as the username.

---

## Workflow

```
Confluence Scenario Detail page
      │  (0) Fill Scenario & Function      [from a spreadsheet]
      │  (1) Download TC to Excel
      ▼
  test_cases.xlsx  ──(upload manually to CBAgnito)──►  Jira test issues + Test Execution
      │
      │  (2) Update Scenario Detail        [inject Jira links, matched from the Test Execution]
      │  (3) Add Test Execution Table      [insert the Jira filter table]
      │  (4) Mark Tickets as Done          [set PASS + close via Xray]
      ▼
Confluence page complete · Jira/Xray updated
```

---

## Build from Source

Requires Go 1.24+.

```bash
# Run locally
go run ./cmd/main.go

# Cross-compile (static; version stamped via -ldflags)
LD="-X github.com/wirsal/kitqat/internal/version.Version=v1.4.0"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$LD" -o bin/windows/KitQAt_windows_x64.exe ./cmd/main.go
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$LD" -o bin/mac/KitQAt_mac_arm64        ./cmd/main.go
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$LD" -o bin/linux/KitQAt_linux_x64      ./cmd/main.go
```

### Releases
Official binaries are built by **GitHub Actions** on a tag push — the version is taken
from the tag name:

```bash
git tag v1.4.1 && git push origin v1.4.1
```

---

## Project Structure

```
cmd/main.go                 # entrypoint (TUI; `serve` for the HTTP API)
internal/
  domain/                   # TestCase / ScenarioRow models
  usecase/                  # download, update, fill, add-test-exec, set-done
  adapter/
    confluence/             # REST client + storage-format parser / injectors
    jira/                   # Jira + Xray REST client
    sheet/                  # spreadsheet source for Fill Scenario
    excel/                  # XLSX exporter (excelize)
    cbagnito/               # CBAgnito portal client (legacy)
  tui/                      # Terminal UI (Bubble Tea + lipgloss)
  version/                  # version string (set via -ldflags)
```

---

## Requirements

- BRI corporate Confluence / Jira credentials (Xray for Test Executions)
- Network access to internal BRISurf services (VPN)
- A pre-built binary for your OS, or Go 1.24+ to build from source
