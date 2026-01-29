# NLSH-Pro (Natural Language Shell Pro)

**The Ultimate VibeRot for Your Terminal.**

> "Vibecoding brainrotted my brain, I cant even remember how to run simple cli commands anymore. halp!" — Anton Vice

**NLSH-Pro** is a hyper-optimized, Go-based AI interceptor for the Fish shell. It completely removes the friction between your thoughts and your terminal actions. Unlike other tools that wrap your shell or require special modes, NLSH-Pro is **invisible** until you need it.

---

## ✨ Features

- **🚀 Zero-Friction Interception**: Automatically catches typos and "command not found" errors.
- **🧠 Dual-Engine Support**: Seamlessly switches between **Gemini Pro** (Cloud) and **Ollama** (Local/Privacy-Focused).
- **🌍 Context Awareness**: 
  - **Dynamic Tool Detection**: Automatically detects installed tools (like `exa`, `bat`, `rg`) and avoids suggesting missing ones.
  - **Project Context**: Reads `.nlsh-context` in your current directory.
  - **Global Context**: Reads `~/.config/nlsh/context.md` for user-wide preferences.
- **⚡ Force Mode**: Type `!task` to bypass detection and force AI reasoning (e.g., `!explain this folder`).
- **🛡️ Safety First**: Doesn't auto-execute dangerous commands; always asks for confirmation.

## � Installation

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
   > !how do I find large files?
   ```
3. **Check Status**:
   ```bash
   > nlsh-pro status
   ```

---

## ⚙️ Configuration

- **Config File**: `~/.config/nlsh/config.json`
- **Global Context**: `~/.config/nlsh/context.md` (Add your preferences here, e.g., "Always use git status -sb")
- **Project Context**: `.nlsh-context` in any directory.
- **Environment**: 
  - `GEMINI_API_KEY`: Set your key here (or via config).
  - `NLSH_ENGINE`: Force `gemini` or `ollama`.

## 🧙 Cool Factor & Status

Run `nlsh-pro status` to see diagnostics.

Developed by **Anton Vice**.
*Maximum VibeRot Achieved.*
