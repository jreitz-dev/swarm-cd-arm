package swarmcd

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"log/slog"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"

	"github.com/docker/cli/cli/command/stack"
	"github.com/goccy/go-yaml"
	"github.com/m-adawi/swarm-cd/util"
)

// envFileRef is a resolved reference to an env file,
// pointing at a specific repo, branch, and path.
type envFileRef struct {
	repo   *stackRepo
	branch string
	path   string
}

type swarmStack struct {
	name            string
	repo            *stackRepo
	branch          string
	composePath     string
	sopsFiles       []string
	valuesFile      string
	envFiles        []envFileRef
	discoverSecrets bool
}

func newSwarmStack(name string, repo *stackRepo, branch string, composePath string, sopsFiles []string, valuesFile string, envFiles []envFileRef, discoverSecrets bool) *swarmStack {
	return &swarmStack{
		name:            name,
		repo:            repo,
		branch:          branch,
		composePath:     composePath,
		sopsFiles:       sopsFiles,
		valuesFile:      valuesFile,
		envFiles:        envFiles,
		discoverSecrets: discoverSecrets,
	}
}

func (swarmStack *swarmStack) updateStack() (revision string, err error) {
	log := logger.With(
		slog.String("stack", swarmStack.name),
		slog.String("branch", swarmStack.branch),
	)

	log.Debug("pulling changes...")
	revision, err = swarmStack.repo.pullChanges(swarmStack.branch)
	if err != nil {
		return
	}
	log.Debug("changes pulled", "revision", revision)

	log.Debug("reading stack file...")
	stackBytes, err := swarmStack.readStack()
	if err != nil {
		return
	}

	if len(swarmStack.envFiles) > 0 {
		log.Debug("loading env files...")
		envMap, envErr := swarmStack.loadEnvFiles()
		if envErr != nil {
			err = envErr
			return
		}
		if len(envMap) > 0 {
			log.Debug("substituting environment variables...")
			stackBytes = substituteEnvVars(stackBytes, envMap)
		}
	}

	if swarmStack.valuesFile != "" {
		log.Debug("rendering template...")
		stackBytes, err = swarmStack.renderComposeTemplate(stackBytes)
	}
	if err != nil {
		return
	}

	log.Debug("parsing stack content...")
	stackContents, err := swarmStack.parseStackString([]byte(stackBytes))
	if err != nil {
		return
	}

	log.Debug("decrypting secrets...")
	err = swarmStack.decryptSopsFiles(stackContents)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt one or more sops files for %s stack: %w", swarmStack.name, err)
	}

	if config.AutoRotate {
		log.Debug("rotating configs and secrets...")
		err = swarmStack.rotateConfigsAndSecrets(stackContents)
		if err != nil {
			return
		}
	}

	log.Debug("writing stack to file...")
	err = swarmStack.writeStack(stackContents)
	if err != nil {
		return
	}

	log.Debug("deploying stack...")
	err = swarmStack.deployStack()
	return
}

func (swarmStack *swarmStack) readStack() ([]byte, error) {
	composeFile := path.Join(swarmStack.repo.path, swarmStack.composePath)
	composeFileBytes, err := os.ReadFile(composeFile)
	if err != nil {
		return nil, fmt.Errorf("could not read compose file %s: %w", composeFile, err)
	}
	return composeFileBytes, nil
}

func (swarmStack *swarmStack) renderComposeTemplate(templateContents []byte) ([]byte, error) {
	valuesFile := path.Join(swarmStack.repo.path, swarmStack.valuesFile)
	valuesBytes, err := os.ReadFile(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("could not read %s stack values file: %w", swarmStack.name, err)
	}
	var valuesMap map[string]any
	yaml.Unmarshal(valuesBytes, &valuesMap)
	templ, err := template.New(swarmStack.name).Parse(string(templateContents[:]))
	if err != nil {
		return nil, fmt.Errorf("could not parse %s stack compose file as a Go template: %w", swarmStack.name, err)
	}
	var stackContents bytes.Buffer
	err = templ.Execute(&stackContents, map[string]map[string]any{"Values": valuesMap})
	if err != nil {
		return nil, fmt.Errorf("error rending %s stack compose template: %w", swarmStack.name, err)
	}
	return stackContents.Bytes(), nil
}

