package swarmcd

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// External objects are ignored by the rotation
func TestRotateExternalObjects(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "docker-compose.yaml", nil, "", nil, false)
	objects := map[string]any{
		"my-secret": map[string]any{"external": true},
	}
	err := stack.rotateObjects(objects, "secrets")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

// Secrets are discovered, external secrets are ignored
func TestSecretDiscovery(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "stacks/docker-compose.yaml", nil, "", nil, false)
	stackString := []byte(`services:
  my-service:
    image: my-image
    secrets:
      - my-secret
      - my-external-secret
secrets:
  my-secret:
    file: secrets/secret.yaml
  my-external-secret:
    external: true`)
	composeMap, err := stack.parseStackString(stackString)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	sopsFiles, err := discoverSecrets(composeMap, stack.composePath)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(sopsFiles) != 1 {
		t.Errorf("unexpected number of sops files: %d", len(sopsFiles))
	}
	if sopsFiles[0] != "stacks/secrets/secret.yaml" {
		t.Errorf("unexpected sops file: %s", sopsFiles[0])
	}
}

// --- env file parsing tests ---

func TestParseEnvFile_Basic(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "test.env")
	content := `# This is a comment
IMAGE_TAG=1.5.2
REPLICAS=3
LOG_LEVEL=info
`
	os.WriteFile(envPath, []byte(content), 0644)

	envMap, err := parseEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(envMap) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(envMap))
	}
	if envMap["IMAGE_TAG"] != "1.5.2" {
		t.Errorf("expected IMAGE_TAG=1.5.2, got %s", envMap["IMAGE_TAG"])
	}
	if envMap["REPLICAS"] != "3" {
		t.Errorf("expected REPLICAS=3, got %s", envMap["REPLICAS"])
	}
	if envMap["LOG_LEVEL"] != "info" {
		t.Errorf("expected LOG_LEVEL=info, got %s", envMap["LOG_LEVEL"])
	}
}

func TestParseEnvFile_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "test.env")
	content := `DOUBLE_QUOTED="hello world"
SINGLE_QUOTED='foo bar'
UNQUOTED=plain
`
	os.WriteFile(envPath, []byte(content), 0644)

	envMap, err := parseEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if envMap["DOUBLE_QUOTED"] != "hello world" {
		t.Errorf("expected 'hello world', got %s", envMap["DOUBLE_QUOTED"])
	}
	if envMap["SINGLE_QUOTED"] != "foo bar" {
		t.Errorf("expected 'foo bar', got %s", envMap["SINGLE_QUOTED"])
	}
	if envMap["UNQUOTED"] != "plain" {
		t.Errorf("expected 'plain', got %s", envMap["UNQUOTED"])
	}
}

func TestParseEnvFile_ExportPrefix(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "test.env")
	content := `export MY_VAR=exported_value
export OTHER_VAR="quoted exported"
`
	os.WriteFile(envPath, []byte(content), 0644)

	envMap, err := parseEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if envMap["MY_VAR"] != "exported_value" {
		t.Errorf("expected 'exported_value', got %s", envMap["MY_VAR"])
	}
	if envMap["OTHER_VAR"] != "quoted exported" {
		t.Errorf("expected 'quoted exported', got %s", envMap["OTHER_VAR"])
	}
}

func TestParseEnvFile_EmptyLinesAndComments(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "test.env")
	content := `
# comment 1
KEY1=val1

# comment 2

KEY2=val2
`
	os.WriteFile(envPath, []byte(content), 0644)

	envMap, err := parseEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(envMap) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(envMap))
	}
}

func TestParseEnvFile_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "test.env")
	content := `EMPTY_VAR=
`
	os.WriteFile(envPath, []byte(content), 0644)

	envMap, err := parseEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if val, ok := envMap["EMPTY_VAR"]; !ok || val != "" {
		t.Errorf("expected EMPTY_VAR to be empty string, got '%s' (ok=%v)", val, ok)
	}
}

