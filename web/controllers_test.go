package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m-adawi/swarm-cd/swarmcd"
)

func TestGetStacks_Empty(t *testing.T) {
	// Ensure no stacks are registered so the endpoint returns an empty list
	status := swarmcd.GetStackStatus()
	for k := range status {
		delete(status, k)
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/stacks", nil)
	if err != nil {
		t.Fatalf("could not create request: %s", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("could not unmarshal response: %s", err)
	}
	if result != nil && len(result) != 0 {
		t.Errorf("expected empty list, got %v", result)
	}
}

func TestGetStacks_SingleStack(t *testing.T) {
	status := swarmcd.GetStackStatus()
	for k := range status {
		delete(status, k)
	}
	status["my-stack"] = &swarmcd.StackStatus{
		Error:    "",
		Revision: "abc12345",
		RepoURL:  "https://github.com/example/repo.git",
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/stacks", nil)
	if err != nil {
		t.Fatalf("could not create request: %s", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("could not unmarshal response: %s", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(result))
	}
	if result[0]["Name"] != "my-stack" {
		t.Errorf("expected Name='my-stack', got %q", result[0]["Name"])
	}
	if result[0]["Revision"] != "abc12345" {
		t.Errorf("expected Revision='abc12345', got %q", result[0]["Revision"])
	}
	if result[0]["RepoURL"] != "https://github.com/example/repo.git" {
		t.Errorf("expected RepoURL='https://github.com/example/repo.git', got %q", result[0]["RepoURL"])
	}
	if result[0]["Error"] != "" {
		t.Errorf("expected empty Error, got %q", result[0]["Error"])
	}
}

func TestGetStacks_WithError(t *testing.T) {
	status := swarmcd.GetStackStatus()
	for k := range status {
		delete(status, k)
	}
	status["broken-stack"] = &swarmcd.StackStatus{
		Error:    "could not pull branch main",
		Revision: "",
		RepoURL:  "https://github.com/example/broken.git",
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/stacks", nil)
	if err != nil {
		t.Fatalf("could not create request: %s", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("could not unmarshal response: %s", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(result))
	}
	if result[0]["Error"] != "could not pull branch main" {
		t.Errorf("expected Error='could not pull branch main', got %q", result[0]["Error"])
	}
}

func TestGetStacks_MultipleStacksSortedByName(t *testing.T) {
	status := swarmcd.GetStackStatus()
	for k := range status {
		delete(status, k)
	}
	status["zebra"] = &swarmcd.StackStatus{Revision: "aaa", RepoURL: "https://example.com/zebra.git"}
	status["alpha"] = &swarmcd.StackStatus{Revision: "bbb", RepoURL: "https://example.com/alpha.git"}
	status["middle"] = &swarmcd.StackStatus{Revision: "ccc", RepoURL: "https://example.com/middle.git"}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/stacks", nil)
	if err != nil {
		t.Fatalf("could not create request: %s", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result []map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("could not unmarshal response: %s", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 stacks, got %d", len(result))
	}
	if result[0]["Name"] != "alpha" {
		t.Errorf("expected first stack 'alpha', got %q", result[0]["Name"])
	}
	if result[1]["Name"] != "middle" {
		t.Errorf("expected second stack 'middle', got %q", result[1]["Name"])
	}
	if result[2]["Name"] != "zebra" {
		t.Errorf("expected third stack 'zebra', got %q", result[2]["Name"])
	}
}

func TestGetStacks_ResponseContentType(t *testing.T) {
	status := swarmcd.GetStackStatus()
	for k := range status {
		delete(status, k)
	}
	status["test"] = &swarmcd.StackStatus{Revision: "def456", RepoURL: "https://example.com/test.git"}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/stacks", nil)
	if err != nil {
		t.Fatalf("could not create request: %s", err)
	}
	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type 'application/json; charset=utf-8', got %q", contentType)
	}
}

func TestGetStacks_AllFieldsPresent(t *testing.T) {
	status := swarmcd.GetStackStatus()
	for k := range status {
		delete(status, k)
	}
	status["full-stack"] = &swarmcd.StackStatus{
		Error:    "some error",
		Revision: "rev123",
		RepoURL:  "https://github.com/example/full.git",
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/stacks", nil)
	if err != nil {
		t.Fatalf("could not create request: %s", err)
	}
	router.ServeHTTP(w, req)

	var result []map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("could not unmarshal response: %s", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(result))
	}

	expectedKeys := []string{"Name", "Error", "Revision", "RepoURL"}
	for _, key := range expectedKeys {
		if _, ok := result[0][key]; !ok {
			t.Errorf("expected key %q in response, but it was missing", key)
		}
	}
}

func TestRootRedirect(t *testing.T) {
	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("could not create request: %s", err)
	}
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/ui" {
		t.Errorf("expected redirect to '/ui', got %q", location)
	}
}
