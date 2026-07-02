package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	Engine string `json:"engine"`
	Debug  bool   `json:"debug"`
	MLX    struct {
		Model       string   `json:"model"`
		Command     []string `json:"command"`
		ChatCommand []string `json:"chat_command"`
		Server      struct {
			URL             string   `json:"url"`
			Command         []string `json:"command"`
			AutoStart       bool     `json:"auto_start"`
			ExternalApp     bool     `json:"external_app"`
			Stream          bool     `json:"stream"`
			LogFile         string   `json:"log_file"`
			KeepWarmSeconds int      `json:"keep_warm_seconds"`
			StampFile       string   `json:"stamp_file"`
			ReaperPIDFile   string   `json:"reaper_pid_file"`
		} `json:"server"`
		MaxTokens int `json:"max_tokens"`
		Timeout   int `json:"timeout_seconds"`
	} `json:"mlx"`
	Gemini struct {
		APIKey string `json:"api_key"`
		Model  string `json:"model"`
	} `json:"gemini"`
	Ollama struct {
		Model string `json:"model"`
		Host  string `json:"host"`
	} `json:"ollama"`
	Agent struct {
		Profile       string `json:"profile"`
		SessionDir    string `json:"session_dir"`
		FastModel     string `json:"fast_model"`
		SmartModel    string `json:"smart_model"`
		Clipboard     bool   `json:"clipboard"`
		Reports       bool   `json:"reports"`
		Background    bool   `json:"background_tasks"`
		RepairRetries int    `json:"repair_retries"`
	} `json:"agent"`
	Rules []string `json:"rules"`
}

func loadConfig() (*Config, error) {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "nlsh", "config.json")

	config := &Config{
		Engine: "mlx",
		Debug:  false,
		Rules: []string{
			"Prefer modern tools (rg over grep, fd over find, bat over cat).",
			"For recursive file listing, use `rg --files` or `fd --type f`; never use `rg -type f`.",
			"For content search, use `rg PATTERN PATH`.",
			"For finding files by extension, use `fd -e EXT -t f`.",
			"For file viewing, use `cat FILE` or `bat FILE`.",
			"Use fish shell syntax (e.g. for loops).",
			"When running commands on files (like bat/cat/grep), ALWAYS ensure you filter for files only (e.g. fd --type f).",
			"Assume macOS environment.",
		},
	}
	config.MLX.Model = "sahilchachra/ornith-1.0-9b-mxfp4-mlx"
	config.MLX.Command = []string{"uv", "tool", "run", "--from", "mlx-lm", "mlx_lm.generate"}
	config.MLX.ChatCommand = []string{"uv", "tool", "run", "--from", "mlx-lm", "mlx_lm.chat"}
	config.MLX.Server.URL = "http://127.0.0.1:8765"
	config.MLX.Server.Command = []string{"uv", "tool", "run", "--from", "mlx-lm", "mlx_lm.server"}
	config.MLX.Server.AutoStart = true
	config.MLX.Server.ExternalApp = false
	config.MLX.Server.Stream = true
	config.MLX.Server.LogFile = filepath.Join(home, ".config", "nlsh", "mlx-server.log")
	config.MLX.Server.KeepWarmSeconds = 180
	config.MLX.Server.StampFile = filepath.Join(home, ".config", "nlsh", "mlx-server.last_used")
	config.MLX.Server.ReaperPIDFile = filepath.Join(home, ".config", "nlsh", "mlx-server-reaper.pid")
	config.MLX.MaxTokens = 48
	config.MLX.Timeout = 180
	config.Gemini.Model = "gemini-2.0-flash"
	config.Ollama.Host = "http://localhost:11434"
	config.Ollama.Model = "qwen2.5-coder:7b"
	config.Agent.Profile = "read-only"
	config.Agent.SessionDir = filepath.Join(home, ".config", "nlsh", "sessions")
	config.Agent.FastModel = "sahilchachra/ornith-1.0-9b-mxfp4-mlx"
	config.Agent.SmartModel = "sahilchachra/ornith-1.0-9b-mxfp4-mlx"
	config.Agent.Clipboard = true
	config.Agent.Reports = true
	config.Agent.Background = true
	config.Agent.RepairRetries = 1

	// Try to read existing config
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, config)
	} else {
		// Save default if not exists
		os.MkdirAll(filepath.Dir(configPath), 0755)
		data, _ := json.MarshalIndent(config, "", "  ")
		os.WriteFile(configPath, data, 0644)
	}

	// Environment variable overrides
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		config.Gemini.APIKey = key
	}
	if engine := os.Getenv("NLSH_ENGINE"); engine != "" {
		config.Engine = engine
	}
	if model := os.Getenv("NLSH_MLX_MODEL"); model != "" {
		config.MLX.Model = model
	}
	if debug := os.Getenv("NLSH_DEBUG"); debug != "" {
		config.Debug = debug != "0" && strings.ToLower(debug) != "false" && strings.ToLower(debug) != "off"
	}
	if profile := os.Getenv("NLSH_AGENT_PROFILE"); profile != "" {
		config.Agent.Profile = profile
	}
	if model := os.Getenv("NLSH_MLX_FAST_MODEL"); model != "" {
		config.Agent.FastModel = model
	}
	if model := os.Getenv("NLSH_MLX_SMART_MODEL"); model != "" {
		config.Agent.SmartModel = model
	}
	normalizeMLXConfig(config, home)

	return config, nil
}

func logf(config *Config, format string, args ...interface{}) {
	if config == nil || !config.Debug {
		return
	}
	fmt.Fprintf(os.Stderr, "nlsh: "+format+"\n", args...)
}

const (
	cReset   = "\033[0m"
	cBold    = "\033[1m"
	cDim     = "\033[2m"
	cMagenta = "\033[35m"
	cCyan    = "\033[36m"
	cGreen   = "\033[32m"
	cYellow  = "\033[33m"
	cRed     = "\033[31m"
	cBlue    = "\033[34m"
	cGray    = "\033[90m"
)

func color(c, s string) string {
	return c + s + cReset
}

func hudLine(label, value string) {
	fmt.Printf(" │ %s %s\n", color(cGray, label), value)
}

func normalizeMLXConfig(config *Config, home string) {
	if config.MLX.Model == "" || config.MLX.Model == "ornith" {
		config.MLX.Model = "sahilchachra/ornith-1.0-9b-mxfp4-mlx"
	}
	if len(config.MLX.Command) == 0 {
		config.MLX.Command = []string{"uv", "tool", "run", "--from", "mlx-lm", "mlx_lm.generate"}
	}
	if len(config.MLX.ChatCommand) == 0 {
		config.MLX.ChatCommand = []string{"uv", "tool", "run", "--from", "mlx-lm", "mlx_lm.chat"}
	}
	if config.MLX.Server.URL == "" || config.MLX.Server.URL == "http://127.0.0.1:8800" {
		config.MLX.Server.URL = "http://127.0.0.1:8765"
	}
	if len(config.MLX.Server.Command) == 0 || strings.Join(config.MLX.Server.Command, " ") == "open -a vMLX" || strings.Join(config.MLX.Server.Command, " ") == "open -a MLX Studio" {
		config.MLX.Server.Command = []string{"uv", "tool", "run", "--from", "mlx-lm", "mlx_lm.server"}
		config.MLX.Server.ExternalApp = false
	}
	if config.MLX.Server.LogFile == "" {
		config.MLX.Server.LogFile = filepath.Join(home, ".config", "nlsh", "mlx-server.log")
	}
	if config.MLX.Server.KeepWarmSeconds <= 0 {
		config.MLX.Server.KeepWarmSeconds = 180
	}
	if config.MLX.Server.StampFile == "" {
		config.MLX.Server.StampFile = filepath.Join(home, ".config", "nlsh", "mlx-server.last_used")
	}
	if config.MLX.Server.ReaperPIDFile == "" {
		config.MLX.Server.ReaperPIDFile = filepath.Join(home, ".config", "nlsh", "mlx-server-reaper.pid")
	}
	if config.MLX.MaxTokens <= 0 {
		config.MLX.MaxTokens = 48
	}
	if config.MLX.Timeout <= 0 {
		config.MLX.Timeout = 180
	}
	if config.Agent.Profile == "" {
		config.Agent.Profile = "read-only"
	}
	config.Agent.Profile = normalizeAgentProfile(config.Agent.Profile)
	if config.Agent.SessionDir == "" {
		config.Agent.SessionDir = filepath.Join(home, ".config", "nlsh", "sessions")
	}
	if config.Agent.FastModel == "" {
		config.Agent.FastModel = config.MLX.Model
	}
	if config.Agent.SmartModel == "" {
		config.Agent.SmartModel = config.MLX.Model
	}
	if config.Agent.RepairRetries < 0 {
		config.Agent.RepairRetries = 0
	}
	if config.Agent.RepairRetries == 0 {
		config.Agent.RepairRetries = 1
	}
}

