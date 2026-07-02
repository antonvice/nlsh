# NLSHP (Natural Language Shell Pro)

**The Ultimate VibeRot for Your Terminal.**

> "Vibecoding brainrotted my brain, I cant even remember how to run simple cli commands anymore. halp!" — Anton Vice

**NLSHP** is a hyper-optimized, Go-based AI interceptor for your shell. It completely removes the friction between your thoughts and your terminal actions. Unlike other tools that wrap your shell or require special modes, NLSHP is **invisible** until you need it.

---

## ✨ Features

- **🚀 Zero-Friction Interception**: Automatically catches typos and "command not found" errors.
- **🧠 Managed MLX-LM Runtime**: Starts `mlx_lm.server` through `uv tool run --from mlx-lm` on `http://127.0.0.1:8765` and keeps the model warm between requests.
- **🕹️ Agent Mode**: Type `!task` to enter an interactive HUD with request history, tool calls, results, and follow-up prompts.
- **🧭 Model HUD**: Run `nlshp models` to list OpenAI-compatible MLX server models, show Ollama models for comparison, and choose the active MLX model.
- **🧰 Native Agent Tools**: Includes media/project/file inspectors, background task launch, clipboard copy, markdown report output, and per-directory session memory.
- **🧯 Repair Loop**: Records stdout/stderr/duration for each tool call and retries common command failures once.
- **🌊 Streaming Output**: Reads server-sent events from the local model runtime and streams tokens while preserving safe shell capture.
- **🌍 Context Awareness**:
  - **Dynamic Tool Detection**: Scans your PATH plus active-shell aliases/functions from Fish, Zsh, or Bash so it can suggest real tools like `rg`, `cat`, `bat`, `fd`, `git`, and your own shell helpers.
  - **Project Context**: Reads `.nlsh-context` in your current directory.
  - **Global Context**: Reads `~/.config/nlsh/context.md` for user-wide preferences.
- **🛡️ Safety First**: Agent mode defaults to read-only tools and records what it ran.

## Installation

### Automatic Install (Recommended)

```bash
git clone https://github.com/antonvice/nlsh-pro
cd nlsh-pro
./install-pro.sh
```

Restart your shell.

### Manual Install (Go)

```bash
git clone https://github.com/antonvice/nlsh-pro
cd nlsh-pro
go build -o nlshp .
install -m 0755 nlshp ~/.local/bin/nlshp
```

## 🎮 Usage

1. **Automatic Fix**: Just type a command. If it fails, NLSHP intervenes.

   ```bash
   > record data
   (Command not found) -> ✨ AI suggests: ffmpeg -f avfoundation -i "1" out.mov
   ```

2. **Force AI**: Prefix with `!`

   ```bash
   > !find all mp4 videos in here
   ```

   Agent mode keeps a per-directory session, shows the plan, runs safe tools, prints a tool timeline, and then waits for follow-up questions.

   Useful follow-ups:

   ```bash
   follow-up › which is longest video?
   follow-up › inspect project metadata
   follow-up › copy that
   follow-up › save report
   ```

3. **Choose a model**:

   ```bash
   > nlshp models
   ```

4. **Check Status**:

   ```bash
   > nlshp status
   ```

5. **Open the command center**:

   ```bash
   > nlshp dashboard
   ```

   The dashboard shows runtime readiness, active model, active shell, safety profile, current directory, available tools, quick commands, and recent agent memory.

6. **See every command**:

   ```bash
   > nlshp help
   ```

---

## ⚙️ Configuration

- **Config File**: `~/.config/nlsh/config.json`
- **Global Context**: `~/.config/nlsh/context.md` (Add your preferences here, e.g., "Always use git status -sb")
- **Project Context**: `.nlsh-context` in any directory.
- **Agent Sessions**: `~/.config/nlsh/sessions/*.json` stores recent turns per working directory.
- **Environment**:
  - `NLSH_ENGINE`: Force `mlx`, `gemini`, or `ollama`.
  - `NLSH_MLX_MODEL`: Override the MLX model.
  - `NLSH_AGENT_PROFILE`: Set `read-only`, `confirm-write`, or `power`.
  - `NLSH_MLX_FAST_MODEL`: Override the model used for command planning.
  - `NLSH_MLX_SMART_MODEL`: Override the model used for final answers.
  - `NLSH_SHELL`: Override shell context detection with `fish`, `zsh`, or `bash`.
  - `GEMINI_API_KEY`: Set your key here only if using the Gemini engine.