func TestParseEnvFile_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "test.env")
	content := `DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require
`
	os.WriteFile(envPath, []byte(content), 0644)

	envMap, err := parseEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if envMap["DATABASE_URL"] != "postgres://user:pass@host:5432/db?sslmode=require" {
		t.Errorf("unexpected value: %s", envMap["DATABASE_URL"])
	}
}

func TestParseEnvFile_InvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "test.env")
	content := `VALID=ok
INVALID_LINE
`
	os.WriteFile(envPath, []byte(content), 0644)

	_, err := parseEnvFile(envPath)
	if err == nil {
		t.Error("expected error for invalid line, got nil")
	}
}

func TestParseEnvFile_FileNotFound(t *testing.T) {
	_, err := parseEnvFile("/nonexistent/path/to/env")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// --- env var substitution tests ---

func TestSubstituteEnvVars_BasicVar(t *testing.T) {
	envMap := map[string]string{
		"IMAGE_TAG": "1.5.2",
		"REPLICAS":  "3",
	}
	input := []byte("image: myapp:${IMAGE_TAG}\nreplicas: ${REPLICAS}")
	result := substituteEnvVars(input, envMap)
	expected := "image: myapp:1.5.2\nreplicas: 3"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestSubstituteEnvVars_BareVar(t *testing.T) {
	envMap := map[string]string{
		"TAG": "latest",
	}
	input := []byte("image: myapp:$TAG")
	result := substituteEnvVars(input, envMap)
	expected := "image: myapp:latest"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestSubstituteEnvVars_DefaultColonDash(t *testing.T) {
	envMap := map[string]string{
		"SET_VAR": "value",
	}
	// SET_VAR is set → use its value; UNSET_VAR is not → use default
	input := []byte("a: ${SET_VAR:-fallback}\nb: ${UNSET_VAR:-fallback}")
	result := substituteEnvVars(input, envMap)
	expected := "a: value\nb: fallback"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestSubstituteEnvVars_DefaultColonDashEmptyValue(t *testing.T) {
	envMap := map[string]string{
		"EMPTY_VAR": "",
	}
	// EMPTY_VAR is set but empty → ":-" treats it as unset → use default
	input := []byte("val: ${EMPTY_VAR:-default_val}")
	result := substituteEnvVars(input, envMap)
	expected := "val: default_val"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestSubstituteEnvVars_DefaultDash(t *testing.T) {
	envMap := map[string]string{
		"EMPTY_VAR": "",
	}
	// EMPTY_VAR is set (even if empty) → "-" returns its value
	input := []byte("a: ${EMPTY_VAR-default_val}\nb: ${MISSING-default_val}")
	result := substituteEnvVars(input, envMap)
	expected := "a: \nb: default_val"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestSubstituteEnvVars_EscapedDollar(t *testing.T) {
	envMap := map[string]string{}
	input := []byte("command: echo $$HOME")
	result := substituteEnvVars(input, envMap)
	expected := "command: echo $HOME"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestSubstituteEnvVars_UnsetVarBecomesEmpty(t *testing.T) {
	envMap := map[string]string{}
	input := []byte("image: myapp:${UNSET_TAG}")
	result := substituteEnvVars(input, envMap)
	expected := "image: myapp:"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestSubstituteEnvVars_MixedSyntax(t *testing.T) {
	envMap := map[string]string{
		"TAG":       "v2",
		"PORT":      "9090",
		"LOG_LEVEL": "warn",
	}
	input := []byte(`services:
  web:
    image: myapp:${TAG}
    ports:
      - "$PORT:8080"
    environment:
      - LOG_LEVEL=${LOG_LEVEL:-info}
      - DEBUG=${DEBUG:-false}
`)
	result := substituteEnvVars(input, envMap)
	expected := `services:
  web:
    image: myapp:v2
    ports:
      - "9090:8080"
    environment:
      - LOG_LEVEL=warn
      - DEBUG=false
`
	if string(result) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(result))
	}
}

func TestSubstituteEnvVars_NoVars(t *testing.T) {
	envMap := map[string]string{"FOO": "bar"}
	input := []byte("no variables here")
	result := substituteEnvVars(input, envMap)
	if string(result) != "no variables here" {
		t.Errorf("expected unchanged content, got %q", string(result))
	}
}

func TestSubstituteEnvVars_EmptyMap(t *testing.T) {
	envMap := map[string]string{}
	input := []byte("image: myapp:${TAG:-latest}")
	result := substituteEnvVars(input, envMap)
	expected := "image: myapp:latest"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

// --- parseStackString tests ---

func TestParseStackString_ValidYAML(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	input := []byte(`services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
`)
	result, err := stack.parseStackString(input)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	services, ok := result["services"].(map[string]any)
	if !ok {
		t.Fatal("expected 'services' key in parsed output")
	}
	web, ok := services["web"].(map[string]any)
	if !ok {
		t.Fatal("expected 'web' key in services")
	}
	if web["image"] != "nginx:latest" {
		t.Errorf("expected image 'nginx:latest', got %v", web["image"])
	}
}

func TestParseStackString_InvalidYAML(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	input := []byte(`services:
  web:
    image: nginx:latest
    ports:
  - this is invalid
    indentation: broken
`)
	_, err := stack.parseStackString(input)
	if err == nil {
		t.Error("expected error parsing invalid YAML, got nil")
	}
}

func TestParseStackString_EmptyInput(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	input := []byte("")
	result, err := stack.parseStackString(input)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if result != nil {
		t.Errorf("expected nil map for empty input, got %v", result)
	}
}

func TestParseStackString_ComplexCompose(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	input := []byte(`version: "3.8"
services:
  web:
    image: nginx:latest
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: "0.5"
          memory: 128M
  db:
    image: postgres:15
    volumes:
      - db-data:/var/lib/postgresql/data
volumes:
  db-data:
configs:
  my-config:
    file: ./config.txt
secrets:
  my-secret:
    file: ./secret.txt
`)
	result, err := stack.parseStackString(input)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, ok := result["services"]; !ok {
		t.Error("expected 'services' key")
	}
	if _, ok := result["volumes"]; !ok {
		t.Error("expected 'volumes' key")
	}
	if _, ok := result["configs"]; !ok {
		t.Error("expected 'configs' key")
	}
	if _, ok := result["secrets"]; !ok {
		t.Error("expected 'secrets' key")
	}
}

// --- readStack / writeStack tests ---

func TestReadStack(t *testing.T) {
	tmpDir := t.TempDir()
	composeContent := `services:
  web:
    image: nginx:latest
`
	composePath := "docker-compose.yaml"
	os.WriteFile(filepath.Join(tmpDir, composePath), []byte(composeContent), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", composePath, nil, "", nil, false)

	result, err := stack.readStack()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(result) != composeContent {
		t.Errorf("expected %q, got %q", composeContent, string(result))
	}
}

func TestReadStack_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "nonexistent.yaml", nil, "", nil, false)

	_, err := stack.readStack()
	if err == nil {
		t.Error("expected error reading nonexistent file, got nil")
	}
}

func TestReadStack_SubdirectoryCompose(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myapp")
	os.MkdirAll(subDir, 0755)
	composeContent := `services:
  api:
    image: myapp:v1
`
	os.WriteFile(filepath.Join(subDir, "compose.yaml"), []byte(composeContent), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "myapp/compose.yaml", nil, "", nil, false)

	result, err := stack.readStack()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(result) != composeContent {
		t.Errorf("expected %q, got %q", composeContent, string(result))
	}
}

func TestWriteStack(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := "docker-compose.yaml"
	originalContent := `services:
  web:
    image: nginx:latest
`
	composeFile := filepath.Join(tmpDir, composePath)
	os.WriteFile(composeFile, []byte(originalContent), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", composePath, nil, "", nil, false)

	composeMap := map[string]any{
		"services": map[string]any{
			"web": map[string]any{
				"image": "nginx:1.25",
			},
		},
	}

	err := stack.writeStack(composeMap)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	written, err := os.ReadFile(composeFile)
	if err != nil {
		t.Fatalf("could not read written file: %s", err)
	}
	if !strings.Contains(string(written), "nginx:1.25") {
		t.Errorf("expected written file to contain 'nginx:1.25', got:\n%s", string(written))
	}
}

func TestWriteStack_PreservesFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := "docker-compose.yaml"
	composeFile := filepath.Join(tmpDir, composePath)
	os.WriteFile(composeFile, []byte("services: {}"), 0600)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", composePath, nil, "", nil, false)

	err := stack.writeStack(map[string]any{"services": map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	info, err := os.Stat(composeFile)
	if err != nil {
		t.Fatalf("could not stat written file: %s", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected file permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestWriteAndReadRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := "docker-compose.yaml"
	composeFile := filepath.Join(tmpDir, composePath)
	os.WriteFile(composeFile, []byte("placeholder: true"), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", composePath, nil, "", nil, false)

	original := map[string]any{
		"services": map[string]any{
			"web": map[string]any{
				"image": "myapp:v2",
				"ports": []any{"8080:80"},
			},
		},
	}

	err := stack.writeStack(original)
	if err != nil {
		t.Fatalf("unexpected error writing: %s", err)
	}

	readBytes, err := stack.readStack()
	if err != nil {
		t.Fatalf("unexpected error reading: %s", err)
	}

	parsed, err := stack.parseStackString(readBytes)
	if err != nil {
		t.Fatalf("unexpected error parsing: %s", err)
	}

	services, ok := parsed["services"].(map[string]any)
	if !ok {
		t.Fatal("expected 'services' in parsed output")
	}
	web, ok := services["web"].(map[string]any)
	if !ok {
		t.Fatal("expected 'web' in services")
	}
	if web["image"] != "myapp:v2" {
		t.Errorf("expected image 'myapp:v2', got %v", web["image"])
	}
}

// --- renderComposeTemplate tests ---

func TestRenderComposeTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	valuesContent := `image_tag: "1.5.2"
replicas: 3
`
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(valuesContent), 0644)

	templateContent := []byte(`services:
  web:
    image: myapp:{{ .Values.image_tag }}
    deploy:
      replicas: {{ .Values.replicas }}
`)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "compose.yaml", nil, "values.yaml", nil, false)

	result, err := stack.renderComposeTemplate(templateContent)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "myapp:1.5.2") {
		t.Errorf("expected rendered output to contain 'myapp:1.5.2', got:\n%s", resultStr)
	}
	if !strings.Contains(resultStr, "replicas: 3") {
		t.Errorf("expected rendered output to contain 'replicas: 3', got:\n%s", resultStr)
	}
}

func TestRenderComposeTemplate_MissingValuesFile(t *testing.T) {
	tmpDir := t.TempDir()

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "compose.yaml", nil, "nonexistent-values.yaml", nil, false)

	_, err := stack.renderComposeTemplate([]byte("image: {{ .Values.tag }}"))
	if err == nil {
		t.Error("expected error when values file is missing, got nil")
	}
}

func TestRenderComposeTemplate_InvalidTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("key: val"), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "compose.yaml", nil, "values.yaml", nil, false)

	_, err := stack.renderComposeTemplate([]byte("image: {{ .Values.tag }"))
	if err == nil {
		t.Error("expected error for invalid Go template syntax, got nil")
	}
}

func TestRenderComposeTemplate_NestedValues(t *testing.T) {
	tmpDir := t.TempDir()

	valuesContent := `web:
  image: nginx
  tag: "1.25"
db:
  image: postgres
  tag: "15"
`
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(valuesContent), 0644)

	templateContent := []byte(`services:
  web:
    image: {{ .Values.web.image }}:{{ .Values.web.tag }}
  db:
    image: {{ .Values.db.image }}:{{ .Values.db.tag }}
`)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "compose.yaml", nil, "values.yaml", nil, false)

	result, err := stack.renderComposeTemplate(templateContent)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "nginx:1.25") {
		t.Errorf("expected 'nginx:1.25' in output, got:\n%s", resultStr)
	}
	if !strings.Contains(resultStr, "postgres:15") {
		t.Errorf("expected 'postgres:15' in output, got:\n%s", resultStr)
	}
}

// --- rotateObjects tests with real files and MD5 hashing ---

func TestRotateObjects_WithRealFiles(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := "server.port=8080\nserver.host=0.0.0.0\n"
	os.WriteFile(filepath.Join(tmpDir, "config.txt"), []byte(configContent), 0644)

	expectedHash := fmt.Sprintf("%x", md5.Sum([]byte(configContent)))[:8]

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	objects := map[string]any{
		"app-config": map[string]any{
			"file": "config.txt",
		},
	}

	err := stack.rotateObjects(objects, "configs")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	configObj := objects["app-config"].(map[string]any)
	expectedName := "mystack-app-config-" + expectedHash
	if configObj["name"] != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, configObj["name"])
	}
}