func askMLX(config *Config, prompt string) (string, error) {
	if config.MLX.Server.URL != "" {
		touchMLXServer(config)
		logf(config, "mlx engine using server %s", config.MLX.Server.URL)
		if err := ensureMLXServer(config); err != nil {
			return "", err
		}
		if !config.MLX.Server.ExternalApp {
			startIdleReaper(config)
		}
		return askMLXServer(config, prompt)
	}

	systemPrompt := "You convert natural-language terminal intent into exactly one fish shell command. Output only the command. No markdown, no prose, no explanations."
	args := append([]string{}, config.MLX.Command[1:]...)
	args = append(args,
		"--model", config.MLX.Model,
		"--temp", "0",
		"--max-tokens", fmt.Sprint(config.MLX.MaxTokens),
		"--system-prompt", systemPrompt,
		"--prompt", prompt,
		"--verbose", "False",
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.MLX.Timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.MLX.Command[0], args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("MLX timed out after %d seconds", config.MLX.Timeout)
		}
		return "", fmt.Errorf("MLX error: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return extractMLXCommand(stdout.String()), nil
}

func ensureMLXServer(config *Config) error {
	logf(config, "checking MLX server readiness")
	if mlxServerReady(config) {
		logf(config, "MLX server ready")
		return nil
	}
	if !config.MLX.Server.AutoStart {
		return fmt.Errorf("MLX server is not running at %s", config.MLX.Server.URL)
	}
	if len(config.MLX.Server.Command) == 0 {
		return fmt.Errorf("MLX server command is empty")
	}

	args := append([]string{}, config.MLX.Server.Command[1:]...)
	if !config.MLX.Server.ExternalApp {
		args = append(args,
			"--model", config.MLX.Model,
			"--host", "127.0.0.1",
			"--port", mlxServerPort(config),
			"--temp", "0",
			"--max-tokens", fmt.Sprint(config.MLX.MaxTokens),
			"--chat-template-args", `{"enable_thinking":false}`,
			"--log-level", "WARNING",
		)
	}

	if err := os.MkdirAll(filepath.Dir(config.MLX.Server.LogFile), 0755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(config.MLX.Server.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(config.MLX.Server.Command[0], args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start MLX server: %w", err)
	}
	pid := cmd.Process.Pid
	cmd.Process.Release()
	logFile.Close()
	logf(config, "starting local model runtime pid %d model=%s url=%s log=%s", pid, config.MLX.Model, config.MLX.Server.URL, config.MLX.Server.LogFile)

	deadline := time.Now().Add(time.Duration(config.MLX.Timeout) * time.Second)
	nextLog := time.Now()
	for time.Now().Before(deadline) {
		if mlxServerReady(config) {
			logf(config, "MLX server ready")
			return nil
		}
		if time.Now().After(nextLog) {
			logf(config, "waiting for MLX server at %s", config.MLX.Server.URL)
			nextLog = time.Now().Add(5 * time.Second)
		}
		time.Sleep(750 * time.Millisecond)
	}
	return fmt.Errorf("MLX server did not become ready within %d seconds; see %s", config.MLX.Timeout, config.MLX.Server.LogFile)
}

func startExternalRuntime(config *Config) error {
	if len(config.MLX.Server.Command) == 0 {
		return fmt.Errorf("runtime command is empty")
	}
	cmd := exec.Command(config.MLX.Server.Command[0], config.MLX.Server.Command[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	cmd.Process.Release()
	return nil
}

func touchMLXServer(config *Config) {
	if config.MLX.Server.StampFile == "" {
		return
	}
	os.MkdirAll(filepath.Dir(config.MLX.Server.StampFile), 0755)
	now := []byte(strconv.FormatInt(time.Now().Unix(), 10) + "\n")
	os.WriteFile(config.MLX.Server.StampFile, now, 0644)
}

func startIdleReaper(config *Config) {
	if config.MLX.Server.KeepWarmSeconds <= 0 || config.MLX.Server.StampFile == "" || config.MLX.Server.ReaperPIDFile == "" {
		return
	}
	if data, err := os.ReadFile(config.MLX.Server.ReaperPIDFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if processAlive(pid) {
				return
			}
		}
	}
	port := mlxServerPort(config)
	if port == "" {
		return
	}
	script := `stamp="$1"; idle="$2"; port="$3"; log="$4"; while true; do sleep 15; now=$(date +%s); last=$(cat "$stamp" 2>/dev/null || echo 0); age=$((now-last)); if [ "$age" -ge "$idle" ]; then pids=$(lsof -tiTCP:$port -sTCP:LISTEN 2>/dev/null); if [ -n "$pids" ]; then kill $pids 2>/dev/null; echo "$(date) stopped MLX server after ${age}s idle" >> "$log"; fi; exit 0; fi; done`
	cmd := exec.Command("/bin/sh", "-c", script, "nlsh-mlx-reaper", config.MLX.Server.StampFile, fmt.Sprint(config.MLX.Server.KeepWarmSeconds), port, config.MLX.Server.LogFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logf(config, "failed to start idle reaper: %v", err)
		return
	}
	pid := cmd.Process.Pid
	cmd.Process.Release()
	os.WriteFile(config.MLX.Server.ReaperPIDFile, []byte(fmt.Sprint(pid)), 0644)
	logf(config, "MLX keep-warm reaper pid=%d idle=%ds", pid, config.MLX.Server.KeepWarmSeconds)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func mlxServerPort(config *Config) string {
	u, err := url.Parse(config.MLX.Server.URL)
	if err != nil {
		return ""
	}
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

func mlxServerReady(config *Config) bool {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	resp, err := client.Get(strings.TrimRight(config.MLX.Server.URL, "/") + "/v1/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func askMLXServer(config *Config, prompt string) (string, error) {
	system := "You convert natural-language terminal intent into exactly one fish shell command. Output only the command. No markdown, no prose, no explanations."
	return askMLXChat(config, system, prompt, config.MLX.Server.Stream)
}

func askMLXChat(config *Config, systemPrompt, userPrompt string, stream bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.MLX.Timeout)*time.Second)
	defer cancel()

	payload := map[string]interface{}{
		"model": config.MLX.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0,
		"max_tokens":  config.MLX.MaxTokens,
		"stream":      stream,
	}
	jsonData, _ := json.Marshal(payload)
	logf(config, "POST /v1/chat/completions stream=%t max_tokens=%d prompt_bytes=%d", stream, config.MLX.MaxTokens, len(userPrompt))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(config.MLX.Server.URL, "/")+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("MLX server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if stream {
		logf(config, "stream connected")
		return readOpenAIStream(config, resp.Body), nil
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty MLX server response")
	}
	return result.Choices[0].Message.Content, nil
}

func readOpenAIStream(config *Config, body io.Reader) string {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var out strings.Builder
	var reasoning strings.Builder
	reasoningBytes := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			if config.Debug && line != "" && strings.HasPrefix(line, ":") {
				fmt.Fprintf(os.Stderr, "nlsh: mlx %s\n", strings.TrimSpace(strings.TrimPrefix(line, ":")))
			}
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			logf(config, "stream done")
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					Reasoning string `json:"reasoning"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				out.WriteString(choice.Delta.Content)
				fmt.Fprint(os.Stderr, choice.Delta.Content)
			} else if choice.Delta.Reasoning != "" {
				reasoning.WriteString(choice.Delta.Reasoning)
				reasoningBytes += len(choice.Delta.Reasoning)
				if config.Debug && (reasoningBytes == len(choice.Delta.Reasoning) || reasoningBytes%64 < len(choice.Delta.Reasoning)) {
					fmt.Fprintf(os.Stderr, "nlsh: reasoning %d bytes...\n", reasoningBytes)
				}
			}
		}
	}
	if out.Len() > 0 {
		fmt.Fprintln(os.Stderr)
		logf(config, "content stream produced %d bytes", out.Len())
		return out.String()
	}
	command := extractCommandFromText(reasoning.String())
	logf(config, "extracted command from reasoning: %s", command)
	return command
}

func askGemini(config *Config, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		config.Gemini.Model, config.Gemini.APIKey)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.2,
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("no response from Gemini")
}

func askOllama(config *Config, prompt string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", config.Ollama.Host)
	payload := map[string]interface{}{
		"model":  config.Ollama.Model,
		"prompt": prompt,
		"stream": false,
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}

func getSystemInfo() string {
	osName := "macOS" // Default for Darwin
	distro := ""

	// Get OS version on Mac
	if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		distro = "macOS " + strings.TrimSpace(string(out))
	} else {
		// Fallback for Linux
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "NAME=") {
					distro = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
					break
				}
			}
		}
	}

	isRoot := os.Getuid() == 0
	shell := os.Getenv("SHELL")

	return fmt.Sprintf("OS: %s, Distro: %s, Shell: %s, IsRoot: %t", osName, distro, shell, isRoot)
}

type ToolInventory struct {
	Available []string
	Missing   []string
	Aliases   []string
	Functions []string
}

func getToolsStatus() ([]string, []string) {
	inventory := getToolInventory()
	return inventory.Available, inventory.Missing
}

func getToolInventory() ToolInventory {
	important := []string{
		"rg", "cat", "bat", "fd", "find", "grep", "sed", "awk", "jq", "git",
		"gh", "go", "uv", "python3", "node", "npm", "pnpm", "bun", "deno",
		"curl", "wget", "tar", "zip", "unzip", "ffmpeg", "brew", "docker",
		"fzf", "zoxide", "eza", "exa", "ls", "cd", "mkdir", "touch", "cp",
		"mv", "rm", "open", "pbcopy", "pbpaste",
	}
	availableSet := map[string]bool{}
	var missing []string

	for _, tool := range important {
		if _, err := exec.LookPath(tool); err == nil {
			availableSet[tool] = true
		} else {
			missing = append(missing, tool)
		}
	}

	for _, tool := range scanPathExecutables() {
		availableSet[tool] = true
	}
	for _, builtin := range shellBuiltins() {
		availableSet[builtin] = true
	}

	aliases := getFishAliasNames()
	for _, alias := range aliases {
		availableSet[alias] = true
	}
	functions := getFishFunctionNames()
	for _, fn := range functions {
		availableSet[fn] = true
	}

	available := make([]string, 0, len(availableSet))
	for tool := range availableSet {
		available = append(available, tool)
	}
	sort.Strings(available)
	sort.Strings(missing)
	return ToolInventory{Available: available, Missing: missing, Aliases: aliases, Functions: functions}
}

func scanPathExecutables() []string {
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if name == "" || strings.HasPrefix(name, ".") || seen[name] {
				continue
			}
			info, err := entry.Info()
			if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
				continue
			}
			seen[name] = true
		}
	}

	tools := make([]string, 0, len(seen))
	for tool := range seen {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	return tools
}

func shellBuiltins() []string {
	return []string{
		"and", "begin", "break", "builtin", "case", "cd", "command", "continue",
		"else", "end", "eval", "exec", "exit", "for", "function", "if", "not",
		"or", "read", "return", "set", "source", "status", "string", "switch",
		"test", "time", "while", "echo", "pwd", "true", "false",
	}
}

func getAvailableTools() string {
	inventory := getToolInventory()
	return fmt.Sprintf("Installed[%s] MissingImportant[%s] Aliases[%s] Functions[%s]",
		strings.Join(inventory.Available, ", "),
		strings.Join(inventory.Missing, ", "),
		strings.Join(inventory.Aliases, ", "),
		strings.Join(inventory.Functions, ", "),
	)
}

func getFishAliases() string {
	aliases, _ := parseFishConfigSymbols()
	return strings.Join(aliases, "\n")
}

func getFishAliasNames() []string {
	aliases, _ := parseFishConfigSymbols()
	names := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		name := parseFishAliasName(alias)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func getFishFunctionNames() []string {
	_, functions := parseFishConfigSymbols()
	filtered := make([]string, 0, len(functions))
	for _, name := range functions {
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, "fish_") || strings.HasPrefix(name, "__fish") {
			continue
		}
		filtered = append(filtered, name)
	}
	sort.Strings(filtered)
	if len(filtered) > 120 {
		filtered = filtered[:120]
	}
	return filtered
}

func parseFishConfigSymbols() ([]string, []string) {
	home, _ := os.UserHomeDir()
	files := []string{filepath.Join(home, ".config", "fish", "config.fish")}
	functionDir := filepath.Join(home, ".config", "fish", "functions")
	if entries, err := os.ReadDir(functionDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".fish") {
				files = append(files, filepath.Join(functionDir, entry.Name()))
			}
		}
	}

	aliasSet := map[string]bool{}
	functionSet := map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, rawLine := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			for _, part := range strings.Split(line, ";") {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "alias ") {
					aliasSet[part] = true
				}
				if strings.HasPrefix(part, "function ") {
					fields := strings.Fields(part)
					if len(fields) >= 2 {
						name := cleanFishSymbolName(fields[1])
						if name != "" {
							functionSet[name] = true
						}
					}
				}
			}
		}
	}

	aliases := make([]string, 0, len(aliasSet))
	for alias := range aliasSet {
		aliases = append(aliases, alias)
	}
	functions := make([]string, 0, len(functionSet))
	for fn := range functionSet {
		functions = append(functions, fn)
	}
	sort.Strings(aliases)
	sort.Strings(functions)
	return aliases, functions
}

func parseFishAliasName(aliasLine string) string {
	aliasLine = strings.TrimSpace(strings.TrimPrefix(aliasLine, "alias "))
	if aliasLine == "" {
		return ""
	}
	if idx := strings.Index(aliasLine, "="); idx > 0 {
		return cleanFishSymbolName(aliasLine[:idx])
	}
	fields := strings.Fields(aliasLine)
	if len(fields) == 0 {
		return ""
	}
	return cleanFishSymbolName(fields[0])
}

func cleanFishSymbolName(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.Index(name, "("); idx > 0 {
		name = name[:idx]
	}
	return strings.Trim(name, `"'`)
}

func isLikelyCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	firstWord := fields[0]

	// 1. Check if it's an executable path (absolute or relative)
	if strings.Contains(firstWord, "/") {
		if info, err := os.Stat(firstWord); err == nil {
			// It exists, check if it's executable
			if info.Mode()&0111 != 0 && !info.IsDir() {
				return true
			}
		}
		return false
	}

	// 2. Check path for bare commands
	_, err := exec.LookPath(firstWord)
	if err == nil {
		return true
	}

	// 3. Check for common shell builtins
	builtins := map[string]bool{}
	for _, builtin := range shellBuiltins() {
		builtins[builtin] = true
	}
	return builtins[strings.ToLower(firstWord)]
}

func main() {
	// Check for "status" command or no args
	if len(os.Args) < 2 {
		showStatus()
		os.Exit(0)
	}

	if os.Args[1] == "status" {
		showStatus()
		os.Exit(0)
	}

	if os.Args[1] == "dashboard" || os.Args[1] == "hud" || os.Args[1] == "home" {
		if err := showDashboard(); err != nil {
			fmt.Printf("Dashboard error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "doctor" {
		if err := runDoctor(); err != nil {
			fmt.Printf("Doctor error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "sessions" || os.Args[1] == "history" {
		if err := showSessions(); err != nil {
			fmt.Printf("Sessions error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "profile" || os.Args[1] == "mode" {
		if err := manageProfile(os.Args[2:]); err != nil {
			fmt.Printf("Profile error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "warm" {
		if err := warmRuntime(); err != nil {
			fmt.Printf("Warm error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "logs" || os.Args[1] == "log" {
		if err := showLogs(); err != nil {
			fmt.Printf("Log error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "forget" {
		if err := forgetCurrentSession(); err != nil {
			fmt.Printf("Forget error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "models" || os.Args[1] == "model" {
		if err := showModelHUD(); err != nil {
			fmt.Printf("Model HUD error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if os.Args[1] == "--agent" {
		query := strings.TrimSpace(strings.TrimPrefix(strings.Join(os.Args[2:], " "), "!"))
		if err := runAgent(query); err != nil {
			fmt.Printf("Agent error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	query := strings.Join(os.Args[1:], " ")
	query = strings.TrimSpace(query)
	query = strings.TrimPrefix(query, "!")
	query = strings.TrimSpace(query)

	sysInfo := getSystemInfo()
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		os.Exit(1)
	}
	logf(config, "request=%q engine=%s model=%s", query, config.Engine, config.MLX.Model)

	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	rules := strings.Join(config.Rules, "\n- ")

	// Global context support
	globalContext := ""
	if data, err := os.ReadFile(filepath.Join(home, ".config", "nlsh", "context.md")); err == nil {
		globalContext = "\nGlobal User Context:\n" + string(data)
	}

	// Local context support
	localContext := ""
	if data, err := os.ReadFile(filepath.Join(cwd, ".nlsh-context")); err == nil {
		localContext = "\nLocal Project Context:\n" + string(data)
	}

	// Parse available tools for validation
	inventory := getToolInventory()
	available, missing := inventory.Available, inventory.Missing
	aliases := getFishAliases()
	logf(config, "tools available=%d aliases=%d functions=%d missing_important=%d", len(inventory.Available), len(inventory.Aliases), len(inventory.Functions), len(inventory.Missing))

	// First attempt
	prompt := generatePrompt(sysInfo, cwd, globalContext, localContext, rules, query, available, missing, aliases, "")
	command, err := getResponse(config, prompt)
	if err != nil {
		fmt.Printf("API Error: %v\n", err)
		os.Exit(1)
	}

	// Validation Loop (Max 1 retry to avoid latency)
	command = cleanCommand(command)
	logf(config, "candidate=%q", command)
	fields := strings.Fields(command)
	if len(fields) > 0 {
		firstWord := fields[0]

		// Check if suggested tool is unavailable before offering it.
		isMissing := !isKnownCommand(firstWord, inventory)

		if isMissing {
			// Retry with explicit error
			retryMsg := fmt.Sprintf("CRITICAL ERROR: The tool '%s' is NOT installed or known on this system. You MUST use one of the installed tools, shell builtins, functions, or aliases. Prefer rg/cat/bat/fd/git when suitable. Do NOT suggest '%s'.", firstWord, firstWord)
			prompt = generatePrompt(sysInfo, cwd, globalContext, localContext, rules, query, available, missing, aliases, retryMsg)
			command, err = getResponse(config, prompt)
			if err == nil {
				command = cleanCommand(command)
			}
		}
	}

	fmt.Println(command)
}

func runAgent(query string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if query == "" {
		return fmt.Errorf("empty request")
	}

	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	globalContext := ""
	if data, err := os.ReadFile(filepath.Join(home, ".config", "nlsh", "context.md")); err == nil {
		globalContext = "\nGlobal User Context:\n" + string(data)
	}
	localContext := ""
	if data, err := os.ReadFile(filepath.Join(cwd, ".nlsh-context")); err == nil {
		localContext = "\nLocal Project Context:\n" + string(data)
	}
	inventory := getToolInventory()
	rules := strings.Join(config.Rules, "\n- ")
	sessionPath := agentSessionPath(config, cwd)
	history := loadAgentSession(sessionPath)

	printAgentHeader(config, cwd, inventory, sessionPath)

	reader := bufio.NewReader(os.Stdin)
	for {
		turn, err := runAgentTurn(config, query, cwd, globalContext, localContext, rules, inventory, history)
		if err != nil {
			return err
		}
		history = append(history, turn)
		saveAgentSession(sessionPath, history)

		fmt.Print("\n" + color(cMagenta, " follow-up") + color(cGray, " › "))
		next, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return nil
		}
		next = strings.TrimSpace(next)
		if next == "" || next == "q" || next == "quit" || next == "exit" {
			fmt.Println(color(cGray, " session closed"))
			return nil
		}
		query = next
	}
}

type AgentTurn struct {
	Request    string
	Command    string
	ExitCode   int
	Output     string
	Stderr     string
	Answer     string
	DurationMS int64
	Profile    string
	StartedAt  string
	Repaired   bool
	Native     bool
}

func printAgentHeader(config *Config, cwd string, inventory ToolInventory, sessionPath string) {
	fmt.Println()
	fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH-Pro Agent ") + color(cMagenta, strings.Repeat("─", 54)))
	hudLine("model ", color(cCyan, config.MLX.Model))
	hudLine("route ", fmt.Sprintf("%s command  %s answer", color(cCyan, config.Agent.FastModel), color(cCyan, config.Agent.SmartModel)))
	hudLine("cwd   ", color(cGreen, cwd))
	hudLine("tools ", fmt.Sprintf("%s available  %s aliases  %s functions",
		color(cCyan, fmt.Sprint(len(inventory.Available))),
		color(cCyan, fmt.Sprint(len(inventory.Aliases))),
		color(cCyan, fmt.Sprint(len(inventory.Functions))),
	))
	hudLine("mode  ", color(cYellow, config.Agent.Profile+" tools")+color(cGray, " · type q/blank to exit"))
	hudLine("memory", color(cGray, sessionPath))
	fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))
}

func runAgentTurn(config *Config, query, cwd, globalContext, localContext, rules string, inventory ToolInventory, history []AgentTurn) (AgentTurn, error) {
	turn := AgentTurn{Request: query, Profile: config.Agent.Profile, StartedAt: time.Now().Format(time.RFC3339)}

	fmt.Println()
	fmt.Println(color(cCyan, " ◆ Request ") + color(cBold, query))
	printPlanner(query, history, config)

	if handled, turn, err := runNativeAgentTool(config, query, cwd, history); handled {
		return turn, err
	}

	commandPrompt := generateAgentCommandPrompt(getSystemInfo(), cwd, globalContext, localContext, rules, query, inventory.Available, inventory.Missing, getFishAliases(), history, config.Agent.Profile)
	originalStream := config.MLX.Server.Stream
	originalModel := config.MLX.Model
	config.MLX.Server.Stream = false
	config.MLX.Model = routeModel(config, "command")
	command, err := getResponse(config, commandPrompt)
	config.MLX.Model = originalModel
	config.MLX.Server.Stream = originalStream
	if err != nil {
		return turn, err
	}
	command = repairCommonCommand(cleanCommand(command), query)
	turn.Command = command
	printToolStart(command, "model="+routeModel(config, "command"))

	if err := validateAgentCommand(command, inventory, config.Agent.Profile); err != nil {
		repaired := repairInvalidAgentCommand(command, query, err)
		if repaired == "" || repaired == command {
			return turn, err
		}
		turn.Repaired = true
		fmt.Println(color(cYellow, " ◆ Repair ") + color(cGray, err.Error()) + color(cGray, " -> ") + color(cBold, repaired))
		command = repaired
		turn.Command = command
		if err := validateAgentCommand(command, inventory, config.Agent.Profile); err != nil {
			return turn, err
		}
	}

	result := runAgentCommand(command)
	if result.Err != nil && config.Agent.RepairRetries > 0 {
		if repaired := repairFailedAgentCommand(command, query, result); repaired != "" && repaired != command {
			fmt.Println(color(cYellow, " ◆ Retry ") + color(cGray, summarizeToolFailure(result)) + color(cGray, " -> ") + color(cBold, repaired))
			if err := validateAgentCommand(repaired, inventory, config.Agent.Profile); err == nil {
				command = repaired
				turn.Command = command
				turn.Repaired = true
				result = runAgentCommand(command)
			}
		}
	}
	turn.Output = result.Output
	turn.Stderr = result.Stderr
	turn.ExitCode = result.ExitCode
	turn.DurationMS = result.Duration.Milliseconds()
	printToolTimeline(result)
	printToolResult(result.Output)

	answerPrompt := fmt.Sprintf(`User request: %s
Working directory: %s
Tool called: %s
Exit code: %d
Duration: %s
Tool output:
%s

Conversation history:
%s

Give a concise final answer. If the output is a list of files, summarize how many and show the paths. If there are no results, say that plainly. No markdown fences.`, query, cwd, command, result.ExitCode, result.Duration.Round(time.Millisecond), truncateText(result.Output, 8000), formatAgentHistory(history))

	fmt.Println(color(cGreen, " ◆ Answer"))
	answer := deterministicAnswer(query, command, result.Output, cwd)
	if answer == "" {
		originalModel := config.MLX.Model
		config.MLX.Model = routeModel(config, "answer")
		answer, err = askMLXAgentAnswer(config, answerPrompt)
		config.MLX.Model = originalModel
		if err != nil {
			return turn, err
		}
	}
	turn.Answer = strings.TrimSpace(answer)
	fmt.Println(indentBlock(turn.Answer, "   "))
	return turn, nil
}

func deterministicAnswer(query, command, output, cwd string) string {
	lower := strings.ToLower(query)
	lines := nonEmptyLines(output)
	if len(lines) == 0 {
		return ""
	}
	if strings.Contains(lower, "mp4") && strings.Contains(command, "fd -e mp4") {
		return fmt.Sprintf("Found %d mp4 files in %s.", len(lines), cwd)
	}
	return ""
}

type ToolResult struct {
	Command   string
	Output    string
	Stderr    string
	ExitCode  int
	Err       error
	Duration  time.Duration
	StartedAt time.Time
}

func printPlanner(query string, history []AgentTurn, config *Config) {
	fmt.Println(color(cYellow, " ◆ Plan"))
	fmt.Println("   " + color(cGray, "1.") + " classify request and reuse session context")
	fmt.Println("   " + color(cGray, "2.") + fmt.Sprintf(" choose %s-safe tool or native inspector", config.Agent.Profile))
	fmt.Println("   " + color(cGray, "3.") + " run, repair once if needed, then answer")
	if len(history) > 0 {
		fmt.Println("   " + color(cGray, "history") + fmt.Sprintf(" %d prior turn(s) for this directory", len(history)))
	}
	_ = query
}

func printToolStart(command, detail string) {
	if detail != "" {
		fmt.Println(color(cBlue, " ◆ Tool ") + color(cBold, command) + color(cGray, " · "+detail))
		return
	}
	fmt.Println(color(cBlue, " ◆ Tool ") + color(cBold, command))
}

func printToolTimeline(result ToolResult) {
	exitColor := cGreen
	if result.ExitCode != 0 {
		exitColor = cRed
	}
	fmt.Printf("   %s %s  %s %s  %s %s\n",
		color(cGray, "exit"), color(exitColor, fmt.Sprint(result.ExitCode)),
		color(cGray, "time"), color(cCyan, result.Duration.Round(time.Millisecond).String()),
		color(cGray, "bytes"), color(cCyan, fmt.Sprint(len(result.Output)+len(result.Stderr))),
	)
	if result.Stderr != "" {
		fmt.Println("   " + color(cGray, "stderr ") + truncateOneLine(result.Stderr, 180))
	}
	if result.Err != nil && result.ExitCode != 0 {
		fmt.Println("   " + color(cGray, "error  ") + truncateOneLine(result.Err.Error(), 180))
	}
}

func truncateOneLine(s string, max int) string {
	s = strings.Join(nonEmptyLines(s), " ")
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func routeModel(config *Config, purpose string) string {
	if purpose == "command" && config.Agent.FastModel != "" {
		return config.Agent.FastModel
	}
	if purpose == "answer" && config.Agent.SmartModel != "" {
		return config.Agent.SmartModel
	}
	return config.MLX.Model
}

func normalizeAgentProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "confirm", "confirm-write", "write":
		return "confirm-write"
	case "power", "power-mode":
		return "power"
	default:
		return "read-only"
	}
}

func agentSessionPath(config *Config, cwd string) string {
	sum := sha1.Sum([]byte(cwd))
	name := hex.EncodeToString(sum[:])[:16] + ".json"
	return filepath.Join(config.Agent.SessionDir, name)
}

func loadAgentSession(path string) []AgentTurn {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var history []AgentTurn
	if err := json.Unmarshal(data, &history); err != nil {
		return nil
	}
	if len(history) > 30 {
		return history[len(history)-30:]
	}
	return history
}

func saveAgentSession(path string, history []AgentTurn) {
	if path == "" {
		return
	}
	if len(history) > 50 {
		history = history[len(history)-50:]
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.MarshalIndent(history, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, append(data, '\n'), 0644)
	}
}

func generateAgentCommandPrompt(sysInfo, cwd, global, local, rules, query string, available, missing []string, aliases string, history []AgentTurn, profile string) string {
	toolsStr := fmt.Sprintf("InstalledPriority[%s] TotalInstalled[%d] MissingImportant[%s]", strings.Join(prioritizedPromptTools(available), ", "), len(available), strings.Join(missing, ", "))
	return fmt.Sprintf(`Choose exactly one SAFE READ-ONLY command to answer the user's request.
Output ONLY the command. No markdown. No prose.

Target: macOS / fish shell.
System: %s
Tools: %s
Aliases:
%s
Context: %s%s%s
Safety profile: %s
Rules:
- Read-only commands only.
- Allowed command families: fd, rg, find, ls, cat, bat, head, tail, wc, du, pwd, git status, git log, git diff.
- Do not use rm, mv, cp, chmod, chown, sudo, kill, curl, network tools, redirects, pipes, semicolons, command substitution, or eval.
- Do not use xargs, sort pipelines, or shell pipes. If the user asks follow-up analysis like longest video, rely on prior tool output or native agent tools.
- For finding files by extension, use fd -e EXT -t f .
- For mp4 videos in the current tree, use fd -e mp4 -t f .
- For recursive file listing, use rg --files or fd --type f .
- %s

Conversation history:
%s

User request: %s`, sysInfo, toolsStr, aliases, cwd, global, local, profile, rules, formatAgentHistory(history), query)
}

func askMLXAgentAnswer(config *Config, prompt string) (string, error) {
	touchMLXServer(config)
	if err := ensureMLXServer(config); err != nil {
		return "", err
	}
	if !config.MLX.Server.ExternalApp {
		startIdleReaper(config)
	}
	system := "You are a concise terminal agent. Answer the user based only on the tool output. Do not invent files."
	return askMLXChat(config, system, prompt, false)
}

type VideoInfo struct {
	Path    string
	Seconds float64
	Size    int64
}

func runNativeAgentTool(config *Config, query, cwd string, history []AgentTurn) (bool, AgentTurn, error) {
	turn := AgentTurn{Request: query, Profile: config.Agent.Profile, StartedAt: time.Now().Format(time.RFC3339), Native: true}
	lower := strings.ToLower(query)
	if handled, nativeTurn := runHistoryAction(config, query, cwd, history, turn); handled {
		return true, nativeTurn, nil
	}
	if handled, nativeTurn := runInspectorTool(query, cwd, turn); handled {
		return true, nativeTurn, nil
	}
	if handled, nativeTurn := runBackgroundAgentTool(config, query, cwd, turn); handled {
		return true, nativeTurn, nil
	}
	if !(strings.Contains(lower, "longest") && strings.Contains(lower, "video")) {
		return false, turn, nil
	}

	files := lastMP4Files(history)
	if len(files) == 0 {
		files = findMP4Files(cwd)
	}
	turn.Command = "native:video-duration-scan"
	start := time.Now()
	printToolStart(turn.Command, fmt.Sprintf("%d file(s)", len(files)))
	if len(files) == 0 {
		turn.Output = ""
		turn.ExitCode = 0
		turn.DurationMS = time.Since(start).Milliseconds()
		printToolTimeline(ToolResult{Command: turn.Command, ExitCode: 0, Duration: time.Since(start), StartedAt: start})
		printToolResult("")
		turn.Answer = "I could not find any mp4 videos in this session or the current directory."
		fmt.Println(color(cGreen, " ◆ Answer"))
		fmt.Println(indentBlock(turn.Answer, "   "))
		return true, turn, nil
	}

	infos := probeVideos(cwd, files)
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Seconds > infos[j].Seconds
	})
	var b strings.Builder
	for _, info := range infos {
		fmt.Fprintf(&b, "%s\t%s\t%s\n", formatDuration(info.Seconds), humanBytes(info.Size), info.Path)
	}
	turn.Output = b.String()
	turn.ExitCode = 0
	turn.DurationMS = time.Since(start).Milliseconds()
	printToolTimeline(ToolResult{Command: turn.Command, Output: turn.Output, ExitCode: 0, Duration: time.Since(start), StartedAt: start})
	printToolResult(turn.Output)

	fmt.Println(color(cGreen, " ◆ Answer"))
	if len(infos) == 0 || infos[0].Seconds <= 0 {
		turn.Answer = "I found videos, but ffprobe could not read their durations."
	} else {
		turn.Answer = fmt.Sprintf("The longest video is %s at %s (%s).", infos[0].Path, formatDuration(infos[0].Seconds), humanBytes(infos[0].Size))
	}
	fmt.Println(indentBlock(turn.Answer, "   "))
	return true, turn, nil
}

func runHistoryAction(config *Config, query, cwd string, history []AgentTurn, turn AgentTurn) (bool, AgentTurn) {
	lower := strings.ToLower(query)
	if len(history) == 0 {
		return false, turn
	}
	last := history[len(history)-1]
	content := strings.TrimSpace(last.Answer)
	if content == "" {
		content = strings.TrimSpace(last.Output)
	}
	if strings.Contains(lower, "copy") && (strings.Contains(lower, "last") || strings.Contains(lower, "that") || strings.Contains(lower, "answer") || strings.Contains(lower, "result")) {
		turn.Command = "native:clipboard-copy-last"
		start := time.Now()
		printToolStart(turn.Command, "pbcopy")
		if !config.Agent.Clipboard {
			turn.ExitCode = 1
			turn.Stderr = "clipboard actions disabled"
			turn.Answer = "Clipboard actions are disabled in the NLSH config."
		} else {
			cmd := exec.Command("pbcopy")
			cmd.Stdin = strings.NewReader(content)
			err := cmd.Run()
			if err != nil {
				turn.ExitCode = 1
				turn.Stderr = err.Error()
				turn.Answer = "I could not copy the last result to the clipboard."
			} else {
				turn.ExitCode = 0
				turn.Output = fmt.Sprintf("copied %d bytes", len(content))
				turn.Answer = "Copied the last result to the clipboard."
			}
		}
		turn.DurationMS = time.Since(start).Milliseconds()
		printToolTimeline(ToolResult{Command: turn.Command, Output: turn.Output, Stderr: turn.Stderr, ExitCode: turn.ExitCode, Duration: time.Since(start), StartedAt: start})
		printToolResult(turn.Output)
		fmt.Println(color(cGreen, " ◆ Answer"))
		fmt.Println(indentBlock(turn.Answer, "   "))
		return true, turn
	}
	if strings.Contains(lower, "save") && (strings.Contains(lower, "report") || strings.Contains(lower, "markdown") || strings.Contains(lower, "md")) {
		turn.Command = "native:save-markdown-report"
		start := time.Now()
		printToolStart(turn.Command, "markdown")
		if !config.Agent.Reports {
			turn.ExitCode = 1
			turn.Stderr = "report actions disabled"
			turn.Answer = "Report actions are disabled in the NLSH config."
		} else {
			name := fmt.Sprintf("nlsh-report-%s.md", time.Now().Format("20060102-150405"))
			path := filepath.Join(cwd, name)
			body := fmt.Sprintf("# NLSH Report\n\nRequest: %s\n\n## Answer\n\n%s\n\n## Tool\n\n```text\n%s\n```\n", last.Request, content, last.Output)
			if err := os.WriteFile(path, []byte(body), 0644); err != nil {
				turn.ExitCode = 1
				turn.Stderr = err.Error()
				turn.Answer = "I could not save the report."
			} else {
				turn.ExitCode = 0
				turn.Output = path
				turn.Answer = "Saved a markdown report at " + path
			}
		}
		turn.DurationMS = time.Since(start).Milliseconds()
		printToolTimeline(ToolResult{Command: turn.Command, Output: turn.Output, Stderr: turn.Stderr, ExitCode: turn.ExitCode, Duration: time.Since(start), StartedAt: start})
		printToolResult(turn.Output)
		fmt.Println(color(cGreen, " ◆ Answer"))
		fmt.Println(indentBlock(turn.Answer, "   "))
		return true, turn
	}
	return false, turn
}

func runInspectorTool(query, cwd string, turn AgentTurn) (bool, AgentTurn) {
	lower := strings.ToLower(query)
	if !(strings.Contains(lower, "inspect") || strings.Contains(lower, "metadata") || strings.Contains(lower, "info about") || strings.Contains(lower, "summarize project") || strings.Contains(lower, "project metadata")) {
		return false, turn
	}
	start := time.Now()
	target := extractInspectionTarget(query)
	if target == "" || strings.Contains(lower, "project") || strings.Contains(lower, "git") {
		turn.Command = "native:project-inspector"
		printToolStart(turn.Command, "git/files")
		turn.Output = inspectProject(cwd)
		turn.ExitCode = 0
		turn.Answer = "Project inspection complete."
	} else {
		turn.Command = "native:file-inspector " + target
		printToolStart(turn.Command, target)
		turn.Output = inspectPath(cwd, target)
		turn.ExitCode = 0
		turn.Answer = "Inspection complete for " + target + "."
	}
	turn.DurationMS = time.Since(start).Milliseconds()
	printToolTimeline(ToolResult{Command: turn.Command, Output: turn.Output, ExitCode: turn.ExitCode, Duration: time.Since(start), StartedAt: start})
	printToolResult(turn.Output)
	fmt.Println(color(cGreen, " ◆ Answer"))
	fmt.Println(indentBlock(turn.Answer, "   "))
	return true, turn
}

func runBackgroundAgentTool(config *Config, query, cwd string, turn AgentTurn) (bool, AgentTurn) {
	lower := strings.ToLower(query)
	if !(strings.Contains(lower, "background") || strings.Contains(lower, "tell me when") || strings.Contains(lower, "watch ")) {
		return false, turn
	}
	start := time.Now()
	turn.Command = "native:background-task"
	printToolStart(turn.Command, "detached")
	if !config.Agent.Background {
		turn.ExitCode = 1
		turn.Stderr = "background tasks disabled"
		turn.Answer = "Background tasks are disabled in the NLSH config."
	} else {
		logPath := filepath.Join(os.TempDir(), "nlsh-background-"+time.Now().Format("20060102-150405")+".log")
		script := backgroundScriptFromQuery(query)
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			turn.ExitCode = 1
			turn.Stderr = err.Error()
			turn.Answer = "I could not create a background task log."
		} else {
			cmd := exec.Command("/bin/sh", "-lc", script)
			cmd.Dir = cwd
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			if err := cmd.Start(); err != nil {
				turn.ExitCode = 1
				turn.Stderr = err.Error()
				turn.Answer = "I could not start the background task."
			} else {
				turn.ExitCode = 0
				turn.Output = fmt.Sprintf("pid %d\nlog %s\ncommand %s", cmd.Process.Pid, logPath, script)
				turn.Answer = fmt.Sprintf("Started background task pid %d. Log: %s", cmd.Process.Pid, logPath)
			}
			_ = logFile.Close()
		}
	}
	turn.DurationMS = time.Since(start).Milliseconds()
	printToolTimeline(ToolResult{Command: turn.Command, Output: turn.Output, Stderr: turn.Stderr, ExitCode: turn.ExitCode, Duration: time.Since(start), StartedAt: start})
	printToolResult(turn.Output)
	fmt.Println(color(cGreen, " ◆ Answer"))
	fmt.Println(indentBlock(turn.Answer, "   "))
	return true, turn
}

func extractInspectionTarget(query string) string {
	fields := strings.Fields(query)
	stop := map[string]bool{
		"inspect": true, "metadata": true, "info": true, "about": true, "for": true,
		"the": true, "this": true, "file": true, "video": true, "audio": true, "image": true, "pdf": true,
	}
	for i := len(fields) - 1; i >= 0; i-- {
		word := strings.Trim(fields[i], "\"'")
		lower := strings.ToLower(word)
		if word == "" || stop[lower] {
			continue
		}
		if strings.Contains(word, ".") || strings.Contains(word, "/") {
			return word
		}
	}
	return ""
}

func inspectPath(cwd, target string) string {
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, target)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "path: %s\n", path)
	if stat, err := os.Stat(path); err == nil {
		fmt.Fprintf(&b, "size: %s\n", humanBytes(stat.Size()))
		fmt.Fprintf(&b, "mode: %s\n", stat.Mode())
		fmt.Fprintf(&b, "modified: %s\n", stat.ModTime().Format(time.RFC3339))
	} else {
		fmt.Fprintf(&b, "stat_error: %s\n", err)
	}
	if out, err := exec.Command("file", path).Output(); err == nil {
		fmt.Fprintf(&b, "file: %s", string(out))
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp4", ".mov", ".m4v", ".mp3", ".wav", ".m4a", ".aac", ".flac":
		appendCommandOutput(&b, "ffprobe", "ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_type,codec_name,width,height", "-of", "default=nw=1", path)
	case ".png", ".jpg", ".jpeg", ".gif", ".heic", ".webp":
		appendCommandOutput(&b, "image", "sips", "-g", "pixelWidth", "-g", "pixelHeight", "-g", "format", path)
	case ".pdf":
		if _, err := exec.LookPath("pdfinfo"); err == nil {
			appendCommandOutput(&b, "pdf", "pdfinfo", path)
		}
	}
	return b.String()
}

func inspectProject(cwd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "cwd: %s\n", cwd)
	appendCommandOutput(&b, "pwd", "pwd")
	appendCommandOutputDir(&b, cwd, "git", "git", "status", "--short", "--branch")
	appendCommandOutputDir(&b, cwd, "files", "sh", "-lc", "find . -maxdepth 2 -type f | sed 's#^./##' | head -40")
	appendCommandOutputDir(&b, cwd, "dirs", "sh", "-lc", "find . -maxdepth 2 -type d | sed 's#^./##' | head -40")
	return b.String()
}

func appendCommandOutput(b *strings.Builder, label, name string, args ...string) {
	appendCommandOutputDir(b, "", label, name, args...)
}

func appendCommandOutputDir(b *strings.Builder, dir, label, name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		fmt.Fprintf(b, "%s_error: %s\n", label, err)
		return
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		text = "(no output)"
	}
	fmt.Fprintf(b, "%s:\n%s\n", label, text)
}

func backgroundScriptFromQuery(query string) string {
	lower := strings.ToLower(query)
	for _, marker := range []string{"tell me when", "background", "run"} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			rest := strings.TrimSpace(query[idx+len(marker):])
			rest = strings.TrimPrefix(rest, "the")
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "command"))
			if rest != "" && !strings.ContainsAny(rest, "\n;&|`$<>") {
				return rest
			}
		}
	}
	if strings.Contains(lower, "build") {
		if _, err := os.Stat("package.json"); err == nil {
			return "npm run build"
		}
	}
	return "sleep 1"
}

func lastMP4Files(history []AgentTurn) []string {
	for i := len(history) - 1; i >= 0; i-- {
		var files []string
		for _, line := range nonEmptyLines(history[i].Output) {
			if strings.HasPrefix(line, "[stderr]") {
				break
			}
			lower := strings.ToLower(line)
			if strings.HasSuffix(lower, ".mp4") {
				files = append(files, line)
			}
		}
		if len(files) > 0 {
			return files
		}
	}
	return nil
}

func findMP4Files(cwd string) []string {
	cmd := exec.Command("fd", "-e", "mp4", "-t", "f", ".")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return nonEmptyLines(string(out))
}

func probeVideos(cwd string, files []string) []VideoInfo {
	var infos []VideoInfo
	for _, file := range files {
		info := VideoInfo{Path: file}
		fullPath := file
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(cwd, file)
		}
		if stat, err := os.Stat(fullPath); err == nil {
			info.Size = stat.Size()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", fullPath)
		out, err := cmd.Output()
		cancel()
		if err == nil {
			if seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
				info.Seconds = seconds
			}
		}
		infos = append(infos, info)
	}
	return infos
}

func formatDuration(seconds float64) string {
	total := int(seconds + 0.5)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func humanBytes(size int64) string {
	if size <= 0 {
		return "-"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", size, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func printToolResult(output string) {
	lines := nonEmptyLines(output)
	fmt.Println(color(cBlue, " ◆ Result ") + color(cGray, fmt.Sprintf("%d line(s)", len(lines))))
	if len(lines) == 0 {
		fmt.Println("   " + color(cGray, "no output"))
		return
	}
	limit := 18
	for i, line := range lines {
		if i >= limit {
			fmt.Printf("   %s\n", color(cGray, fmt.Sprintf("... %d more", len(lines)-limit)))
			return
		}
		fmt.Printf("   %s %s\n", color(cGray, fmt.Sprintf("%2d", i+1)), line)
	}
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func formatAgentHistory(history []AgentTurn) string {
	if len(history) == 0 {
		return "(none)"
	}
	start := 0
	if len(history) > 4 {
		start = len(history) - 4
	}
	var b strings.Builder
	for i, turn := range history[start:] {
		fmt.Fprintf(&b, "%d. request: %s\n", i+1, turn.Request)
		fmt.Fprintf(&b, "   tool: %s\n", turn.Command)
		fmt.Fprintf(&b, "   exit: %d\n", turn.ExitCode)
		fmt.Fprintf(&b, "   answer: %s\n", truncateText(turn.Answer, 500))
	}
	return b.String()
}

func repairCommonCommand(command, query string) string {
	lower := strings.ToLower(query)
	if strings.Contains(lower, "mp4") && (strings.Contains(lower, "find") || strings.Contains(lower, "video")) {
		return "fd -e mp4 -t f ."
	}
	if strings.Contains(command, "rg --files") && strings.Contains(command, "*.mp4") {
		return "fd -e mp4 -t f ."
	}
	return command
}

func validateAgentCommand(command string, inventory ToolInventory, profile string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty tool command")
	}
	banned := []string{";", "&&", "||", "|", ">", "<", "`", "$(", "\n"}
	if profile == "power" {
		banned = []string{"`", "$(", "\n"}
	}
	for _, bad := range banned {
		if strings.Contains(command, bad) {
			return fmt.Errorf("refusing shell metacharacter %q in %q", bad, command)
		}
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("empty tool command")
	}
	readOnly := map[string]bool{
		"fd": true, "rg": true, "find": true, "ls": true, "cat": true, "bat": true,
		"head": true, "tail": true, "wc": true, "du": true, "pwd": true, "git": true,
	}
	writeAllowed := map[string]bool{
		"mkdir": true, "touch": true, "cp": true, "mv": true, "open": true, "pbcopy": true,
	}
	allowed := readOnly[fields[0]] || (profile != "read-only" && writeAllowed[fields[0]])
	if !allowed && profile != "power" {
		return fmt.Errorf("refusing non-read-only tool %q", fields[0])
	}
	if fields[0] == "git" && len(fields) > 1 {
		gitAllowed := map[string]bool{"status": true, "log": true, "diff": true, "show": true}
		if !gitAllowed[fields[1]] && profile != "power" {
			return fmt.Errorf("refusing mutating git subcommand %q", fields[1])
		}
	}
	if !isKnownCommand(fields[0], inventory) {
		return fmt.Errorf("tool %q is not installed", fields[0])
	}
	return nil
}

func runAgentCommand(command string) ToolResult {
	start := time.Now()
	fields := strings.Fields(command)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result := ToolResult{Command: command, StartedAt: start}
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result.Output = stdout.String()
	result.Stderr = stderr.String()
	result.Err = err
	result.Duration = time.Since(start)
	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		result.ExitCode = 124
		result.Err = ctx.Err()
		return result
	}
	result.ExitCode = exitCode
	return result
}

func repairInvalidAgentCommand(command, query string, cause error) string {
	_ = cause
	return repairCommonCommand(command, query)
}

func repairFailedAgentCommand(command, query string, result ToolResult) string {
	lowerOutput := strings.ToLower(result.Output + "\n" + result.Stderr)
	repaired := repairCommonCommand(command, query)
	if repaired != command {
		return repaired
	}
	if strings.Contains(command, "rg --files") && (strings.Contains(lowerOutput, "no such file") || strings.Contains(lowerOutput, "os error 2")) {
		return "fd -e mp4 -t f ."
	}
	return ""
}

func summarizeToolFailure(result ToolResult) string {
	text := result.Stderr
	if text == "" {
		text = result.Output
	}
	if text == "" && result.Err != nil {
		text = result.Err.Error()
	}
	return truncateOneLine(text, 120)
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... truncated %d bytes ...", len(s)-max)
}

func indentBlock(s, prefix string) string {
	if strings.TrimSpace(s) == "" {
		return prefix + "(no output)"
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func isKnownCommand(name string, inventory ToolInventory) bool {
	if name == "" {
		return false
	}
	if strings.Contains(name, "/") {
		info, err := os.Stat(name)
		return err == nil && !info.IsDir() && info.Mode()&0111 != 0
	}
	if _, err := exec.LookPath(name); err == nil {
		return true
	}
	for _, builtin := range shellBuiltins() {
		if name == builtin {
			return true
		}
	}
	for _, alias := range inventory.Aliases {
		if name == alias {
			return true
		}
	}
	for _, fn := range inventory.Functions {
		if name == fn {
			return true
		}
	}
	return false
}

func getResponse(config *Config, prompt string) (string, error) {
	switch config.Engine {
	case "mlx":
		return askMLX(config, prompt)
	case "ollama":
		return askOllama(config, prompt)
	case "gemini":
		if config.Gemini.APIKey == "" {
			fmt.Fprintf(os.Stderr, "GEMINI_API_KEY not found. Attempting local MLX link...\n")
			config.Engine = "mlx"
			return askMLX(config, prompt)
		}
		return askGemini(config, prompt)
	default:
		return "", fmt.Errorf("unknown engine %q; expected mlx, gemini, or ollama", config.Engine)
	}
}

func generatePrompt(sysInfo, cwd, global, local, rules, query string, available, missing []string, aliases, extraInstructions string) string {
	toolsStr := fmt.Sprintf("InstalledPriority[%s] TotalInstalled[%d] MissingImportant[%s]", strings.Join(prioritizedPromptTools(available), ", "), len(available), strings.Join(missing, ", "))

	return fmt.Sprintf(`Convert this user request into a shell command.
Rules:
1. Output ONLY the command. No markdown. No backticks. No comments.
2. Target: macOS / fish shell.
3. System Info: %s
4. Tools Status: %s
5. Valid User Aliases:
%s
6. Context: %s%s%s
7. CRITICAL RULES:
- DO NOT use tools listed in "MissingImportant".
- IF a requested tool is missing, substitute it with an available alternative (e.g. use 'rg' instead of grep when available, 'cat' or 'bat' for file viewing).
- You MAY use any command listed in InstalledPriority, any shell builtin, and any listed fish alias/function.
- Use valid syntax for each tool: recursive file listing is rg --files or fd --type f, content search is rg PATTERN, and file display is cat FILE or bat FILE.
- For finding files by extension such as mp4, mov, png, or pdf, prefer fd -e EXT -t f.
- CONSIDER using a user's alias if one matches the intent.
- %s
- %s

User typed: %s

Note: If the user input is ALREADY a valid command, return it as is.`, sysInfo, toolsStr, aliases, cwd, global, local, rules, extraInstructions, query)
}

func prioritizedPromptTools(available []string) []string {
	want := []string{
		"rg", "cat", "bat", "fd", "find", "grep", "sed", "awk", "jq", "git", "gh",
		"go", "uv", "python3", "node", "npm", "pnpm", "curl", "tar", "zip", "unzip",
		"ffmpeg", "brew", "docker", "fzf", "zoxide", "eza", "ls", "cd", "mkdir",
		"touch", "cp", "mv", "rm", "open", "pbcopy", "pbpaste",
	}
	availableSet := map[string]bool{}
	for _, tool := range available {
		availableSet[tool] = true
	}
	var out []string
	for _, tool := range want {
		if availableSet[tool] {
			out = append(out, tool)
		}
	}
	return out
}

func cleanCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = extractMLXCommand(cmd)
	cmd = extractCommandFromText(cmd)
	cmd = strings.TrimPrefix(cmd, "```bash")
	cmd = strings.TrimPrefix(cmd, "```fish")
	cmd = strings.TrimPrefix(cmd, "```")
	cmd = strings.TrimSuffix(cmd, "```")
	return strings.TrimSpace(cmd)
}

func extractCommandFromText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.Count(text, "`") >= 2 {
		parts := strings.Split(text, "`")
		for i := 1; i < len(parts); i += 2 {
			candidate := strings.TrimSpace(parts[i])
			if candidate != "" && looksLikeShellCommand(candidate) {
				return candidate
			}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "$"))
		if looksLikeShellCommand(line) {
			return line
		}
	}
	return text
}

func looksLikeShellCommand(line string) bool {
	if line == "" || strings.Contains(line, " ") == false {
		return isLikelyCommand(line)
	}
	first := strings.Fields(line)[0]
	if strings.Contains(first, "/") {
		return true
	}
	return isLikelyCommand(first)
}

func extractMLXCommand(output string) string {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	lines := strings.Split(output, "\n")
	var candidates []string
	for _, line := range lines {
		line = strings.TrimSpace(stripANSI(line))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "assistant:") {
			line = strings.TrimSpace(line[len("assistant:"):])
			lower = strings.ToLower(line)
			if line == "" {
				continue
			}
		}
		if strings.HasPrefix(lower, "system:") ||
			strings.HasPrefix(lower, "user:") ||
			strings.HasPrefix(lower, "prompt:") ||
			strings.HasPrefix(lower, "========") ||
			strings.Contains(lower, "help:") {
			continue
		}
		if strings.HasPrefix(line, ">") {
			line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		}
		candidates = append(candidates, line)
	}
	if len(candidates) == 0 {
		return strings.TrimSpace(stripANSI(output))
	}
	return candidates[len(candidates)-1]
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inEscape {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEscape = false
			}
			continue
		}
		if c == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func showDashboard() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	inventory := getToolInventory()
	sessions := recentSessionSummaries(config, 5)
	ready := mlxServerReady(config)

	fmt.Println()
	fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH Command Center ") + color(cMagenta, strings.Repeat("─", 51)))
	hudLine("runtime", runtimeBadge(ready)+" "+color(cCyan, config.MLX.Server.URL))
	hudLine("model  ", color(cCyan, config.MLX.Model))
	hudLine("profile", profileBadge(config.Agent.Profile))
	hudLine("cwd    ", color(cGreen, cwd))
	hudLine("tools  ", fmt.Sprintf("%s available  %s aliases  %s functions", color(cCyan, fmt.Sprint(len(inventory.Available))), color(cCyan, fmt.Sprint(len(inventory.Aliases))), color(cCyan, fmt.Sprint(len(inventory.Functions)))))
	fmt.Println(color(cMagenta, " ├"+strings.Repeat("─", 72)))
	hudLine("quick  ", color(cGray, "nlsh-pro doctor  ·  nlsh-pro sessions  ·  nlsh-pro warm"))
	hudLine("agent  ", color(cGray, "!ask anything  ·  copy that  ·  save report  ·  inspect project"))
	hudLine("safety ", color(cGray, "nlsh-pro profile read-only|confirm-write|power"))
	if len(sessions) > 0 {
		fmt.Println(color(cMagenta, " ├"+strings.Repeat("─", 72)))
		for i, session := range sessions {
			hudLine(fmt.Sprintf("mem %d ", i+1), fmt.Sprintf("%s %s %s", color(cCyan, fmt.Sprintf("%d turn(s)", session.Turns)), color(cGray, session.Age), session.LastRequest))
		}
	}
	fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))
	return nil
}

func runDoctor() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	checks := []DoctorCheck{
		checkCommand("nlsh-pro", "/opt/homebrew/bin/nlsh-pro"),
		checkCommand("uv", "uv"),
		checkCommand("mlx-lm server", "uv"),
		checkCommand("fd", "fd"),
		checkCommand("rg", "rg"),
		checkCommand("ffprobe", "ffprobe"),
		checkPath("fish hook", filepath.Join(os.Getenv("HOME"), ".config", "fish", "functions", "fish_command_not_found.fish")),
		checkPath("config", filepath.Join(os.Getenv("HOME"), ".config", "nlsh", "config.json")),
		checkDir("session dir", config.Agent.SessionDir),
		{Name: "mlx server", OK: mlxServerReady(config), Detail: config.MLX.Server.URL},
		{Name: "local context", OK: pathExists(filepath.Join(cwd, ".nlsh-context")), Detail: filepath.Join(cwd, ".nlsh-context")},
	}

	fmt.Println()
	fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH Doctor ") + color(cMagenta, strings.Repeat("─", 58)))
	for _, check := range checks {
		status := color(cRed, "fail")
		if check.OK {
			status = color(cGreen, "ok  ")
		}
		hudLine(status, fmt.Sprintf("%-14s %s", check.Name, color(cGray, check.Detail)))
	}
	fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))
	return nil
}