func (swarmStack *swarmStack) parseStackString(stackContent []byte) (map[string]any, error) {
	var composeMap map[string]any
	err := yaml.Unmarshal(stackContent, &composeMap)
	if err != nil {
		return nil, fmt.Errorf("could not parse stack yaml: %w", err)
	}
	return composeMap, nil
}

func (swarmStack *swarmStack) decryptSopsFiles(composeMap map[string]any) (err error) {
	var sopsFiles []string
	if !swarmStack.discoverSecrets {
		sopsFiles = swarmStack.sopsFiles
	} else {
		sopsFiles, err = discoverSecrets(composeMap, swarmStack.composePath)
		if err != nil {
			return
		}
	}
	log := logger.With(
		slog.String("stack", swarmStack.name),
		slog.String("branch", swarmStack.branch),
	)
	for _, sopsFile := range sopsFiles {
		log.Debug("decrypting secret...", "secret", sopsFile)
		err = util.DecryptFile(path.Join(swarmStack.repo.path, sopsFile))
		if err != nil {
			return
		}
	}
	return
}

func discoverSecrets(composeMap map[string]any, composePath string) ([]string, error) {
	var sopsFiles []string
	if secrets, ok := composeMap["secrets"].(map[string]any); ok {
		for secretName, secret := range secrets {
			secretMap, ok := secret.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid compose file: %s secret must be a map", secretName)
			}
			isExternal, ok := secretMap["external"].(bool)
			if ok && isExternal {
				continue
			}
			secretFile, ok := secretMap["file"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid compose file: %s file field must be a string", secretName)
			}
			secretPath := path.Join(path.Dir(composePath), secretFile)
			sopsFiles = append(sopsFiles, secretPath)
		}
	}
	return sopsFiles, nil
}

func (swarmStack *swarmStack) rotateConfigsAndSecrets(composeMap map[string]any) error {
	if configs, ok := composeMap["configs"].(map[string]any); ok {
		err := swarmStack.rotateObjects(configs, "configs")
		if err != nil {
			return fmt.Errorf("could not rotate one or more config files of stack %s: %w", swarmStack.name, err)
		}
	}
	if secrets, ok := composeMap["secrets"].(map[string]any); ok {
		err := swarmStack.rotateObjects(secrets, "secrets")
		if err != nil {
			return fmt.Errorf("could not rotate one or more secret files of stack %s: %w", swarmStack.name, err)
		}
	}
	return nil
}

func (swarmStack *swarmStack) rotateObjects(objects map[string]any, objectType string) error {
	objectsDir := path.Dir(path.Join(swarmStack.repo.path, swarmStack.composePath))
	for objectName, object := range objects {
		log := logger.With(
			slog.String("stack", swarmStack.name),
			slog.String("branch", swarmStack.branch),
			slog.String(objectType, objectName),
		)
		objectMap, ok := object.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid compose file: %s object must be a map", objectName)
		}
		isExternal, ok := objectMap["external"].(bool)
		if ok && isExternal {
			continue
		}
		objectFile, ok := objectMap["file"].(string)
		if !ok {
			return fmt.Errorf("invalid compose file: %s file field must be a string", objectName)
		}
		log.Debug("reading...", "file", objectFile)
		objectFilePath := path.Join(objectsDir, objectFile)
		configFileBytes, err := os.ReadFile(objectFilePath)
		if err != nil {
			return fmt.Errorf("could not read file %s for rotation: %w", objectFilePath, err)
		}
		log.Debug("computing hash...", "file", objectFile)
		hash := fmt.Sprintf("%x", md5.Sum(configFileBytes))[:8]
		newObjectName := swarmStack.name + "-" + objectName + "-" + hash
		log.Debug("renaming...", "new_name", newObjectName)
		objectMap["name"] = newObjectName
	}
	return nil
}

func (swarmStack *swarmStack) writeStack(composeMap map[string]any) error {
	composeFileBytes, err := yaml.Marshal(composeMap)
	if err != nil {
		return fmt.Errorf("could not store compose file as yaml after calculating hashes for stack %s", swarmStack.name)
	}
	composeFile := path.Join(swarmStack.repo.path, swarmStack.composePath)
	fileInfo, _ := os.Stat(composeFile)
	os.WriteFile(composeFile, composeFileBytes, fileInfo.Mode())
	return nil
}

