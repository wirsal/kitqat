package api

import "github.com/wirsal/kitqa/internal/usecase"

// downloadRequest mirrors the UC1 form fields in internal/tui/model.go.
type downloadRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	PageURL  string `json:"pageUrl"`
	RepoPath string `json:"repoPath"`
	Project  string `json:"project"`
}

// updateScenarioRequest mirrors the UC2 form fields in internal/tui/model.go.
type updateScenarioRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	PageURL  string `json:"pageUrl"`
	JobURL   string `json:"jobUrl"`
}

type updateScenarioResponse struct {
	Result *usecase.UpdateResult `json:"result"`
}

// setDoneRequest mirrors the UC3 form fields in internal/tui/model.go.
type setDoneRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	JobURL   string `json:"jobUrl"`
}

type setDoneResponse struct {
	Result *usecase.SetDoneResult `json:"result"`
}

type errorResponse struct {
	Error string `json:"error"`
}