type DoctorCheck struct {
	Name   string
	OK     bool
	Detail string
}

func checkCommand(name, command string) DoctorCheck {
	path, err := exec.LookPath(command)
	if err != nil {
		return DoctorCheck{Name: name, OK: false, Detail: command + " not found"}
	}
	return DoctorCheck{Name: name, OK: true, Detail: path}
}

func checkPath(name, path string) DoctorCheck {
	return DoctorCheck{Name: name, OK: pathExists(path), Detail: path}
}

func checkDir(name, path string) DoctorCheck {
	info, err := os.Stat(path)
	return DoctorCheck{Name: name, OK: err == nil && info.IsDir(), Detail: path}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type SessionSummary struct {
	Path        string
	Turns       int
	LastRequest string
	LastAnswer  string
	UpdatedAt   time.Time
	Age         string
}

func showSessions() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	sessions := recentSessionSummaries(config, 20)
	fmt.Println()
	fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH Sessions ") + color(cMagenta, strings.Repeat("─", 56)))
	if len(sessions) == 0 {
		hudLine("empty ", color(cGray, "no saved agent sessions yet"))
	} else {
		for i, session := range sessions {
			hudLine(fmt.Sprintf("%2d   ", i+1), fmt.Sprintf("%s  %s  %s", color(cCyan, fmt.Sprintf("%d turn(s)", session.Turns)), color(cGray, session.Age), session.LastRequest))
			if session.LastAnswer != "" {
				hudLine("     ", color(cGray, truncateOneLine(session.LastAnswer, 96)))
			}
			hudLine("file ", color(cGray, session.Path))
		}
	}
	fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))
	return nil
}

