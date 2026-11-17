package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestBacklogCommands(t *testing.T) {
	cfg := setConfigEnv(t)
	var requests []string
	var lastMethod, lastPath, lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		lastMethod = r.Method
		lastPath = r.URL.Path
		lastBody = string(bodyBytes)
		appendPrimaryRequest(&requests, r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/demo":
			_, _ = io.WriteString(w, `{"project":{"id":"demo","name":"demo","path":"/tmp/demo"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/demo/backlog":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"item":{"id":"bl-1","projectId":"demo","orchestratorSessionId":"orch-1","title":"Add billing webhooks","summary":"Discuss retry handling","prompt":"Implement webhook retries","status":"open"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/demo/backlog":
			_, _ = io.WriteString(w, `{"items":[{"id":"bl-1","projectId":"demo","title":"Add billing webhooks","summary":"Discuss retry handling","status":"open"}]}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/backlog/bl-1":
			_, _ = io.WriteString(w, `{"item":{"id":"bl-1","projectId":"demo","title":"Updated title","summary":"Discuss retry handling","status":"open"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/backlog/bl-1/implement":
			_, _ = io.WriteString(w, `{"item":{"id":"bl-1","projectId":"demo","title":"Updated title","status":"implemented","spawnedSessionId":"demo-1"},"sessionId":"demo-1"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/backlog/bl-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	writeRunFileFor(t, cfg, srv)
	t.Setenv("AO_SESSION_ID", "orch-1")

	out, errOut, err := executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }},
		"backlog", "add", "--project", "demo", "--title", "Add billing webhooks", "--summary", "Discuss retry handling", "--prompt", "Implement webhook retries")
	if err != nil {
		t.Fatalf("backlog add failed: %v\nstderr=%s", err, errOut)
	}
	if !strings.Contains(out, "bl-1") || !strings.Contains(out, "Add billing webhooks") {
		t.Fatalf("unexpected add output: %q", out)
	}
	var addBody backlogCreateDTO
	if err := json.Unmarshal([]byte(lastBody), &addBody); err != nil {
		t.Fatalf("decode add body: %v", err)
	}
	if addBody.OrchestratorSessionID != "orch-1" || addBody.Title != "Add billing webhooks" || addBody.Prompt != "Implement webhook retries" {
		t.Fatalf("unexpected add payload: %+v", addBody)
	}

	out, errOut, err = executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }},
		"backlog", "ls", "--project", "demo")
	if err != nil {
		t.Fatalf("backlog ls failed: %v\nstderr=%s", err, errOut)
	}
	if !strings.Contains(out, "bl-1") || !strings.Contains(out, "Add billing webhooks") {
		t.Fatalf("unexpected ls output: %q", out)
	}

	out, errOut, err = executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }},
		"backlog", "update", "bl-1", "--title", "Updated title")
	if err != nil {
		t.Fatalf("backlog update failed: %v\nstderr=%s", err, errOut)
	}
	if !strings.Contains(out, "Updated title") {
		t.Fatalf("unexpected update output: %q", out)
	}

	out, errOut, err = executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }},
		"backlog", "implement", "bl-1", "--harness", "codex", "--mode", "chat")
	if err != nil {
		t.Fatalf("backlog implement failed: %v\nstderr=%s", err, errOut)
	}
	if !strings.Contains(out, "demo-1") {
		t.Fatalf("unexpected implement output: %q", out)
	}
	var implBody backlogImplementDTO
	if err := json.Unmarshal([]byte(lastBody), &implBody); err != nil {
		t.Fatalf("decode implement body: %v", err)
	}
	if implBody.Harness != "codex" || implBody.InterfaceMode != "chat" {
		t.Fatalf("unexpected implement payload: %+v", implBody)
	}

	out, errOut, err = executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }},
		"backlog", "rm", "bl-1")
	if err != nil {
		t.Fatalf("backlog rm failed: %v\nstderr=%s", err, errOut)
	}
	if lastMethod != http.MethodDelete || lastPath != "/api/v1/backlog/bl-1" || !strings.Contains(out, "deleted backlog item bl-1") {
		t.Fatalf("unexpected delete result: method=%s path=%s out=%q", lastMethod, lastPath, out)
	}

	want := []string{
		"GET /api/v1/projects/demo",
		"POST /api/v1/projects/demo/backlog",
		"GET /api/v1/projects/demo",
		"GET /api/v1/projects/demo/backlog",
		"PATCH /api/v1/backlog/bl-1",
		"POST /api/v1/backlog/bl-1/implement",
		"DELETE /api/v1/backlog/bl-1",
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}

func TestBacklogAddRequiresTitle(t *testing.T) {
	_, _, err := executeCLI(t, Deps{}, "backlog", "add", "--project", "demo")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("err=%v exit=%d, want usage error", err, ExitCode(err))
	}
}
