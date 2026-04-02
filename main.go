package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

const repo = "tornikegomareli/claudeignore"

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "setup":
		cmdSetup()
	case "hook":
		cmdHook()
	case "test":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: claudeignore test <filepath>")
			os.Exit(1)
		}
		cmdTest(os.Args[2])
	case "status":
		cmdStatus()
	case "wrap":
		cmdWrap()
	case "update":
		cmdUpdate()
	case "version", "--version", "-v":
		fmt.Printf("claudeignore v%s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		printUsage()
		os.Exit(1)
	}
}

func cmdSetup() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fatal("Cannot find home directory: %v", err)
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	selfPath, _ := os.Executable()
	if selfPath == "" {
		selfPath = "claudeignore"
	}

	if err := writeSettings(filepath.Join(claudeDir, "settings.json"), selfPath); err != nil {
		fatal("Failed to write settings: %v", err)
	}

	fmt.Println("✓ Hook registered globally in ~/.claude/settings.json")
	fmt.Println("\nDrop a .claudeignore in any project. Changes take effect immediately.")
}

func writeSettings(path, selfPath string) error {
	hookCmd := selfPath + " hook"
	hookEntry := map[string]any{
		"matcher": "Read|Edit|Write|Glob|Grep",
		"hooks":   []any{map[string]any{"type": "command", "command": hookCmd}},
	}

	if data, err := os.ReadFile(path); err == nil {
		if strings.Contains(string(data), "claudeignore hook") {
			fmt.Println("✓ Already registered")
			return nil
		}
		var existing map[string]any
		if json.Unmarshal(data, &existing) == nil {
			hooks, _ := existing["hooks"].(map[string]any)
			if hooks == nil {
				hooks = map[string]any{}
			}
			pre, _ := hooks["PreToolUse"].([]any)
			hooks["PreToolUse"] = append(pre, hookEntry)
			existing["hooks"] = hooks
			out, _ := json.MarshalIndent(existing, "", "  ")
			return os.WriteFile(path, out, 0644)
		}
	}

	out, _ := json.MarshalIndent(map[string]any{
		"hooks": map[string]any{"PreToolUse": []any{hookEntry}},
	}, "", "  ")
	return os.WriteFile(path, out, 0644)
}

type hookInput struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

func cmdHook() {
	var input hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		os.Exit(0)
	}

	var filePath string
	switch input.ToolName {
	case "Read", "Edit", "Write":
		filePath, _ = input.ToolInput["file_path"].(string)
	case "Glob", "Grep":
		filePath, _ = input.ToolInput["path"].(string)
	default:
		os.Exit(0)
	}

	if filePath == "" {
		os.Exit(0)
	}

	gi, root := findIgnoreFile(filePath)
	if gi == nil {
		os.Exit(0)
	}

	relPath, _ := filepath.Rel(root, filePath)
	if matches(gi, relPath, filePath) {
		fmt.Fprintln(os.Stderr, "Blocked by .claudeignore — skip to save context.")
		os.Exit(2)
	}
	os.Exit(0)
}

func findIgnoreFile(fromPath string) (*ignore.GitIgnore, string) {
	dir := filepath.Dir(fromPath)
	for {
		if gi, err := ignore.CompileIgnoreFile(filepath.Join(dir, ".claudeignore")); err == nil {
			return gi, dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ""
		}
		dir = parent
	}
}

func matches(gi *ignore.GitIgnore, relPath, absPath string) bool {
	if gi.MatchesPath(relPath) {
		return true
	}
	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		return gi.MatchesPath(relPath + "/")
	}
	return gi.MatchesPath(relPath + "/")
}

func cmdWrap() {
	if runtime.GOOS != "darwin" {
		fatal("wrap is only supported on macOS (uses sandbox-exec/Seatbelt)")
	}

	root, err := os.Getwd()
	if err != nil {
		fatal("Cannot get working directory: %v", err)
	}

	gi, err := ignore.CompileIgnoreFile(filepath.Join(root, ".claudeignore"))
	if err != nil {
		fatal("No .claudeignore found in current directory")
	}

	denied := collectDeniedPaths(root, gi)
	if len(denied) == 0 {
		fmt.Println("No files matched .claudeignore — running claude without sandbox.")
		execClaude(os.Args[2:])
		return
	}

	profile := generateSeatbeltProfile(denied)

	tmpFile, err := os.CreateTemp("", "claudeignore-*.sb")
	if err != nil {
		fatal("Cannot create sandbox profile: %v", err)
	}
	tmpFile.WriteString(profile)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	fmt.Printf("Sandboxing %d paths via macOS Seatbelt\n", len(denied))

	args := []string{"-f", tmpFile.Name()}
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		fatal("claude not found in PATH")
	}
	args = append(args, claudePath)
	args = append(args, os.Args[2:]...)

	cmd := exec.Command("sandbox-exec", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatal("sandbox-exec failed: %v", err)
	}
}

