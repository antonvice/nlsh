# NLSH-Pro (Natural Language Shell Pro)

**The Ultimate Neural Link for Your Terminal.**

> "Vibecoding brainrotted my brain, I cant even remember how to run simple cli commands anymore. halp!" — Anton Vice

**NLSH-Pro** is a hyper-optimized, Go-based AI interceptor for the Fish shell. It completely removes the friction between your thoughts and your terminal actions. Unlike other tools that wrap your shell or require special modes, NLSH-Pro is **invisible** until you need it.

---

## ⚡ Why is this "The Ultimate"?

I (Anton Vice) analyzed the best natural language shell tools out there and synthesized their strengths into one zero-friction binary:

1.  **Zero-Friction Interception**:
    *   **Command Not Found?** Automatically intercepted. The AI fixes your typo or generates the command you meant to type.
    *   **Intention Mode**: Type `!list all large files` to force the AI to take over instantly.
    *   **Neural Link**: Press **Alt+L** (Option+L) to transform your current buffer into a shell command.

2.  **Dual-Engine Core**:
    *   **Cloud Power**: Supports Google Gemini 2.0 Flash for reasoning.
    *   **Local Privacy**: Automatically falls back to **Ollama (qwen2.5-coder)** if no API key is present. It works offline, right out of the box.

3.  **Context-Aware Intelligence**:
    *   **System Sensing**: Automatically detects your OS, Distro, Shell, and Root status.
    *   **Project Context**: Drop a `.nlsh-context` file in any directory to teach the AI about your specific repo rules.

4.  **Premium Experience**:
    *   Cyberpunk-inspired "Orbital Link" UI.
    *   Non-blocking scan animations.
    *   Smart path validation (no more errors on directories).

---

## 🛠 Installation

### Prerequisites
- **Go** (for building the binary)
- **Fish Shell** (the superior shell)

### One-Step Install
Run the installer to build the binary and register the Fish plugins:

```bash
chmod +x install-pro.sh
./install-pro.sh
```

Restart your shell: `source ~/.config/fish/config.fish`

---

## 🎮 Usage Guide

### 1. The "Oops" Workflow (Automatic)
Just type what you think works. If it's not a command, NLSH-Pro catches it.
```fish
$ list all py files sorted by size
⚡ Command 'list' not found. Routing to NLSH-Pro...
✨ AI suggests: fd -e py -x stat -f "%z %N" | sort -n
```

### 2. The "Force" Workflow (Explicit)
Prefix any sentence with `!` to skip command detection.
```fish
$ !find headers in src but exclude test files
🌌 NLSH-Pro | Force-AI Mode Engaged
✨ AI suggests: rg "header" src -g "!*test*"
```

### 3. The "Link" Workflow (Interactive)
Type `check status`, realize you don't know the git command, and press **Alt+L**.
The text is instantly replaced by `git status -sb`.

---

## ⚙️ Configuration

Your config lives at `~/.config/nlsh/config.json`.

```json
{
  "engine": "gemini", 
  "gemini": {
    "api_key": "YOUR_KEY_HERE",
    "model": "gemini-2.0-flash"
  },
  "rules": [
    "Prefer modern tools (rg over grep, fd over find, bat over cat).",
    "Use fish shell syntax."
  ]
}
```

*Note: If `api_key` is empty, it auto-switches to Ollama.*

---

## 🧙 Cool Factor & Status

Run `nlsh-pro status` to see your Neural Link diagnostics (Connectivity, active model, and local context).

Developed by **Anton Vice**.
*Maximum Coolness Achieved.*
