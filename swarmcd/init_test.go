package swarmcd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/m-adawi/swarm-cd/util"
)

func TestCreateHTTPBasicAuth_PublicRepo(t *testing.T) {
	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"public-repo": {
				Url:      "https://github.com/example/public.git",
				Username: "",
				Password: "",
			},
		},
	}

	auth, err := createHTTPBasicAuth("public-repo")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if auth != nil {
		t.Errorf("expected nil auth for public repo, got %+v", auth)
	}
}

func TestCreateHTTPBasicAuth_WithPassword(t *testing.T) {
	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"private-repo": {
				Url:      "https://github.com/example/private.git",
				Username: "user1",
				Password: "secret123",
			},
		},
	}

	auth, err := createHTTPBasicAuth("private-repo")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
	if auth.Username != "user1" {
		t.Errorf("expected username 'user1', got %q", auth.Username)
	}
	if auth.Password != "secret123" {
		t.Errorf("expected password 'secret123', got %q", auth.Password)
	}
}

func TestCreateHTTPBasicAuth_WithPasswordFile(t *testing.T) {
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password.txt")
	err := os.WriteFile(passwordFile, []byte("file-secret\n"), 0600)
	if err != nil {
		t.Fatalf("could not write password file: %s", err)
	}

	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"private-repo": {
				Url:          "https://github.com/example/private.git",
				Username:     "user1",
				PasswordFile: passwordFile,
			},
		},
	}

	auth, err := createHTTPBasicAuth("private-repo")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
	if auth.Username != "user1" {
		t.Errorf("expected username 'user1', got %q", auth.Username)
	}
	if auth.Password != "file-secret" {
		t.Errorf("expected password 'file-secret', got %q", auth.Password)
	}
}

func TestCreateHTTPBasicAuth_PasswordFileTrimmed(t *testing.T) {
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password.txt")
	// Write password with leading/trailing whitespace and newlines
	err := os.WriteFile(passwordFile, []byte("  my-password  \n\n"), 0600)
	if err != nil {
		t.Fatalf("could not write password file: %s", err)
	}

	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"repo": {
				Url:          "https://github.com/example/repo.git",
				Username:     "user",
				PasswordFile: passwordFile,
			},
		},
	}

	auth, err := createHTTPBasicAuth("repo")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if auth.Password != "my-password" {
		t.Errorf("expected trimmed password 'my-password', got %q", auth.Password)
	}
}

func TestCreateHTTPBasicAuth_PasswordTakesPrecedenceOverFile(t *testing.T) {
	tmpDir := t.TempDir()
	passwordFile := filepath.Join(tmpDir, "password.txt")
	os.WriteFile(passwordFile, []byte("file-secret"), 0600)

	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"repo": {
				Url:          "https://github.com/example/repo.git",
				Username:     "user",
				Password:     "inline-secret",
				PasswordFile: passwordFile,
			},
		},
	}

	auth, err := createHTTPBasicAuth("repo")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if auth.Password != "inline-secret" {
		t.Errorf("expected inline password 'inline-secret', got %q", auth.Password)
	}
}

func TestCreateHTTPBasicAuth_MissingUsername(t *testing.T) {
	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"repo": {
				Url:      "https://github.com/example/repo.git",
				Username: "",
				Password: "secret",
			},
		},
	}

	_, err := createHTTPBasicAuth("repo")
	if err == nil {
		t.Error("expected error when username is missing but password is set, got nil")
	}
}

func TestCreateHTTPBasicAuth_MissingPasswordAndPasswordFile(t *testing.T) {
	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"repo": {
				Url:      "https://github.com/example/repo.git",
				Username: "user",
				Password: "",
			},
		},
	}

	_, err := createHTTPBasicAuth("repo")
	if err == nil {
		t.Error("expected error when password and password_file are both missing, got nil")
	}
}

func TestCreateHTTPBasicAuth_PasswordFileNotFound(t *testing.T) {
	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"repo": {
				Url:          "https://github.com/example/repo.git",
				Username:     "user",
				PasswordFile: "/nonexistent/password/file",
			},
		},
	}

	_, err := createHTTPBasicAuth("repo")
	if err == nil {
		t.Error("expected error when password file does not exist, got nil")
	}
}

func TestCreateHTTPBasicAuth_OnlyPasswordFileSet(t *testing.T) {
	// Username is missing, but password_file is set → should error about missing username
	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"repo": {
				Url:          "https://github.com/example/repo.git",
				Username:     "",
				PasswordFile: "/some/file",
			},
		},
	}

	_, err := createHTTPBasicAuth("repo")
	if err == nil {
		t.Error("expected error when username is missing but password_file is set, got nil")
	}
}