func recentSessionSummaries(config *Config, limit int) []SessionSummary {
	entries, err := os.ReadDir(config.Agent.SessionDir)
	if err != nil {
		return nil
	}
	var sessions []SessionSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(config.Agent.SessionDir, entry.Name())
		history := loadAgentSession(path)
		if len(history) == 0 {
			continue
		}
		info, err := entry.Info()
		updated := time.Now()
		if err == nil {
			updated = info.ModTime()
		}
		last := history[len(history)-1]
		sessions = append(sessions, SessionSummary{
			Path:        path,
			Turns:       len(history),
			LastRequest: truncateOneLine(last.Request, 84),
			LastAnswer:  truncateOneLine(last.Answer, 120),
			UpdatedAt:   updated,
			Age:         formatAge(time.Since(updated)),
		})
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	if limit > 0 && len(sessions) > limit {
		return sessions[:limit]
	}
	return sessions
}

func manageProfile(args []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		fmt.Println()
		fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH Safety Profiles ") + color(cMagenta, strings.Repeat("─", 49)))
		for _, profile := range []string{"read-only", "confirm-write", "power"} {
			marker := " "
			if config.Agent.Profile == profile {
				marker = "*"
			}
			hudLine(marker+" "+profile, profileDescription(profile))
		}
		fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))
		return nil
	}
	profile := normalizeAgentProfile(args[0])
	config.Agent.Profile = profile
	if err := saveConfig(config); err != nil {
		return err
	}
	fmt.Println(color(cGreen, "selected profile ") + profile)
	return nil
}

