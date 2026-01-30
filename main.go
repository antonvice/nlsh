package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	Engine string `json:"engine"`
	Gemini struct {
		APIKey string `json:"api_key"`
		Model  string `json:"model"`
	} `json:"gemini"`
	Ollama struct {
		Model string `json:"model"`
		Host  string `json:"host"`
	} `json:"ollama"`
	Rules []string `json:"rules"`
}

func loadConfig() (*Config, error) {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "nlsh", "config.json")
	
	config := &Config{
		Engine: "gemini",
		Rules: []string{
			"Prefer modern tools (rg over grep, fd over find, bat over cat).",
			"Use fish shell syntax (e.g. for loops).",
			"When running commands on files (like bat/cat/grep), ALWAYS ensure you filter for files only (e.g. fd --type f).",
			"Assume macOS environment.",
		},
	}
	config.Gemini.Model = "gemini-2.0-flash"
	config.Ollama.Host = "http://localhost:11434"
	config.Ollama.Model = "qwen2.5-coder:7b"

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

	return config, nil
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

	return fmt.Sprintf("OS: %s, Distro: %s, Shell: %s, IsRoot: %t, Tools: %s", osName, distro, shell, isRoot, getAvailableTools())
}

func getToolsStatus() ([]string, []string) {
	tools := []string{"exa", "eza", "bat", "rg", "fd", "fzf", "zoxide", "nvim", "code", "git"}
	var found []string
	var missing []string
	
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			found = append(found, tool)
		} else {
			missing = append(missing, tool)
		}
	}
	return found, missing
}

func getAvailableTools() string {
	found, missing := getToolsStatus()
	return fmt.Sprintf("Installed[%s] Missing[%s]", strings.Join(found, ", "), strings.Join(missing, ", "))
}

func getFishAliases() string {
	cmd := exec.Command("fish", "-c", "alias")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
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
	builtins := map[string]bool{
		"cd": true, "ls": true, "git": true, "docker": true, "npm": true, 
		"node": true, "python": true, "pip": true, "brew": true, "rm": true,
		"cp": true, "mv": true, "mkdir": true, "touch": true, "cat": true,
		"echo": true, "exit": true, "source": true,
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
	available, missing := getToolsStatus()
	aliases := getFishAliases()
	
	// First attempt
	prompt := generatePrompt(sysInfo, cwd, globalContext, localContext, rules, query, available, missing, aliases, "")
	command, err := getResponse(config, prompt)
	if err != nil {
		fmt.Printf("API Error: %v\n", err)
		os.Exit(1)
	}
	
	// Validation Loop (Max 1 retry to avoid latency)
	command = cleanCommand(command)
	fields := strings.Fields(command)
	if len(fields) > 0 {
		firstWord := fields[0]
		
		// Check if suggested tool is missing
		isMissing := false
		for _, m := range missing {
			if m == firstWord {
				isMissing = true
				break
			}
		}
		
		if isMissing {
			// Retry with explicit error
			retryMsg := fmt.Sprintf("CRITICAL ERROR: The tool '%s' is NOT installed on this system. You MUST use a standard alternative like 'ls', 'cd', 'find', or 'grep'. Do NOT suggest '%s'.", firstWord, firstWord)
			prompt = generatePrompt(sysInfo, cwd, globalContext, localContext, rules, query, available, missing, aliases, retryMsg)
			command, err = getResponse(config, prompt)
			if err == nil {
				command = cleanCommand(command)
			}
		}
	}

	fmt.Println(command)
}

func getResponse(config *Config, prompt string) (string, error) {
	if config.Engine == "ollama" {
		return askOllama(config, prompt)
	}
	
	if config.Gemini.APIKey == "" {
		fmt.Fprintf(os.Stderr, "⚠️  GEMINI_API_KEY not found. Attempting local link via Ollama...\n")
		config.Engine = "ollama"
		if config.Ollama.Model == "" || config.Ollama.Model == "llama3" {
			config.Ollama.Model = "qwen2.5-coder:7b"
		}
		return askOllama(config, prompt)
	}
	
	return askGemini(config, prompt)
}

func generatePrompt(sysInfo, cwd, global, local, rules, query string, available, missing []string, aliases, extraInstructions string) string {
	toolsStr := fmt.Sprintf("Installed[%s] Missing[%s]", strings.Join(available, ", "), strings.Join(missing, ", "))
	
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
- DO NOT use tools listed in "Missing".
- IF a requested tool is missing, substitute it with an available alternative (e.g. use 'ls' if 'exa' is missing).
- CONSIDER using a user's alias if one matches the intent.
- %s
- %s

User typed: %s

Note: If the user input is ALREADY a valid command, return it as is.`, sysInfo, toolsStr, aliases, cwd, global, local, rules, extraInstructions, query)
}

func cleanCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimPrefix(cmd, "```bash")
	cmd = strings.TrimPrefix(cmd, "```fish")
	cmd = strings.TrimPrefix(cmd, "```")
	cmd = strings.TrimSuffix(cmd, "```")
	return strings.TrimSpace(cmd)
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
	if config.Engine == "ollama" {
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
	fmt.Println("\033[38;5;238m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Println(" \033[3m\"Ready to interface.\"\033[0m")
}
