package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

func main() {
	root := flag.NewFlagSet("atmosphere-cli", flag.ExitOnError)
	apiBase := root.String("api", "http://localhost:3000", "Atmosphere API base URL")
	timeout := root.Duration("timeout", 30*time.Second, "HTTP timeout")
	if err := root.Parse(os.Args[1:]); err != nil {
		fatal(err)
	}

	args := root.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	client := &apiClient{
		baseURL: strings.TrimRight(*apiBase, "/"),
		http: &http.Client{
			Timeout: *timeout,
		},
	}

	if err := run(client, args); err != nil {
		fatal(err)
	}
}

type apiClient struct {
	baseURL string
	http    *http.Client
}

func run(c *apiClient, args []string) error {
	switch args[0] {
	case "apps":
		return runApps(c, args[1:])
	case "backups":
		return runBackups(c, args[1:])
	case "restores":
		return runRestores(c, args[1:])
	case "templates":
		return runTemplates(c, args[1:])
	case "system":
		return runSystem(c, args[1:])
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runApps(c *apiClient, args []string) error {
	if len(args) == 0 {
		printAppsUsage()
		return errors.New("apps subcommand required")
	}

	switch args[0] {
	case "list":
		return c.requestAndPrint(http.MethodGet, "/api/v1/apps", nil)
	case "get":
		if len(args) < 2 {
			return errors.New("usage: apps get <name>")
		}
		return c.requestAndPrint(http.MethodGet, "/api/v1/apps/"+url.PathEscape(args[1]), nil)
	case "containers":
		if len(args) < 2 {
			return errors.New("usage: apps containers <name>")
		}
		return c.requestAndPrint(http.MethodGet, "/api/v1/apps/"+url.PathEscape(args[1])+"/containers", nil)
	case "create":
		body, err := parseJSONFlags("apps create", args[1:])
		if err != nil {
			return err
		}
		return c.requestAndPrint(http.MethodPost, "/api/v1/apps", body)
	case "update":
		if len(args) < 2 {
			return errors.New("usage: apps update <name> [--json <payload> | --file <path>]")
		}
		body, err := parseJSONFlags("apps update", args[2:])
		if err != nil {
			return err
		}
		return c.requestAndPrint(http.MethodPut, "/api/v1/apps/"+url.PathEscape(args[1]), body)
	case "deploy", "start", "stop", "delete", "destroy":
		if len(args) < 2 {
			return fmt.Errorf("usage: apps %s <name>", args[0])
		}
		name := url.PathEscape(args[1])
		suffix := map[string]string{
			"deploy":  "/deploy",
			"start":   "/start",
			"stop":    "/stop",
			"delete":  "",
			"destroy": "/destroy",
		}[args[0]]
		method := http.MethodPost
		if args[0] == "delete" {
			method = http.MethodDelete
		}
		return c.requestAndPrint(method, "/api/v1/apps/"+name+suffix, nil)
	case "logs":
		if len(args) < 2 {
			return errors.New("usage: apps logs <name> [--limit <n>]")
		}
		limit := ""
		fs := flag.NewFlagSet("apps logs", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		limitFlag := fs.Int("limit", 0, "Limit number of deployment logs")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *limitFlag > 0 {
			limit = "?limit=" + strconv.Itoa(*limitFlag)
		}
		return c.requestAndPrint(http.MethodGet, "/api/v1/apps/"+url.PathEscape(args[1])+"/logs"+limit, nil)
	default:
		printAppsUsage()
		return fmt.Errorf("unknown apps subcommand: %s", args[0])
	}
}

func runBackups(c *apiClient, args []string) error {
	if len(args) == 0 {
		printBackupsUsage()
		return errors.New("backups subcommand required")
	}

	switch args[0] {
	case "create":
		if len(args) < 2 {
			return errors.New("usage: backups create <app> [--upload-to-s3]")
		}
		fs := flag.NewFlagSet("backups create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uploadToS3 := fs.Bool("upload-to-s3", false, "Upload backup to S3")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		var body []byte
		if *uploadToS3 {
			var err error
			body, err = json.Marshal(map[string]bool{"upload_to_s3": true})
			if err != nil {
				return err
			}
		}
		return c.requestAndPrint(http.MethodPost, "/api/v1/apps/"+url.PathEscape(args[1])+"/backups", body)
	case "list":
		if len(args) < 2 {
			return errors.New("usage: backups list <app> [--limit <n>]")
		}
		fs := flag.NewFlagSet("backups list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		limit := fs.Int("limit", 0, "Limit returned backups")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		query := ""
		if *limit > 0 {
			query = "?limit=" + strconv.Itoa(*limit)
		}
		return c.requestAndPrint(http.MethodGet, "/api/v1/apps/"+url.PathEscape(args[1])+"/backups"+query, nil)
	case "get":
		if len(args) < 3 {
			return errors.New("usage: backups get <app> <backup-id>")
		}
		return c.requestAndPrint(http.MethodGet, "/api/v1/apps/"+url.PathEscape(args[1])+"/backups/"+url.PathEscape(args[2]), nil)
	case "delete":
		if len(args) < 3 {
			return errors.New("usage: backups delete <app> <backup-id>")
		}
		return c.requestAndPrint(http.MethodDelete, "/api/v1/apps/"+url.PathEscape(args[1])+"/backups/"+url.PathEscape(args[2]), nil)
	default:
		printBackupsUsage()
		return fmt.Errorf("unknown backups subcommand: %s", args[0])
	}
}

func runRestores(c *apiClient, args []string) error {
	if len(args) == 0 {
		printRestoresUsage()
		return errors.New("restores subcommand required")
	}

	switch args[0] {
	case "start":
		if len(args) < 2 {
			return errors.New("usage: restores start <app> --backup-id <id> [--source-app <name>] [--restore-as-new] [--new-app-name <name>]")
		}
		fs := flag.NewFlagSet("restores start", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		backupID := fs.String("backup-id", "", "Backup ID")
		sourceApp := fs.String("source-app", "", "Source app name")
		restoreAsNew := fs.Bool("restore-as-new", false, "Restore as new app")
		newAppName := fs.String("new-app-name", "", "Name for restored app when --restore-as-new is set")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *backupID == "" {
			return errors.New("--backup-id is required")
		}
		body, err := json.Marshal(map[string]any{
			"backup_id":      *backupID,
			"source_app":     *sourceApp,
			"restore_as_new": *restoreAsNew,
			"new_app_name":   *newAppName,
		})
		if err != nil {
			return err
		}
		return c.requestAndPrint(http.MethodPost, "/api/v1/apps/"+url.PathEscape(args[1])+"/restores", body)
	case "fresh":
		fs := flag.NewFlagSet("restores fresh", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		sourceApp := fs.String("source-app", "", "Source app name")
		backupID := fs.String("backup-id", "", "Backup ID")
		appName := fs.String("app-name", "", "Target app name (defaults to source app)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *sourceApp == "" {
			return errors.New("--source-app is required")
		}
		if *backupID == "" {
			return errors.New("--backup-id is required")
		}
		body, err := json.Marshal(map[string]string{
			"source_app": *sourceApp,
			"backup_id":  *backupID,
			"app_name":   *appName,
		})
		if err != nil {
			return err
		}
		return c.requestAndPrint(http.MethodPost, "/api/v1/restores/fresh", body)
	case "get":
		if len(args) < 3 {
			return errors.New("usage: restores get <app> <restore-id>")
		}
		return c.requestAndPrint(http.MethodGet, "/api/v1/apps/"+url.PathEscape(args[1])+"/restores/"+url.PathEscape(args[2]), nil)
	default:
		printRestoresUsage()
		return fmt.Errorf("unknown restores subcommand: %s", args[0])
	}
}

func runSystem(c *apiClient, args []string) error {
	if len(args) == 0 {
		printSystemUsage()
		return errors.New("system subcommand required")
	}

	switch args[0] {
	case "hard-reset":
		fs := flag.NewFlagSet("system hard-reset", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		confirm := fs.Bool("confirm", false, "Required: confirm destructive hard reset")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if !*confirm {
			return errors.New("--confirm flag is required for hard reset (this is irreversible)")
		}
		body, err := json.Marshal(map[string]bool{"confirm": true})
		if err != nil {
			return err
		}
		return c.requestAndPrint(http.MethodPost, "/api/v1/system/hard-reset", body)
	default:
		printSystemUsage()
		return fmt.Errorf("unknown system subcommand: %s", args[0])
	}
}

func runTemplates(c *apiClient, args []string) error {	if len(args) == 0 {
		printTemplatesUsage()
		return errors.New("templates subcommand required")
	}

	switch args[0] {
	case "list":
		return c.requestAndPrint(http.MethodGet, "/api/v1/templates", nil)
	case "get":
		if len(args) < 2 {
			return errors.New("usage: templates get <id>")
		}
		return c.requestAndPrint(http.MethodGet, "/api/v1/templates/"+url.PathEscape(args[1]), nil)
	case "provision":
		if len(args) < 2 {
			return errors.New("usage: templates provision <id> [--json <payload> | --file <path>]")
		}
		body, err := parseJSONFlags("templates provision", args[2:])
		if err != nil {
			return err
		}
		return c.requestAndPrint(http.MethodPost, "/api/v1/templates/"+url.PathEscape(args[1])+"/provision", body)
	default:
		printTemplatesUsage()
		return fmt.Errorf("unknown templates subcommand: %s", args[0])
	}
}

func parseJSONFlags(name string, args []string) ([]byte, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	inline := fs.String("json", "", "Inline JSON payload")
	filePath := fs.String("file", "", "Path to JSON payload file")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if *inline == "" && *filePath == "" {
		return nil, errors.New("one of --json or --file is required")
	}
	if *inline != "" && *filePath != "" {
		return nil, errors.New("use only one of --json or --file")
	}

	if *inline != "" {
		if !json.Valid([]byte(*inline)) {
			return nil, errors.New("--json is not valid JSON")
		}
		return []byte(*inline), nil
	}

	content, err := os.ReadFile(*filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", *filePath, err)
	}
	if !json.Valid(content) {
		return nil, fmt.Errorf("file %s does not contain valid JSON", *filePath)
	}
	return content, nil
}

func (c *apiClient) requestAndPrint(method, reqPath string, body []byte) error {
	respBody, err := c.request(method, reqPath, body)
	if err != nil {
		return err
	}

	if len(respBody) == 0 {
		fmt.Println("{}")
		return nil
	}

	var pretty bytes.Buffer
	if json.Indent(&pretty, respBody, "", "  ") == nil {
		fmt.Println(pretty.String())
		return nil
	}

	fmt.Println(string(respBody))
	return nil
}

func (c *apiClient) request(method, reqPath string, body []byte) ([]byte, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = path.Join(u.Path, reqPath)

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, u.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(respBody) == 0 {
			return nil, fmt.Errorf("API error: %s", resp.Status)
		}
		return nil, fmt.Errorf("API error: %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	return respBody, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func printUsage() {
	fmt.Println("atmosphere-cli - Atmosphere API wrapper")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  atmosphere-cli [--api <url>] [--timeout <duration>] <command> <subcommand> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  apps       Manage applications")
	fmt.Println("  backups    Manage app backups")
	fmt.Println("  restores   Start and inspect restores")
	fmt.Println("  templates  Template operations")
	fmt.Println("  system     System-level operations")
	fmt.Println()
	fmt.Println("Run with a command only to see command-specific usage, for example:")
	fmt.Println("  atmosphere-cli apps")
}

func printAppsUsage() {
	fmt.Println("apps commands:")
	fmt.Println("  apps list")
	fmt.Println("  apps get <name>")
	fmt.Println("  apps containers <name>")
	fmt.Println("  apps create --json <payload> | --file <path>")
	fmt.Println("  apps update <name> --json <payload> | --file <path>")
	fmt.Println("  apps deploy <name>")
	fmt.Println("  apps start <name>")
	fmt.Println("  apps stop <name>")
	fmt.Println("  apps delete <name>")
	fmt.Println("  apps destroy <name>")
	fmt.Println("  apps logs <name> [--limit <n>]")
}

func printBackupsUsage() {
	fmt.Println("backups commands:")
	fmt.Println("  backups create <app> [--upload-to-s3]")
	fmt.Println("  backups list <app> [--limit <n>]")
	fmt.Println("  backups get <app> <backup-id>")
	fmt.Println("  backups delete <app> <backup-id>")
}

func printRestoresUsage() {
	fmt.Println("restores commands:")
	fmt.Println("  restores start <app> --backup-id <id> [--source-app <name>] [--restore-as-new] [--new-app-name <name>]")
	fmt.Println("  restores fresh --source-app <name> --backup-id <id> [--app-name <name>]")
	fmt.Println("  restores get <app> <restore-id>")
}

func printTemplatesUsage() {
	fmt.Println("templates commands:")
	fmt.Println("  templates list")
	fmt.Println("  templates get <id>")
	fmt.Println("  templates provision <id> --json <payload> | --file <path>")
}

func printSystemUsage() {
	fmt.Println("system commands:")
	fmt.Println("  system hard-reset --confirm")
	fmt.Println()
	fmt.Println("  hard-reset  Permanently deletes all containers, volumes, workspaces, keys,")
	fmt.Println("              logs, local backups, and the database.")
	fmt.Println("              *.ini files and S3 backups are preserved.")
	fmt.Println("              The server must be restarted after a hard reset.")
}