func TestRotateObjects_MultipleObjects(t *testing.T) {
	tmpDir := t.TempDir()

	config1 := "config1-content"
	config2 := "config2-content"
	os.WriteFile(filepath.Join(tmpDir, "config1.txt"), []byte(config1), 0644)
	os.WriteFile(filepath.Join(tmpDir, "config2.txt"), []byte(config2), 0644)

	hash1 := fmt.Sprintf("%x", md5.Sum([]byte(config1)))[:8]
	hash2 := fmt.Sprintf("%x", md5.Sum([]byte(config2)))[:8]

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	objects := map[string]any{
		"cfg1": map[string]any{"file": "config1.txt"},
		"cfg2": map[string]any{"file": "config2.txt"},
	}

	err := stack.rotateObjects(objects, "configs")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	obj1 := objects["cfg1"].(map[string]any)
	if obj1["name"] != "mystack-cfg1-"+hash1 {
		t.Errorf("expected name 'mystack-cfg1-%s', got %q", hash1, obj1["name"])
	}
	obj2 := objects["cfg2"].(map[string]any)
	if obj2["name"] != "mystack-cfg2-"+hash2 {
		t.Errorf("expected name 'mystack-cfg2-%s', got %q", hash2, obj2["name"])
	}
}

func TestRotateObjects_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	objects := map[string]any{
		"missing-config": map[string]any{"file": "nonexistent.txt"},
	}

	err := stack.rotateObjects(objects, "configs")
	if err == nil {
		t.Error("expected error when config file does not exist, got nil")
	}
}

