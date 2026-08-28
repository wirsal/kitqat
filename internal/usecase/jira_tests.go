package usecase

import "strings"

// testAssoc is one entry of a Test Execution's "Tests association" custom field.
type testAssoc struct {
	Key   string // e.g. "SPBRISURF-10912"
	RunID int    // Xray test run id, used to set the run status
}

// extractTests pulls the {testKey, testRunId} entries from a Test Execution's
// "Tests association" custom field — found by shape so the exact custom-field id
// need not be hard-coded.
func extractTests(fields map[string]interface{}) []testAssoc {
	for k, v := range fields {
		if !strings.HasPrefix(k, "customfield_") {
			continue
		}
		arr, ok := v.([]interface{})
		if !ok || len(arr) == 0 {
			continue
		}
		out := make([]testAssoc, 0, len(arr))
		valid := true
		for _, it := range arr {
			m, ok := it.(map[string]interface{})
			if !ok {
				valid = false
				break
			}
			tk, ok := m["testKey"].(string)
			if !ok || tk == "" {
				valid = false
				break
			}
			run := 0
			if rid, ok := m["testRunId"].(float64); ok {
				run = int(rid)
			}
			out = append(out, testAssoc{Key: tk, RunID: run})
		}
		if valid && len(out) > 0 {
			return out
		}
	}
	return nil
}

// extractTestKeys returns just the test keys from a Test Execution's tests field.
func extractTestKeys(fields map[string]interface{}) []string {
	tests := extractTests(fields)
	keys := make([]string, 0, len(tests))
	for _, t := range tests {
		keys = append(keys, t.Key)
	}
	return keys
}