func collectDeniedPaths(root string, gi *ignore.GitIgnore) []string {
	var denied []string
	skipDirs := map[string]bool{}

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}

		if rel == ".git" {
			return filepath.SkipDir
		}

		for skip := range skipDirs {
			if strings.HasPrefix(path, skip) {
				return nil
			}
		}

		if info.IsDir() {
			if gi.MatchesPath(rel) || gi.MatchesPath(rel+"/") {
				denied = append(denied, path)
				skipDirs[path+string(filepath.Separator)] = true
				return filepath.SkipDir
			}
			return nil
		}

		if gi.MatchesPath(rel) {
			denied = append(denied, path)
		}
		return nil
	})

	return denied
}

func generateSeatbeltProfile(deniedPaths []string) string {
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(deny default)\n")
	b.WriteString("(allow default)\n")

	for _, p := range deniedPaths {
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			resolved = p
		}
		escaped := seatbeltEscape(resolved)
		info, err := os.Stat(p)
		if err == nil && info.IsDir() {
			fmt.Fprintf(&b, "(deny file-read-data file-write-data (subpath \"%s\"))\n", escaped)
		} else {
			fmt.Fprintf(&b, "(deny file-read-data file-write-data (literal \"%s\"))\n", escaped)
		}
	}

	return b.String()
}

func seatbeltEscape(path string) string {
	path = strings.ReplaceAll(path, "\\", "\\\\")
	path = strings.ReplaceAll(path, "\"", "\\\"")
	return path
}

func execClaude(args []string) {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		fatal("claude not found in PATH")
	}
	cmd := exec.Command(claudePath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
	}
}

func cmdUpdate() {
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		fatal("Failed to check for updates: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fatal("No releases found on GitHub")
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fatal("Failed to parse release info: %v", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	if latest == version {
		fmt.Printf("Already on latest version (v%s)\n", version)
		return
	}

	fmt.Printf("Updating v%s → v%s\n", version, latest)

	arch := runtime.GOARCH
	goos := runtime.GOOS
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/claudeignore-%s-%s", repo, release.TagName, goos, arch)

	binResp, err := http.Get(url)
	if err != nil {
		fatal("Download failed: %v", err)
	}
	defer binResp.Body.Close()

	if binResp.StatusCode != 200 {
		fatal("Binary not found for %s/%s at %s", goos, arch, release.TagName)
	}

	selfPath, err := os.Executable()
	if err != nil {
		fatal("Cannot find own path: %v", err)
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)

	tmpFile := selfPath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		fatal("Cannot write update: %v", err)
	}

	if _, err := io.Copy(f, binResp.Body); err != nil {
		f.Close()
		os.Remove(tmpFile)
		fatal("Download interrupted: %v", err)
	}
	f.Close()

	if err := os.Rename(tmpFile, selfPath); err != nil {
		os.Remove(tmpFile)
		fatal("Cannot replace binary: %v", err)
	}

	fmt.Printf("✓ Updated to v%s\n", latest)
}

func cmdTest(path string) {
	root, _ := os.Getwd()
	gi, err := ignore.CompileIgnoreFile(filepath.Join(root, ".claudeignore"))
	if err != nil {
		fatal("No .claudeignore found in current directory")
	}

	checkPath := path
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(root, path); err == nil {
			checkPath = rel
		}
	}
	absPath := filepath.Join(root, checkPath)

	if matches(gi, checkPath, absPath) {
		fmt.Printf("\033[31mBLOCKED\033[0m  %s\n", path)
		os.Exit(1)
	}
	fmt.Printf("\033[32mALLOWED\033[0m  %s\n", path)
}

func cmdStatus() {
	fmt.Printf("claudeignore v%s\n\n", version)

	homeDir, _ := os.UserHomeDir()
	if data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json")); err == nil && strings.Contains(string(data), "claudeignore hook") {
		fmt.Println("✓ Hook: registered globally")
	} else {
		fmt.Println("✗ Hook: not registered (run 'claudeignore setup')")
	}

	root, _ := os.Getwd()
	if data, err := os.ReadFile(filepath.Join(root, ".claudeignore")); err == nil {
		count := 0
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				count++
			}
		}
		fmt.Printf("✓ .claudeignore: %d patterns\n", count)
	} else {
		fmt.Println("- .claudeignore: not found in current directory")
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func printUsage() {
	fmt.Printf(`claudeignore v%s — Save tokens by keeping Claude out of ignored files

USAGE
    claudeignore <command> [args]

COMMANDS
    setup         One-time: register hook globally in ~/.claude/settings.json
    wrap          Launch claude inside macOS Seatbelt sandbox (blocks sub-agents)
    update        Update to the latest version
    hook          PreToolUse handler (called automatically by Claude Code)
    test <path>   Test if a file path would be blocked
    status        Show current configuration
    version       Show version

HOW IT WORKS
    1. Run 'claudeignore setup' once after installing
    2. Drop a .claudeignore file in any project (gitignore syntax)
    3. That's it — Claude Code will skip those files automatically

    For sub-agent protection on macOS, use 'claudeignore wrap' to launch
    claude inside a kernel-level sandbox.
`, version)
}