func TestRotateObjects_InvalidObjectFormat(t *testing.T) {
	tmpDir := t.TempDir()

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	// Object is a string instead of a map
	objects := map[string]any{
		"bad-config": "not-a-map",
	}

	err := stack.rotateObjects(objects, "configs")
	if err == nil {
		t.Error("expected error for invalid object format, got nil")
	}
}

func TestRotateObjects_MissingFileField(t *testing.T) {
	tmpDir := t.TempDir()

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	// Object map has no "file" field
	objects := map[string]any{
		"no-file-config": map[string]any{"driver": "some-driver"},
	}

	err := stack.rotateObjects(objects, "configs")
	if err == nil {
		t.Error("expected error when 'file' field is missing, got nil")
	}
}

func TestRotateObjects_ContentChangeChangesHash(t *testing.T) {
	tmpDir := t.TempDir()

	content1 := "version1"
	os.WriteFile(filepath.Join(tmpDir, "config.txt"), []byte(content1), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	objects1 := map[string]any{
		"cfg": map[string]any{"file": "config.txt"},
	}
	err := stack.rotateObjects(objects1, "configs")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	name1 := objects1["cfg"].(map[string]any)["name"].(string)

	// Change content and rotate again
	content2 := "version2"
	os.WriteFile(filepath.Join(tmpDir, "config.txt"), []byte(content2), 0644)

	objects2 := map[string]any{
		"cfg": map[string]any{"file": "config.txt"},
	}
	err = stack.rotateObjects(objects2, "configs")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	name2 := objects2["cfg"].(map[string]any)["name"].(string)

	if name1 == name2 {
		t.Errorf("expected different names for different content, both got %q", name1)
	}
}

func TestRotateObjects_SameContentSameHash(t *testing.T) {
	tmpDir := t.TempDir()

	content := "same-content"
	os.WriteFile(filepath.Join(tmpDir, "config.txt"), []byte(content), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	objects1 := map[string]any{
		"cfg": map[string]any{"file": "config.txt"},
	}
	stack.rotateObjects(objects1, "configs")
	name1 := objects1["cfg"].(map[string]any)["name"].(string)

	objects2 := map[string]any{
		"cfg": map[string]any{"file": "config.txt"},
	}
	stack.rotateObjects(objects2, "configs")
	name2 := objects2["cfg"].(map[string]any)["name"].(string)

	if name1 != name2 {
		t.Errorf("expected same names for same content, got %q and %q", name1, name2)
	}
}

func TestRotateObjects_SubdirectoryCompose(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myapp")
	os.MkdirAll(subDir, 0755)

	configContent := "my-config-data"
	os.WriteFile(filepath.Join(subDir, "config.txt"), []byte(configContent), 0644)

	expectedHash := fmt.Sprintf("%x", md5.Sum([]byte(configContent)))[:8]

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	// composePath is in a subdirectory — file paths are relative to compose dir
	stack := newSwarmStack("mystack", repo, "main", "myapp/docker-compose.yaml", nil, "", nil, false)

	objects := map[string]any{
		"cfg": map[string]any{"file": "config.txt"},
	}

	err := stack.rotateObjects(objects, "configs")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	obj := objects["cfg"].(map[string]any)
	expectedName := "mystack-cfg-" + expectedHash
	if obj["name"] != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, obj["name"])
	}
}

