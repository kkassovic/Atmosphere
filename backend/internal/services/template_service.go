package services

import (
	"atmosphere/internal/models"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const templateManifestFile = "template.json"

// ListTemplates lists all app templates from the configured templates directory.
func (s *AppService) ListTemplates() ([]*models.AppTemplate, error) {
	entries, err := os.ReadDir(s.cfg.TemplatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates dir: %w", err)
	}

	templates := make([]*models.AppTemplate, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t, err := s.loadTemplate(entry.Name())
		if err != nil {
			continue
		}
		templates = append(templates, t)
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

// GetTemplate gets one app template by ID.
func (s *AppService) GetTemplate(templateID string) (*models.AppTemplate, error) {
	t, err := s.loadTemplate(templateID)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// ProvisionTemplate provisions a new app from a template.
func (s *AppService) ProvisionTemplate(templateID string, req *models.TemplateProvisionRequest) (*models.TemplateProvisionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.AppName == "" {
		return nil, fmt.Errorf("app_name is required")
	}

	templateDef, err := s.loadTemplate(templateID)
	if err != nil {
		return nil, err
	}

	inputs, err := s.resolveTemplateInputs(templateDef, req)
	if err != nil {
		return nil, err
	}

	envVars := make(models.EnvVars)
	for k, v := range templateDef.DefaultEnv {
		rendered, err := renderTemplateString(v, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to render default env %s: %w", k, err)
		}
		envVars[k] = rendered
	}
	for k, v := range req.EnvVars {
		envVars[k] = v
	}

	createReq := &models.CreateAppRequest{
		Name:           req.AppName,
		DeploymentType: templateDef.DeploymentType,
		BuildType:      templateDef.BuildType,
		Domains:        req.Domains,
		EnvVars:        envVars,
		DockerfilePath: templateDef.DockerfilePath,
		ComposePath:    templateDef.ComposePath,
		Port:           templateDef.Port,
	}

	app, err := s.CreateApp(createReq)
	if err != nil {
		return nil, err
	}

	if err := s.materializeTemplateFiles(templateID, app.Name, inputs); err != nil {
		_ = s.DeleteApp(app.Name)
		return nil, err
	}

	resp := &models.TemplateProvisionResponse{
		Message: "App provisioned from template",
		App:     app,
	}

	if req.AutoDeploy {
		deployLog, err := s.DeployApp(app.Name)
		if err != nil {
			return nil, fmt.Errorf("app created but deploy failed to start: %w", err)
		}
		resp.DeploymentLog = deployLog
	}

	return resp, nil
}

func (s *AppService) loadTemplate(templateID string) (*models.AppTemplate, error) {
	if !isValidTemplateID(templateID) {
		return nil, fmt.Errorf("invalid template id")
	}

	manifestPath := filepath.Join(s.cfg.TemplatesDir, templateID, templateManifestFile)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template not found")
		}
		return nil, fmt.Errorf("failed to read template manifest: %w", err)
	}

	var t models.AppTemplate
	if err := json.Unmarshal(content, &t); err != nil {
		return nil, fmt.Errorf("failed to parse template manifest: %w", err)
	}

	if t.ID == "" {
		t.ID = templateID
	}
	if t.Name == "" {
		t.Name = templateID
	}
	if t.DeploymentType == "" {
		t.DeploymentType = "manual"
	}
	if t.BuildType == "" {
		t.BuildType = "compose"
	}
	if t.BuildType == "compose" && t.ComposePath == "" {
		t.ComposePath = "docker-compose.yml"
	}
	if t.DefaultEnv == nil {
		t.DefaultEnv = make(map[string]string)
	}

	return &t, nil
}

func (s *AppService) resolveTemplateInputs(templateDef *models.AppTemplate, req *models.TemplateProvisionRequest) (map[string]string, error) {
	values := make(map[string]string)

	values["app_name"] = req.AppName
	if len(req.Domains) > 0 {
		values["domain"] = req.Domains[0]
		values["domains_csv"] = strings.Join(req.Domains, ",")
	} else {
		values["domain"] = ""
		values["domains_csv"] = ""
	}

	for _, input := range templateDef.Inputs {
		val := ""
		if req.Inputs != nil {
			val = req.Inputs[input.Name]
		}
		if val == "" {
			val = input.Default
		}
		if input.Required && strings.TrimSpace(val) == "" {
			return nil, fmt.Errorf("missing required input: %s", input.Name)
		}
		values[input.Name] = val
	}

	for k, v := range req.Inputs {
		if _, exists := values[k]; !exists {
			values[k] = v
		}
	}

	return values, nil
}

func (s *AppService) materializeTemplateFiles(templateID, appName string, values map[string]string) error {
	root := filepath.Join(s.cfg.TemplatesDir, templateID)

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == templateManifestFile {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rendered, err := renderTemplateString(string(content), values)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", relPath, err)
		}

		if err := s.UploadFile(appName, filepath.ToSlash(relPath), []byte(rendered)); err != nil {
			return fmt.Errorf("failed to write %s: %w", relPath, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to materialize template files: %w", err)
	}

	return nil
}

func renderTemplateString(input string, values map[string]string) (string, error) {
	re := regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)
	missing := make(map[string]struct{})

	out := re.ReplaceAllStringFunc(input, func(match string) string {
		groups := re.FindStringSubmatch(match)
		if len(groups) != 2 {
			return match
		}
		key := groups[1]
		val, ok := values[key]
		if !ok {
			missing[key] = struct{}{}
			return match
		}
		return val
	})

	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", fmt.Errorf("missing template values: %s", strings.Join(keys, ", "))
	}

	return out, nil
}

func isValidTemplateID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	match, _ := regexp.MatchString(`^[a-z0-9-]+$`, id)
	return match
}
