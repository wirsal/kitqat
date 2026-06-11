# KitQA

QA productivity tool for BRISurf — download test cases from Confluence to Excel and sync Jira ticket links back to Confluence after CBAgnito upload.

---

## Features

### 1. Download TC to Excel
Fetch test cases from a Confluence Scenario Detail page and export to a formatted `.xlsx` file ready to upload to CBAgnito.

**Output columns:** No · Project · Issue Type · Test ID · Summary · Description · Assignee · Test Repository Path · Expected Result · Manual Test Steps (Action / Data / Expected Result × 3) · Test Plan · Test Execution · Labels

### 2. Update Scenario Detail
After uploading the Excel to CBAgnito (which creates Jira tickets), automatically inject the Jira ticket links into the "No. Test Case" cells of the original Confluence page.

---

## Usage

Download the latest binary from [Releases](../../releases) and run it directly — no installation required.

```
KitQA_windows_x64.exe
```

### Navigation
| Key | Action |
|---|---|
| `j` / `k` or `↑` / `↓` | Navigate menu |
| `Enter` | Select / confirm |
| `Tab` / `Shift+Tab` | Move between fields |
| `Esc` | Back |
| `q` | Quit |

---

## Workflow

```
Confluence Page
      │
      │  (1) Download TC to Excel
      ▼
  test_cases.xlsx
      │
      │  (upload manually to CBAgnito)
      ▼
  CBAgnito Job  ──►  Jira Tickets created
      │
      │  (2) Update Scenario Detail
      ▼
Confluence Page  (No. Test Case cells now have Jira links)
```

---

## Build from Source

Requires Go 1.24+.

```bash
# Run locally
go run ./cmd/main.go

# Build for Windows x64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/KitQA_windows_x64.exe ./cmd/main.go
```

---

## Project Structure

```
cmd/main.go
internal/
  domain/          # TestCase model
  usecase/         # Business logic (download, update)
  adapter/
    confluence/    # Confluence REST API client + HTML parser
    jira/          # Jira REST API client
    cbagnito/      # CBAgnito portal client (Flask SSR)
    excel/         # XLSX exporter (excelize)
  tui/             # Terminal UI (Bubble Tea + lipgloss)
```

---

## Requirements

- Confluence, Jira, and CBAgnito credentials (same username/password)
- Network access to internal BRISurf services
- Windows x64 (pre-built binary) or Go 1.24+ (build from source)