// --- rotateConfigsAndSecrets tests ---

func TestRotateConfigsAndSecrets_BothTypes(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "app.conf"), []byte("config-data"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "tls.key"), []byte("secret-key-data"), 0644)

	configHash := fmt.Sprintf("%x", md5.Sum([]byte("config-data")))[:8]
	secretHash := fmt.Sprintf("%x", md5.Sum([]byte("secret-key-data")))[:8]

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	composeMap := map[string]any{
		"configs": map[string]any{
			"app-config": map[string]any{"file": "app.conf"},
		},
		"secrets": map[string]any{
			"tls-key": map[string]any{"file": "tls.key"},
		},
	}

	err := stack.rotateConfigsAndSecrets(composeMap)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	configObj := composeMap["configs"].(map[string]any)["app-config"].(map[string]any)
	if configObj["name"] != "mystack-app-config-"+configHash {
		t.Errorf("unexpected config name: %q", configObj["name"])
	}
	secretObj := composeMap["secrets"].(map[string]any)["tls-key"].(map[string]any)
	if secretObj["name"] != "mystack-tls-key-"+secretHash {
		t.Errorf("unexpected secret name: %q", secretObj["name"])
	}
}

func TestRotateConfigsAndSecrets_NoConfigsOrSecrets(t *testing.T) {
	tmpDir := t.TempDir()

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	composeMap := map[string]any{
		"services": map[string]any{
			"web": map[string]any{"image": "nginx"},
		},
	}

	err := stack.rotateConfigsAndSecrets(composeMap)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestRotateConfigsAndSecrets_MixedExternalAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "local.conf"), []byte("local-data"), 0644)

	repo := &stackRepo{name: "test", path: tmpDir, url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "main", "docker-compose.yaml", nil, "", nil, false)

	composeMap := map[string]any{
		"configs": map[string]any{
			"local-config":    map[string]any{"file": "local.conf"},
			"external-config": map[string]any{"external": true},
		},
	}

	err := stack.rotateConfigsAndSecrets(composeMap)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	localObj := composeMap["configs"].(map[string]any)["local-config"].(map[string]any)
	if _, ok := localObj["name"]; !ok {
		t.Error("expected 'name' to be set on local config after rotation")
	}
	externalObj := composeMap["configs"].(map[string]any)["external-config"].(map[string]any)
	if _, ok := externalObj["name"]; ok {
		t.Error("expected external config to NOT have a 'name' after rotation")
	}
}