func warmRuntime() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	fmt.Println(color(cMagenta, "warming mlx runtime ") + color(cGray, config.MLX.Server.URL))
	if err := ensureMLXServer(config); err != nil {
		return err
	}
	if !config.MLX.Server.ExternalApp {
		startIdleReaper(config)
	}
	fmt.Println(color(cGreen, "runtime ready"))
	return nil
}

func showLogs() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(config.MLX.Server.LogFile)
	if err != nil {
		fmt.Println()
		fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH Runtime Log ") + color(cMagenta, strings.Repeat("─", 52)))
		hudLine("file  ", color(cGray, config.MLX.Server.LogFile))
		hudLine("state ", color(cYellow, "no log yet"))
		hudLine("hint  ", color(cGray, "run `nlsh-pro warm` or use agent mode to start the runtime"))
		fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := 0
	if len(lines) > 80 {
		start = len(lines) - 80
	}
	fmt.Println()
	fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH Runtime Log ") + color(cMagenta, strings.Repeat("─", 52)))
	hudLine("file  ", color(cGray, config.MLX.Server.LogFile))
	fmt.Println(color(cMagenta, " ├"+strings.Repeat("─", 72)))
	for _, line := range lines[start:] {
		fmt.Println(" │ " + line)
	}
	fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))
	return nil
}

