package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadConfig_Defaults(t *testing.T) {
	// Work in an empty temp dir so no config file is found
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Reset Configs to zero value before test
	Configs = Config{}

	err = readConfig()
	if err != nil {
		t.Fatalf("unexpected error reading config with no file: %s", err)
	}

	if Configs.UpdateInterval != 120 {
		t.Errorf("expected default UpdateInterval=120, got %d", Configs.UpdateInterval)
	}
	if Configs.ReposPath != "repos" {
		t.Errorf("expected default ReposPath='repos', got %q", Configs.ReposPath)
	}
	if Configs.AutoRotate != true {
		t.Errorf("expected default AutoRotate=true, got %v", Configs.AutoRotate)
	}
	if Configs.SopsSecretsDiscovery != false {
		t.Errorf("expected default SopsSecretsDiscovery=false, got %v", Configs.SopsSecretsDiscovery)
	}
	if Configs.Address != "0.0.0.0:8080" {
		t.Errorf("expected default Address='0.0.0.0:8080', got %q", Configs.Address)
	}
}

func TestReadConfig_FromFile(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	configContent := `
update_interval: 60
repos_path: /custom/repos
auto_rotate: false
sops_secrets_discovery: true
address: "127.0.0.1:9090"
`
	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644)

	Configs = Config{}

	err = readConfig()
	if err != nil {
		t.Fatalf("unexpected error reading config file: %s", err)
	}

	if Configs.UpdateInterval != 60 {
		t.Errorf("expected UpdateInterval=60, got %d", Configs.UpdateInterval)
	}
	if Configs.ReposPath != "/custom/repos" {
		t.Errorf("expected ReposPath='/custom/repos', got %q", Configs.ReposPath)
	}
	if Configs.AutoRotate != false {
		t.Errorf("expected AutoRotate=false, got %v", Configs.AutoRotate)
	}
	if Configs.SopsSecretsDiscovery != true {
		t.Errorf("expected SopsSecretsDiscovery=true, got %v", Configs.SopsSecretsDiscovery)
	}
	if Configs.Address != "127.0.0.1:9090" {
		t.Errorf("expected Address='127.0.0.1:9090', got %q", Configs.Address)
	}
}

func TestReadConfig_PartialOverrides(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Only override some values; the rest should use defaults
	configContent := `
update_interval: 30
`
	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644)

	Configs = Config{}

	err = readConfig()
	if err != nil {
		t.Fatalf("unexpected error reading config file: %s", err)
	}

	if Configs.UpdateInterval != 30 {
		t.Errorf("expected UpdateInterval=30, got %d", Configs.UpdateInterval)
	}
	// Defaults should still apply for unset values
	if Configs.ReposPath != "repos" {
		t.Errorf("expected default ReposPath='repos', got %q", Configs.ReposPath)
	}
	if Configs.AutoRotate != true {
		t.Errorf("expected default AutoRotate=true, got %v", Configs.AutoRotate)
	}
	if Configs.Address != "0.0.0.0:8080" {
		t.Errorf("expected default Address='0.0.0.0:8080', got %q", Configs.Address)
	}
}

func TestReadRepoConfigs(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	reposContent := `
my-repo:
  url: "https://github.com/example/repo.git"
  username: "user1"
  password: "pass1"
another-repo:
  url: "https://github.com/example/other.git"
`
	os.WriteFile(filepath.Join(tmpDir, "repos.yaml"), []byte(reposContent), 0644)

	Configs = Config{}

	err = readRepoConfigs()
	if err != nil {
		t.Fatalf("unexpected error reading repos config: %s", err)
	}

	if Configs.RepoConfigs == nil {
		t.Fatal("expected RepoConfigs to be non-nil")
	}
	if len(Configs.RepoConfigs) != 2 {
		t.Fatalf("expected 2 repo configs, got %d", len(Configs.RepoConfigs))
	}

	myRepo, ok := Configs.RepoConfigs["my-repo"]
	if !ok {
		t.Fatal("expected 'my-repo' in RepoConfigs")
	}
	if myRepo.Url != "https://github.com/example/repo.git" {
		t.Errorf("expected url 'https://github.com/example/repo.git', got %q", myRepo.Url)
	}
	if myRepo.Username != "user1" {
		t.Errorf("expected username 'user1', got %q", myRepo.Username)
	}
	if myRepo.Password != "pass1" {
		t.Errorf("expected password 'pass1', got %q", myRepo.Password)
	}

	anotherRepo, ok := Configs.RepoConfigs["another-repo"]
	if !ok {
		t.Fatal("expected 'another-repo' in RepoConfigs")
	}
	if anotherRepo.Url != "https://github.com/example/other.git" {
		t.Errorf("expected url 'https://github.com/example/other.git', got %q", anotherRepo.Url)
	}
	if anotherRepo.Username != "" {
		t.Errorf("expected empty username, got %q", anotherRepo.Username)
	}
}