// --- discoverSecrets edge cases ---

func TestDiscoverSecrets_NoSecretsSection(t *testing.T) {
	composeMap := map[string]any{
		"services": map[string]any{
			"web": map[string]any{"image": "nginx"},
		},
	}

	sopsFiles, err := discoverSecrets(composeMap, "docker-compose.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sopsFiles) != 0 {
		t.Errorf("expected 0 sops files, got %d", len(sopsFiles))
	}
}

func TestDiscoverSecrets_AllExternal(t *testing.T) {
	composeMap := map[string]any{
		"secrets": map[string]any{
			"secret1": map[string]any{"external": true},
			"secret2": map[string]any{"external": true},
		},
	}

	sopsFiles, err := discoverSecrets(composeMap, "docker-compose.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sopsFiles) != 0 {
		t.Errorf("expected 0 sops files for all-external secrets, got %d", len(sopsFiles))
	}
}

func TestDiscoverSecrets_MultipleFileSecrets(t *testing.T) {
	composeMap := map[string]any{
		"secrets": map[string]any{
			"cert":    map[string]any{"file": "secrets/tls.crt"},
			"key":     map[string]any{"file": "secrets/tls.key"},
			"ca-cert": map[string]any{"file": "secrets/ca.crt"},
		},
	}

	sopsFiles, err := discoverSecrets(composeMap, "myapp/docker-compose.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sopsFiles) != 3 {
		t.Fatalf("expected 3 sops files, got %d", len(sopsFiles))
	}
	// All paths should be relative to compose dir
	for _, f := range sopsFiles {
		if !strings.HasPrefix(f, "myapp/secrets/") {
			t.Errorf("expected path to start with 'myapp/secrets/', got %q", f)
		}
	}
}