func forgetCurrentSession() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	path := agentSessionPath(config, cwd)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println(color(cGreen, "forgot session ") + color(cGray, path))
	return nil
}

func runtimeBadge(ready bool) string {
	if ready {
		return color(cGreen, "ready")
	}
	return color(cYellow, "cold ")
}

func profileBadge(profile string) string {
	switch profile {
	case "read-only":
		return color(cGreen, profile)
	case "confirm-write":
		return color(cYellow, profile)
	case "power":
		return color(cRed, profile)
	default:
		return color(cGray, profile)
	}
}

func profileDescription(profile string) string {
	switch profile {
	case "read-only":
		return color(cGray, "safe discovery tools only")
	case "confirm-write":
		return color(cGray, "small write-capable set for files/apps")
	case "power":
		return color(cGray, "wide local command access with shell-substitution guardrails")
	default:
		return color(cGray, "unknown")
	}
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func showStatus() {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	sysInfo := getSystemInfo()

	// Check for local context
	cwd, _ := os.Getwd()
	localContextFound := "❌ No"
	if _, err := os.Stat(filepath.Join(cwd, ".nlsh-context")); err == nil {
		localContextFound = "✅ Yes"
	}

	fmt.Println("\n\033[1;35m 🌌 NLSH-PRO | NEURAL LINK STATUS \033[0m")
	fmt.Println("\033[38;5;238m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Printf(" 📡 Engine:         \033[1;36m%s\033[0m\n", config.Engine)
	if config.Engine == "mlx" {
		fmt.Printf(" 🧠 Model:          \033[33m%s\033[0m\n", config.MLX.Model)
		fmt.Printf(" 🛠️  Server:         %s\n", config.MLX.Server.URL)
		fmt.Printf(" 🌊 Streaming:      %t\n", config.MLX.Server.Stream)
		fmt.Printf(" 🚀 Server Cmd:     %s\n", strings.Join(config.MLX.Server.Command, " "))
		if len(config.MLX.ChatCommand) > 0 {
			fmt.Printf(" 💬 Chat:           %s --model %s\n", strings.Join(config.MLX.ChatCommand, " "), config.MLX.Model)
		}
	} else if config.Engine == "ollama" {
		fmt.Printf(" 🧠 Model:          \033[33m%s\033[0m\n", config.Ollama.Model)
		fmt.Printf(" 🔗 Host:           %s\n", config.Ollama.Host)
	} else {
		fmt.Printf(" 🧠 Model:          \033[33m%s\033[0m\n", config.Gemini.Model)
		maskedKey := "Not Set"
		if len(config.Gemini.APIKey) > 8 {
			maskedKey = config.Gemini.APIKey[:4] + "..." + config.Gemini.APIKey[len(config.Gemini.APIKey)-4:]
		}
		fmt.Printf(" 🔑 API Key:        %s\n", maskedKey)
	}
	fmt.Println("\033[38;5;238m────────────────────────────────────────────────────────────\033[0m")
	fmt.Printf(" 💻 System:         %s\n", sysInfo)

	// Check global context
	globalContextFound := "❌ No"
	home, _ := os.UserHomeDir()
	if _, err := os.Stat(filepath.Join(home, ".config", "nlsh", "context.md")); err == nil {
		globalContextFound = "✅ Yes"
	}
	fmt.Printf(" 🌍 Global Context: %s\n", globalContextFound)
	fmt.Printf(" 📂 Local Context:  %s\n", localContextFound)
	fmt.Printf(" 🧭 Agent Profile:  %s\n", config.Agent.Profile)
	fmt.Printf(" 🧠 Agent Routing:  fast=%s smart=%s\n", config.Agent.FastModel, config.Agent.SmartModel)
	fmt.Printf(" 🗃️  Agent Memory:   %s\n", config.Agent.SessionDir)
	fmt.Println("\033[38;5;238m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Println(" \033[3m\"Ready to interface.\"\033[0m")
}

type ModelOption struct {
	Source string
	Name   string
}

func showModelHUD() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	var options []ModelOption
	if len(fetchOpenAIModels(config.MLX.Server.URL)) == 0 && config.MLX.Server.AutoStart {
		if config.MLX.Server.ExternalApp {
			_ = startExternalRuntime(config)
			time.Sleep(2 * time.Second)
		} else {
			_ = ensureMLXServer(config)
		}
	}
	for _, model := range fetchOpenAIModels(config.MLX.Server.URL) {
		options = append(options, ModelOption{Source: "MLX", Name: model})
	}
	for _, model := range fetchOllamaModels(config.Ollama.Host) {
		options = append(options, ModelOption{Source: "Ollama", Name: model})
	}

	fmt.Println()
	fmt.Println(color(cMagenta, " ╭─") + color(cBold+cMagenta, " NLSH Model HUD ") + color(cMagenta, strings.Repeat("─", 56)))
	hudLine("runtime", color(cCyan, config.MLX.Server.URL))
	hudLine("active ", color(cGreen, config.MLX.Model))
	fmt.Println(color(cMagenta, " ╰"+strings.Repeat("─", 72)))

	if len(options) == 0 {
		fmt.Println(color(cYellow, "No models found."))
		fmt.Println(color(cGray, "Start the mlx-lm server, then run `nlsh-pro models` again."))
		return nil
	}
	for i, option := range options {
		marker := " "
		if option.Name == config.MLX.Model {
			marker = "*"
		}
		fmt.Printf(" %s %s %s %s\n",
			color(cGray, fmt.Sprintf("%2d.", i+1)),
			color(cMagenta, marker),
			color(cCyan, option.Source),
			option.Name,
		)
	}
	fmt.Print("\n" + color(cMagenta, " choose model") + color(cGray, " › "))
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		fmt.Println(color(cGray, "no change"))
		return nil
	}
	idx, err := strconv.Atoi(text)
	if err != nil || idx < 1 || idx > len(options) {
		return fmt.Errorf("invalid selection %q", text)
	}
	selected := options[idx-1]
	if selected.Source == "Ollama" {
		fmt.Println(color(cYellow, "Ollama model selected, but mlx-lm cannot run Ollama/GGUF models directly."))
		fmt.Println(color(cGray, "Choose an MLX-compatible model to use the MLX runtime."))
		return nil
	}
	config.MLX.Model = selected.Name
	if err := saveConfig(config); err != nil {
		return err
	}
	fmt.Println(color(cGreen, "selected ") + selected.Name)
	return nil
}

func fetchOpenAIModels(baseURL string) []string {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/v1/models")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}
	var models []string
	for _, item := range result.Data {
		if item.ID != "" {
			models = append(models, item.ID)
		}
	}
	sort.Strings(models)
	return models
}

func fetchOllamaModels(host string) []string {
	client := &http.Client{Timeout: 1000 * time.Millisecond}
	resp, err := client.Get(strings.TrimRight(host, "/") + "/api/tags")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}
	var models []string
	for _, item := range result.Models {
		if item.Name != "" {
			models = append(models, item.Name)
		}
	}
	sort.Strings(models)
	return models
}

func saveConfig(config *Config) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "nlsh", "config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