func TestReadRepoConfigs_WithPasswordFile(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	reposContent := `
my-repo:
  url: "https://github.com/example/repo.git"
  username: "user1"
  password_file: "/path/to/password"
`
	os.WriteFile(filepath.Join(tmpDir, "repos.yaml"), []byte(reposContent), 0644)

	Configs = Config{}

	err = readRepoConfigs()
	if err != nil {
		t.Fatalf("unexpected error reading repos config: %s", err)
	}

	myRepo := Configs.RepoConfigs["my-repo"]
	if myRepo.PasswordFile != "/path/to/password" {
		t.Errorf("expected password_file '/path/to/password', got %q", myRepo.PasswordFile)
	}
}

func TestReadRepoConfigs_MissingFile(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	Configs = Config{}

	err = readRepoConfigs()
	if err == nil {
		t.Error("expected error when repos config file is missing, got nil")
	}
}

func TestReadStackConfigs(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	stacksContent := `
nginx:
  repo: my-repo
  branch: main
  compose_file: nginx/compose.yaml
  sops_files:
    - nginx/secrets/tls.crt
    - nginx/secrets/tls.key
  sops_secrets_discovery: false
webapp:
  repo: my-repo
  branch: develop
  compose_file: webapp/compose.yaml
  values_file: webapp/values.yaml
  sops_secrets_discovery: true
`
	os.WriteFile(filepath.Join(tmpDir, "stacks.yaml"), []byte(stacksContent), 0644)

	Configs = Config{}

	err = readStackConfigs()
	if err != nil {
		t.Fatalf("unexpected error reading stack configs: %s", err)
	}

	if Configs.StackConfigs == nil {
		t.Fatal("expected StackConfigs to be non-nil")
	}
	if len(Configs.StackConfigs) != 2 {
		t.Fatalf("expected 2 stack configs, got %d", len(Configs.StackConfigs))
	}

	nginx, ok := Configs.StackConfigs["nginx"]
	if !ok {
		t.Fatal("expected 'nginx' in StackConfigs")
	}
	if nginx.Repo != "my-repo" {
		t.Errorf("expected repo 'my-repo', got %q", nginx.Repo)
	}
	if nginx.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", nginx.Branch)
	}
	if nginx.ComposeFile != "nginx/compose.yaml" {
		t.Errorf("expected compose_file 'nginx/compose.yaml', got %q", nginx.ComposeFile)
	}
	if len(nginx.SopsFiles) != 2 {
		t.Fatalf("expected 2 sops_files, got %d", len(nginx.SopsFiles))
	}
	if nginx.SopsFiles[0] != "nginx/secrets/tls.crt" {
		t.Errorf("expected first sops_file 'nginx/secrets/tls.crt', got %q", nginx.SopsFiles[0])
	}
	if nginx.SopsFiles[1] != "nginx/secrets/tls.key" {
		t.Errorf("expected second sops_file 'nginx/secrets/tls.key', got %q", nginx.SopsFiles[1])
	}
	if nginx.SopsSecretsDiscovery != false {
		t.Errorf("expected sops_secrets_discovery=false, got %v", nginx.SopsSecretsDiscovery)
	}

	webapp, ok := Configs.StackConfigs["webapp"]
	if !ok {
		t.Fatal("expected 'webapp' in StackConfigs")
	}
	if webapp.Branch != "develop" {
		t.Errorf("expected branch 'develop', got %q", webapp.Branch)
	}
	if webapp.ComposeFile != "webapp/compose.yaml" {
		t.Errorf("expected compose_file 'webapp/compose.yaml', got %q", webapp.ComposeFile)
	}
	if webapp.ValuesFile != "webapp/values.yaml" {
		t.Errorf("expected values_file 'webapp/values.yaml', got %q", webapp.ValuesFile)
	}
	if webapp.SopsSecretsDiscovery != true {
		t.Errorf("expected sops_secrets_discovery=true, got %v", webapp.SopsSecretsDiscovery)
	}
}