func TestDiscoverSecrets_InvalidSecretFormat(t *testing.T) {
	composeMap := map[string]any{
		"secrets": map[string]any{
			"bad-secret": "not-a-map",
		},
	}

	_, err := discoverSecrets(composeMap, "docker-compose.yaml")
	if err == nil {
		t.Error("expected error for invalid secret format, got nil")
	}
}

func TestDiscoverSecrets_MissingFileField(t *testing.T) {
	composeMap := map[string]any{
		"secrets": map[string]any{
			"bad-secret": map[string]any{"driver": "something"},
		},
	}

	_, err := discoverSecrets(composeMap, "docker-compose.yaml")
	if err == nil {
		t.Error("expected error for secret missing 'file' field, got nil")
	}
}

func TestDiscoverSecrets_ExternalFalse(t *testing.T) {
	// external: false should NOT be treated as external
	composeMap := map[string]any{
		"secrets": map[string]any{
			"local-secret": map[string]any{
				"external": false,
				"file":     "secrets/local.key",
			},
		},
	}

	sopsFiles, err := discoverSecrets(composeMap, "docker-compose.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sopsFiles) != 1 {
		t.Errorf("expected 1 sops file for non-external secret, got %d", len(sopsFiles))
	}
}

func TestDiscoverSecrets_RootComposePath(t *testing.T) {
	// Compose file at repo root — paths should not get extra prefix
	composeMap := map[string]any{
		"secrets": map[string]any{
			"my-secret": map[string]any{"file": "secrets/secret.yaml"},
		},
	}

	sopsFiles, err := discoverSecrets(composeMap, "docker-compose.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sopsFiles) != 1 {
		t.Fatalf("expected 1 sops file, got %d", len(sopsFiles))
	}
	if sopsFiles[0] != "secrets/secret.yaml" {
		t.Errorf("expected 'secrets/secret.yaml', got %q", sopsFiles[0])
	}
}