Default model runtime:

```json
{
  "mlx": {
    "model": "sahilchachra/ornith-1.0-9b-mxfp4-mlx",
    "server": {
      "url": "http://127.0.0.1:8765",
      "command": ["uv", "tool", "run", "--from", "mlx-lm", "mlx_lm.server"],
      "auto_start": true,
      "external_app": false,
      "stream": true
    }
  },
  "agent": {
    "profile": "read-only",
    "session_dir": "~/.config/nlsh/sessions",
    "fast_model": "sahilchachra/ornith-1.0-9b-mxfp4-mlx",
    "smart_model": "sahilchachra/ornith-1.0-9b-mxfp4-mlx",
    "clipboard": true,
    "reports": true,
    "background_tasks": true,
    "repair_retries": 1
  },
  "shell": {
    "preferred": "fish"
  }
}
```

### Agent Mode

Agent mode is designed for terminal replacement workflows. It shows:

- **Plan**: a visible checklist for request classification, tool choice, execution, and answer.
- **Tool Timeline**: command, exit code, runtime, output size, stderr preview, and repair/retry notes.
- **Result Preview**: numbered output lines with truncation for long results.
- **Answer**: a concise final answer grounded in the tool output.
- **Memory**: recent turns are saved by working directory and reused for follow-ups.

### Safety Profiles

- `read-only`: default. Allows safe discovery tools like `fd`, `rg`, `ls`, `cat`, `bat`, `du`, `wc`, and read-only `git` commands.
- `confirm-write`: permits a small write-capable set such as `mkdir`, `touch`, `cp`, `mv`, `open`, and `pbcopy`.
- `power`: wider command access while still blocking dangerous shell substitution/newline patterns.

Set a profile for one shell:

```bash
set -x NLSH_AGENT_PROFILE confirm-write
```

### Native Agent Tools

Some workflows bypass the LLM command planner and run native helpers:

- **Media duration**: `which is longest video?`
- **File/media metadata**: `inspect path/to/file.mp4`
- **Project metadata**: `inspect project metadata`
- **Clipboard**: `copy that`
- **Markdown report**: `save report`
- **Background task**: `tell me when npm run build finishes`

### Command Center

NLSHP includes a few terminal-native HUDs and utilities:

- `nlshp help`: complete command reference.
- `nlshp dashboard`: command center with runtime, profile, shell, tool, and memory status.
- `nlshp doctor`: health check for binary, shell config, tool context, sessions, context, and MLX server readiness.
- `nlshp sessions`: recent per-directory agent memory.
- `nlshp profile`: list safety profiles.
- `nlshp profile read-only`: switch safety profile.
- `nlshp warm`: start the managed MLX server before the first agent request.
- `nlshp logs`: show the tail of the MLX runtime log.
- `nlshp forget`: delete the saved agent session for the current directory.

### Shell Context

NLSHP prefers Fish shell context when Fish is configured, then falls back to the current `$SHELL`.
You can override that with:

```bash
set -x NLSH_SHELL zsh
```

or in `~/.config/nlsh/config.json`:

```json
{
  "shell": {
    "preferred": "zsh"
  }
}
```

The active shell context pulls:

- Executables from every directory in `PATH`.
- Shell builtins for Fish, Zsh, or Bash.
- Aliases and functions from active shell config files.
- Project context from `.nlsh-context`.
- Global context from `~/.config/nlsh/context.md`.

### Model Routing

NLSH can use different local models for different agent stages:

- `fast_model`: command planning.
- `smart_model`: final answer synthesis.

Both default to the managed MLX model. Use `nlshp models` to inspect the OpenAI-compatible MLX server model list and compare with Ollama models.

## 🧙 Cool Factor & Status

Run `nlshp status` or `nlshp help` to see diagnostics.

Developed by **Anton Vice**.
*Maximum VibeRot Achieved.*