func TestReadStackConfigs_MissingFile(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	Configs = Config{}

	err = readStackConfigs()
	if err == nil {
		t.Error("expected error when stacks config file is missing, got nil")
	}
}

func TestReadStackConfigs_WithEnvFiles(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	stacksContent := `
mystack:
  repo: my-repo
  branch: main
  compose_file: compose.yaml
  env_files:
    - path: defaults.env
    - path: prod.env
      repo: env-repo
      branch: production
`
	os.WriteFile(filepath.Join(tmpDir, "stacks.yaml"), []byte(stacksContent), 0644)

	Configs = Config{}

	err = readStackConfigs()
	if err != nil {
		t.Fatalf("unexpected error reading stack configs: %s", err)
	}

	mystack := Configs.StackConfigs["mystack"]
	if mystack == nil {
		t.Fatal("expected 'mystack' in StackConfigs")
	}
	if len(mystack.EnvFiles) != 2 {
		t.Fatalf("expected 2 env_files, got %d", len(mystack.EnvFiles))
	}
	if mystack.EnvFiles[0].Path != "defaults.env" {
		t.Errorf("expected first env file path 'defaults.env', got %q", mystack.EnvFiles[0].Path)
	}
	if mystack.EnvFiles[0].Repo != "" {
		t.Errorf("expected first env file repo to be empty, got %q", mystack.EnvFiles[0].Repo)
	}
	if mystack.EnvFiles[1].Path != "prod.env" {
		t.Errorf("expected second env file path 'prod.env', got %q", mystack.EnvFiles[1].Path)
	}
	if mystack.EnvFiles[1].Repo != "env-repo" {
		t.Errorf("expected second env file repo 'env-repo', got %q", mystack.EnvFiles[1].Repo)
	}
	if mystack.EnvFiles[1].Branch != "production" {
		t.Errorf("expected second env file branch 'production', got %q", mystack.EnvFiles[1].Branch)
	}
}

func TestLoadConfigs_InlineStacksAndRepos(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// A config file that embeds both repos and stacks inline
	configContent := `
update_interval: 45
repos_path: /data/repos
auto_rotate: true
address: "0.0.0.0:3000"
repos:
  inline-repo:
    url: "https://github.com/example/inline.git"
stacks:
  inline-stack:
    repo: inline-repo
    branch: main
    compose_file: compose.yaml
`
	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644)

	Configs = Config{}

	err = LoadConfigs()
	if err != nil {
		t.Fatalf("unexpected error loading configs: %s", err)
	}

	if Configs.UpdateInterval != 45 {
		t.Errorf("expected UpdateInterval=45, got %d", Configs.UpdateInterval)
	}
	if Configs.Address != "0.0.0.0:3000" {
		t.Errorf("expected Address='0.0.0.0:3000', got %q", Configs.Address)
	}
	if Configs.RepoConfigs == nil {
		t.Fatal("expected RepoConfigs to be non-nil")
	}
	if _, ok := Configs.RepoConfigs["inline-repo"]; !ok {
		t.Error("expected 'inline-repo' in RepoConfigs")
	}
	if Configs.StackConfigs == nil {
		t.Fatal("expected StackConfigs to be non-nil")
	}
	if _, ok := Configs.StackConfigs["inline-stack"]; !ok {
		t.Error("expected 'inline-stack' in StackConfigs")
	}
}

func TestLoadConfigs_SeparateFiles(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %s", err)
	}
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// No inline repos/stacks in config → LoadConfigs should read repos.yaml and stacks.yaml
	configContent := `
update_interval: 90
`
	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644)

	reposContent := `
file-repo:
  url: "https://github.com/example/file.git"
`
	os.WriteFile(filepath.Join(tmpDir, "repos.yaml"), []byte(reposContent), 0644)

	stacksContent := `
file-stack:
  repo: file-repo
  branch: main
  compose_file: compose.yaml
`
	os.WriteFile(filepath.Join(tmpDir, "stacks.yaml"), []byte(stacksContent), 0644)

	Configs = Config{}

	err = LoadConfigs()
	if err != nil {
		t.Fatalf("unexpected error loading configs: %s", err)
	}

	if Configs.UpdateInterval != 90 {
		t.Errorf("expected UpdateInterval=90, got %d", Configs.UpdateInterval)
	}
	if _, ok := Configs.RepoConfigs["file-repo"]; !ok {
		t.Error("expected 'file-repo' in RepoConfigs")
	}
	if _, ok := Configs.StackConfigs["file-stack"]; !ok {
		t.Error("expected 'file-stack' in StackConfigs")
	}
}