// loadEnvFiles pulls any external env file repos and reads all env files
// in order, merging them into a single map (later files override earlier ones).
func (swarmStack *swarmStack) loadEnvFiles() (map[string]string, error) {
	log := logger.With(
		slog.String("stack", swarmStack.name),
	)
	envMap := map[string]string{}
	for _, ef := range swarmStack.envFiles {
		// Pull the env file repo if it differs from the stack's own repo
		if ef.repo != swarmStack.repo {
			ef.repo.lock.Lock()
			_, pullErr := ef.repo.pullChanges(ef.branch)
			ef.repo.lock.Unlock()
			if pullErr != nil {
				return nil, fmt.Errorf("could not pull env file repo %s: %w", ef.repo.name, pullErr)
			}
		}
		filePath := path.Join(ef.repo.path, ef.path)
		log.Debug("reading env file...", "path", filePath)
		parsed, parseErr := parseEnvFile(filePath)
		if parseErr != nil {
			return nil, fmt.Errorf("could not parse env file %s for stack %s: %w", filePath, swarmStack.name, parseErr)
		}
		for k, v := range parsed {
			envMap[k] = v
		}
	}
	return envMap, nil
}

// parseEnvFile reads a .env file and returns a map of key-value pairs.
// It supports comments (#), empty lines, quoted values, and "export" prefix.
func parseEnvFile(filePath string) (map[string]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	envMap := map[string]string{}
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Support "export KEY=VALUE" syntax
		line = strings.TrimPrefix(line, "export ")

		idx := strings.IndexByte(line, '=')
		if idx == -1 {
			return nil, fmt.Errorf("invalid syntax at line %d: %s", i+1, line)
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Strip matching quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		envMap[key] = value
	}
	return envMap, nil
}

// envVarPattern matches:
//   - $$ (escaped dollar sign)
//   - ${VAR}, ${VAR:-default}, ${VAR-default}
//   - $VAR (bare variable reference)
var envVarPattern = regexp.MustCompile(`\$\$|\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// substituteEnvVars replaces ${VAR} and $VAR references in content
// using the provided env map, following docker-compose semantics:
//   - ${VAR} / $VAR → value from map, or empty string if unset
//   - ${VAR:-default} → value if set and non-empty, otherwise default
//   - ${VAR-default} → value if set (even if empty), otherwise default
//   - $$ → literal $
func substituteEnvVars(content []byte, envMap map[string]string) []byte {
	result := envVarPattern.ReplaceAllStringFunc(string(content), func(match string) string {
		if match == "$$" {
			return "$"
		}

		var varExpr string
		if strings.HasPrefix(match, "${") {
			// ${...} form
			varExpr = match[2 : len(match)-1]
		} else {
			// $VAR form — no default syntax possible
			varName := match[1:]
			if val, ok := envMap[varName]; ok {
				return val
			}
			return ""
		}

		// Handle ${VAR:-default} (default if unset or empty)
		if idx := strings.Index(varExpr, ":-"); idx != -1 {
			varName := varExpr[:idx]
			defaultVal := varExpr[idx+2:]
			if val, ok := envMap[varName]; ok && val != "" {
				return val
			}
			return defaultVal
		}

		// Handle ${VAR-default} (default if unset)
		if idx := strings.IndexByte(varExpr, '-'); idx != -1 {
			varName := varExpr[:idx]
			defaultVal := varExpr[idx+1:]
			if val, ok := envMap[varName]; ok {
				return val
			}
			return defaultVal
		}

		// Plain ${VAR}
		if val, ok := envMap[varExpr]; ok {
			return val
		}
		return ""
	})
	return []byte(result)
}

func (swarmStack *swarmStack) deployStack() error {
	cmd := stack.NewStackCommand(dockerCli)
	cmd.SetArgs([]string{
		"deploy", "--detach", "--with-registry-auth", "-c",
		path.Join(swarmStack.repo.path, swarmStack.composePath),
		swarmStack.name,
	})
	// To stop printing errors and
	// usage message to stdout
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	if err != nil {
		return fmt.Errorf("could not deploy stack %s: %s", swarmStack.name, err)
	}
	return nil
}
