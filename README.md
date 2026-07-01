# NLSH-Pro (Natural Language Shell Pro)

**The Ultimate VibeRot for Your Terminal.**

> "Vibecoding brainrotted my brain, I cant even remember how to run simple cli commands anymore. halp!" — Anton Vice

**NLSH-Pro** is a hyper-optimized, Go-based AI interceptor for the Fish shell. It completely removes the friction between your thoughts and your terminal actions. Unlike other tools that wrap your shell or require special modes, NLSH-Pro is **invisible** until you need it.

---

## ✨ Features

- **🚀 Zero-Friction Interception**: Automatically catches typos and "command not found" errors.
- **🧠 Managed MLX-LM Runtime**: Starts `mlx_lm.server` through `uv tool run --from mlx-lm` on `http://127.0.0.1:8765` and keeps the model warm between requests.
- **🕹️ Agent Mode**: Type `!task` to enter an interactive HUD with request history, tool calls, results, and follow-up prompts.
- **🧭 Model HUD**: Run `nlsh-pro models` to list OpenAI-compatible MLX server models, show Ollama models for comparison, and choose the active MLX model.
- **🧰 Native Agent Tools**: Includes media/project/file inspectors, background task launch, clipboard copy, markdown report output, and per-directory session memory.
- **🧯 Repair Loop**: Records stdout/stderr/duration for each tool call and retries common command failures once.
- **🌊 Streaming Output**: Reads server-sent events from the local model runtime and streams tokens while preserving safe Fish capture.
- **🌍 Context Awareness**:
  - **Dynamic Tool Detection**: Scans your PATH plus Fish aliases/functions so it can suggest real tools like `rg`, `cat`, `bat`, `fd`, `git`, and your own shell helpers.
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

Restart your shell: `exec fish`

### Manual Install (Go)

```bash
go install github.com/antonvice/nlsh-pro@latest
```

## 🎮 Usage

1. **Automatic Fix**: Just type a command. If it fails, NLSH-Pro intervenes.

   ```bash
   > record data
   (Command not found) -> ✨ AI suggests: ffmpeg -f avfoundation -i "1" out.mov
   ```

2. **Force AI**: Prefix with `!`

   ```bash
   > !find all mp4 videos in here
   ```

3. **Choose a model**:

   ```bash
   > nlsh-pro models
   ```

4. **Check Status**:

   ```bash
   > nlsh-pro status
   ```

---

## ⚙️ Configuration

- **Config File**: `~/.config/nlsh/config.json`
- **Global Context**: `~/.config/nlsh/context.md` (Add your preferences here, e.g., "Always use git status -sb")
- **Project Context**: `.nlsh-context` in any directory.
- **Environment**:
  - `NLSH_ENGINE`: Force `mlx`, `gemini`, or `ollama`.
  - `NLSH_MLX_MODEL`: Override the MLX model.
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
    "fast_model": "sahilchachra/ornith-1.0-9b-mxfp4-mlx",
    "smart_model": "sahilchachra/ornith-1.0-9b-mxfp4-mlx",
    "repair_retries": 1
  }
}
```

## 🧙 Cool Factor & Status

Run `nlsh-pro status` to see diagnostics.

Developed by **Anton Vice**.
*Maximum VibeRot Achieved.*