// --- newSwarmStack tests ---

func TestNewSwarmStack(t *testing.T) {
	repo := &stackRepo{name: "test-repo", path: "/tmp/repo", url: "https://example.com", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	envFiles := []envFileRef{
		{repo: repo, branch: "main", path: "defaults.env"},
	}

	stack := newSwarmStack("my-stack", repo, "develop", "app/compose.yaml", []string{"secrets/a.key"}, "values.yaml", envFiles, true)

	if stack.name != "my-stack" {
		t.Errorf("expected name 'my-stack', got %q", stack.name)
	}
	if stack.repo != repo {
		t.Error("expected repo reference to match")
	}
	if stack.branch != "develop" {
		t.Errorf("expected branch 'develop', got %q", stack.branch)
	}
	if stack.composePath != "app/compose.yaml" {
		t.Errorf("expected composePath 'app/compose.yaml', got %q", stack.composePath)
	}
	if len(stack.sopsFiles) != 1 || stack.sopsFiles[0] != "secrets/a.key" {
		t.Errorf("unexpected sopsFiles: %v", stack.sopsFiles)
	}
	if stack.valuesFile != "values.yaml" {
		t.Errorf("expected valuesFile 'values.yaml', got %q", stack.valuesFile)
	}
	if len(stack.envFiles) != 1 {
		t.Fatalf("expected 1 env file, got %d", len(stack.envFiles))
	}
	if stack.envFiles[0].path != "defaults.env" {
		t.Errorf("expected env file path 'defaults.env', got %q", stack.envFiles[0].path)
	}
	if !stack.discoverSecrets {
		t.Error("expected discoverSecrets=true")
	}
}

// --- integration-style test: parse env file then substitute ---

func TestEnvFileAndSubstitution_Integration(t *testing.T) {
	dir := t.TempDir()

	// Write a defaults env file
	defaultsPath := filepath.Join(dir, "defaults.env")
	os.WriteFile(defaultsPath, []byte(`IMAGE_TAG=latest
REPLICAS=1
LOG_LEVEL=info
`), 0644)

	// Write a production overrides env file
	prodPath := filepath.Join(dir, "prod.env")
	os.WriteFile(prodPath, []byte(`IMAGE_TAG=1.5.2
REPLICAS=3
DATABASE_URL=postgres://prod:5432/app
`), 0644)

	// Parse both, prod overrides defaults
	envMap := map[string]string{}
	defaults, err := parseEnvFile(defaultsPath)
	if err != nil {
		t.Fatalf("unexpected error parsing defaults: %s", err)
	}
	for k, v := range defaults {
		envMap[k] = v
	}
	prod, err := parseEnvFile(prodPath)
	if err != nil {
		t.Fatalf("unexpected error parsing prod: %s", err)
	}
	for k, v := range prod {
		envMap[k] = v
	}

	compose := []byte(`services:
  web:
    image: myapp:${IMAGE_TAG}
    deploy:
      replicas: ${REPLICAS}
    environment:
      - LOG_LEVEL=${LOG_LEVEL}
      - DATABASE_URL=${DATABASE_URL}
      - DEBUG=${DEBUG:-false}
`)
	result := substituteEnvVars(compose, envMap)
	expected := `services:
  web:
    image: myapp:1.5.2
    deploy:
      replicas: 3
    environment:
      - LOG_LEVEL=info
      - DATABASE_URL=postgres://prod:5432/app
      - DEBUG=false
`
	if string(result) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(result))
	}
}