func TestCreateHTTPBasicAuth_AllFieldsEmpty(t *testing.T) {
	// All auth fields empty → treated as public repo
	config = &util.Config{
		RepoConfigs: map[string]*util.RepoConfig{
			"repo": {
				Url:          "https://github.com/example/repo.git",
				Username:     "",
				Password:     "",
				PasswordFile: "",
			},
		},
	}

	auth, err := createHTTPBasicAuth("repo")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if auth != nil {
		t.Errorf("expected nil auth for repo with all empty fields, got %+v", auth)
	}
}

func TestInitStacks_Success(t *testing.T) {
	// Set up repos map and config
	testRepo := &stackRepo{
		name: "test-repo",
		path: "/tmp/test-repo",
		url:  "https://github.com/example/test.git",
	}

	// Save and restore global state
	origRepos := repos
	origStacks := stacks
	origStackStatus := stackStatus
	origConfig := config
	defer func() {
		repos = origRepos
		stacks = origStacks
		stackStatus = origStackStatus
		config = origConfig
	}()

	repos = map[string]*stackRepo{
		"test-repo": testRepo,
	}
	stacks = nil
	stackStatus = map[string]*StackStatus{}
	config = &util.Config{
		SopsSecretsDiscovery: false,
		StackConfigs: map[string]*util.StackConfig{
			"my-stack": {
				Repo:        "test-repo",
				Branch:      "main",
				ComposeFile: "compose.yaml",
				SopsFiles:   []string{"secrets/tls.crt"},
			},
		},
	}

	err := initStacks()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(stacks))
	}
	if stacks[0].name != "my-stack" {
		t.Errorf("expected stack name 'my-stack', got %q", stacks[0].name)
	}
	if stacks[0].repo != testRepo {
		t.Error("expected stack repo to match test-repo")
	}
	if stacks[0].branch != "main" {
		t.Errorf("expected branch 'main', got %q", stacks[0].branch)
	}
	if stacks[0].composePath != "compose.yaml" {
		t.Errorf("expected composePath 'compose.yaml', got %q", stacks[0].composePath)
	}
	if len(stacks[0].sopsFiles) != 1 || stacks[0].sopsFiles[0] != "secrets/tls.crt" {
		t.Errorf("unexpected sopsFiles: %v", stacks[0].sopsFiles)
	}

	status, ok := stackStatus["my-stack"]
	if !ok {
		t.Fatal("expected 'my-stack' in stackStatus")
	}
	if status.RepoURL != testRepo.url {
		t.Errorf("expected RepoURL %q, got %q", testRepo.url, status.RepoURL)
	}
}

func TestInitStacks_UnknownRepo(t *testing.T) {
	origRepos := repos
	origStacks := stacks
	origStackStatus := stackStatus
	origConfig := config
	defer func() {
		repos = origRepos
		stacks = origStacks
		stackStatus = origStackStatus
		config = origConfig
	}()

	repos = map[string]*stackRepo{}
	stacks = nil
	stackStatus = map[string]*StackStatus{}
	config = &util.Config{
		StackConfigs: map[string]*util.StackConfig{
			"bad-stack": {
				Repo:        "nonexistent-repo",
				Branch:      "main",
				ComposeFile: "compose.yaml",
			},
		},
	}

	err := initStacks()
	if err == nil {
		t.Error("expected error when stack references unknown repo, got nil")
	}
}

func TestInitStacks_UnknownEnvFileRepo(t *testing.T) {
	testRepo := &stackRepo{
		name: "test-repo",
		path: "/tmp/test-repo",
		url:  "https://github.com/example/test.git",
	}

	origRepos := repos
	origStacks := stacks
	origStackStatus := stackStatus
	origConfig := config
	defer func() {
		repos = origRepos
		stacks = origStacks
		stackStatus = origStackStatus
		config = origConfig
	}()

	repos = map[string]*stackRepo{
		"test-repo": testRepo,
	}
	stacks = nil
	stackStatus = map[string]*StackStatus{}
	config = &util.Config{
		StackConfigs: map[string]*util.StackConfig{
			"env-stack": {
				Repo:        "test-repo",
				Branch:      "main",
				ComposeFile: "compose.yaml",
				EnvFiles: []util.EnvFileConfig{
					{Path: "prod.env", Repo: "missing-env-repo"},
				},
			},
		},
	}

	err := initStacks()
	if err == nil {
		t.Error("expected error when env file references unknown repo, got nil")
	}
}

