package api

import (
	"encoding/json"
	"net/http"

	"github.com/wirsal/kitqa/internal/adapter/cbagnito"
	"github.com/wirsal/kitqa/internal/adapter/confluence"
	"github.com/wirsal/kitqa/internal/adapter/excel"
	"github.com/wirsal/kitqa/internal/adapter/jira"
	"github.com/wirsal/kitqa/internal/usecase"
)

const (
	confluenceBaseURL = "https://confluence.bri.co.id"
	jiraBaseURL       = "https://jira.bri.co.id"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

// handleDownload implements POST /api/v1/test-cases/download.
// It mirrors doDownload in internal/tui/model.go, but streams the resulting
// .xlsx as the HTTP response body instead of writing it to disk.
func handleDownload(w http.ResponseWriter, r *http.Request) {
	var req downloadRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" || req.PageURL == "" {
		writeError(w, http.StatusBadRequest, errMissingField("username, password, pageUrl"))
		return
	}

	confluenceClient := confluence.NewClient(confluenceBaseURL, req.Username, req.Password)
	repo := confluence.NewRepository(confluenceClient)
	jiraClient := jira.NewClient(jiraBaseURL, req.Username, req.Password)
	repo.WithJiraClient(jiraClient)
	repo.SetDefaultRepoPath(req.RepoPath)
	repo.SetDefaultProject(req.Project)

	pageID, err := repo.ResolvePageID(req.PageURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	testCases, err := repo.FetchTestCases(pageID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	for i := range testCases {
		testCases[i].No = i + 1
	}

	xlsx, err := excel.NewExporter().ExportBytes(testCases)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="test_cases.xlsx"`)
	w.Write(xlsx)
}

// handleUpdateScenario implements POST /api/v1/scenarios/update.
// It mirrors doUpdateScenario in internal/tui/model.go.
func handleUpdateScenario(w http.ResponseWriter, r *http.Request) {
	var req updateScenarioRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" || req.PageURL == "" || req.JobURL == "" {
		writeError(w, http.StatusBadRequest, errMissingField("username, password, pageUrl, jobUrl"))
		return
	}

	confluenceClient := confluence.NewClient(confluenceBaseURL, req.Username, req.Password)
	repo := confluence.NewRepository(confluenceClient)
	cbClient := cbagnito.NewClient(req.Username, req.Password)

	uc := usecase.NewUpdateScenarioUseCase(repo, cbClient)
	result, err := uc.Execute(req.PageURL, req.JobURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, updateScenarioResponse{Result: result})
}

// handleSetDone implements POST /api/v1/jobs/set-done.
// It mirrors doSetDone in internal/tui/model.go.
func handleSetDone(w http.ResponseWriter, r *http.Request) {
	var req setDoneRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" || req.JobURL == "" {
		writeError(w, http.StatusBadRequest, errMissingField("username, password, jobUrl"))
		return
	}

	jiraClient := jira.NewClient(jiraBaseURL, req.Username, req.Password)
	cbClient := cbagnito.NewClient(req.Username, req.Password)

	uc := usecase.NewSetDoneUseCase(jiraClient, cbClient)
	result, err := uc.Execute(req.JobURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, setDoneResponse{Result: result})
}

type fieldError string

func (e fieldError) Error() string { return "missing required field(s): " + string(e) }

func errMissingField(fields string) error { return fieldError(fields) }