func TestInitStacks_WithEnvFiles(t *testing.T) {
	testRepo := &stackRepo{
		name: "test-repo",
		path: "/tmp/test-repo",
		url:  "https://github.com/example/test.git",
	}
	envRepo := &stackRepo{
		name: "env-repo",
		path: "/tmp/env-repo",
		url:  "https://github.com/example/env.git",
	}

	origRepos := repos
	origStacks := stacks
	origStackStatus := stackStatus
	origConfig := config
	defer func() {
		repos = origRepos
		stacks = origStacks
		stackStatus = origStackStatus
		config = origConfig
	}()

	repos = map[string]*stackRepo{
		"test-repo": testRepo,
		"env-repo":  envRepo,
	}
	stacks = nil
	stackStatus = map[string]*StackStatus{}
	config = &util.Config{
		StackConfigs: map[string]*util.StackConfig{
			"my-stack": {
				Repo:        "test-repo",
				Branch:      "main",
				ComposeFile: "compose.yaml",
				EnvFiles: []util.EnvFileConfig{
					{Path: "defaults.env"},
					{Path: "prod.env", Repo: "env-repo", Branch: "production"},
				},
			},
		},
	}

	err := initStacks()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(stacks))
	}
	if len(stacks[0].envFiles) != 2 {
		t.Fatalf("expected 2 env files, got %d", len(stacks[0].envFiles))
	}

	// First env file should use the stack's own repo and branch
	if stacks[0].envFiles[0].repo != testRepo {
		t.Error("expected first env file repo to be test-repo")
	}
	if stacks[0].envFiles[0].branch != "main" {
		t.Errorf("expected first env file branch 'main', got %q", stacks[0].envFiles[0].branch)
	}
	if stacks[0].envFiles[0].path != "defaults.env" {
		t.Errorf("expected first env file path 'defaults.env', got %q", stacks[0].envFiles[0].path)
	}

	// Second env file should use the specified repo and branch
	if stacks[0].envFiles[1].repo != envRepo {
		t.Error("expected second env file repo to be env-repo")
	}
	if stacks[0].envFiles[1].branch != "production" {
		t.Errorf("expected second env file branch 'production', got %q", stacks[0].envFiles[1].branch)
	}
	if stacks[0].envFiles[1].path != "prod.env" {
		t.Errorf("expected second env file path 'prod.env', got %q", stacks[0].envFiles[1].path)
	}
}

func TestInitStacks_SopsSecretsDiscoveryGlobal(t *testing.T) {
	testRepo := &stackRepo{
		name: "test-repo",
		path: "/tmp/test-repo",
		url:  "https://github.com/example/test.git",
	}

	origRepos := repos
	origStacks := stacks
	origStackStatus := stackStatus
	origConfig := config
	defer func() {
		repos = origRepos
		stacks = origStacks
		stackStatus = origStackStatus
		config = origConfig
	}()

	repos = map[string]*stackRepo{
		"test-repo": testRepo,
	}
	stacks = nil
	stackStatus = map[string]*StackStatus{}
	config = &util.Config{
		SopsSecretsDiscovery: true,
		StackConfigs: map[string]*util.StackConfig{
			"my-stack": {
				Repo:                 "test-repo",
				Branch:               "main",
				ComposeFile:          "compose.yaml",
				SopsSecretsDiscovery: false, // stack-level is false, but global is true
			},
		},
	}

	err := initStacks()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !stacks[0].discoverSecrets {
		t.Error("expected discoverSecrets=true when global SopsSecretsDiscovery is true")
	}
}

func TestInitStacks_SopsSecretsDiscoveryStackLevel(t *testing.T) {
	testRepo := &stackRepo{
		name: "test-repo",
		path: "/tmp/test-repo",
		url:  "https://github.com/example/test.git",
	}

	origRepos := repos
	origStacks := stacks
	origStackStatus := stackStatus
	origConfig := config
	defer func() {
		repos = origRepos
		stacks = origStacks
		stackStatus = origStackStatus
		config = origConfig
	}()

	repos = map[string]*stackRepo{
		"test-repo": testRepo,
	}
	stacks = nil
	stackStatus = map[string]*StackStatus{}
	config = &util.Config{
		SopsSecretsDiscovery: false,
		StackConfigs: map[string]*util.StackConfig{
			"my-stack": {
				Repo:                 "test-repo",
				Branch:               "main",
				ComposeFile:          "compose.yaml",
				SopsSecretsDiscovery: true,
			},
		},
	}

	err := initStacks()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !stacks[0].discoverSecrets {
		t.Error("expected discoverSecrets=true when stack-level SopsSecretsDiscovery is true")
	}
}
