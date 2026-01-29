#!/bin/bash
set -e

echo "🚀 Installing NLSH-Pro (The Ultimate NLSH)..."

# Build Go tool
GO_BIN=$(command -v go || echo "/usr/local/go/bin/go")

if [ -f "$GO_BIN" ]; then
    "$GO_BIN" build -o nlsh-pro main.go
    mkdir -p ~/.local/bin
    cp nlsh-pro ~/.local/bin/
    echo "✅ Go binary built and installed to ~/.local/bin/nlsh-pro"
else
    echo "❌ Go not found in PATH or /usr/local/go/bin/go. Please install Go. If you just installed it, you might need to add it to your PATH."
    exit 1
fi

# Set up Fish integration
FISH_CONF_DIR="$HOME/.config/fish/functions"
mkdir -p "$FISH_CONF_DIR"

# Copy functions
cp functions/__nlsh_pro_invoke.fish "$FISH_CONF_DIR/"
cp functions/fish_command_not_found.fish "$FISH_CONF_DIR/"
echo "✅ Fish functions installed to $FISH_CONF_DIR"

# Add binding to fish config if not present
BIND_CONFIG="
# NLSH-Pro Bindings
if status is-interactive
    bind \el __nlsh_pro_invoke
    bind \e\r __nlsh_pro_invoke
end"

if ! grep -q "__nlsh_pro_invoke" ~/.config/fish/config.fish 2>/dev/null; then
    echo "$BIND_CONFIG" >> ~/.config/fish/config.fish
    echo "✅ Fish bindings added to ~/.config/fish/config.fish"
fi

# Ensure ~/.local/bin is in path
if ! fish -c "contains $HOME/.local/bin \$fish_user_paths" 2>/dev/null; then
    fish -c "fish_add_path $HOME/.local/bin"
    echo "✅ Added ~/.local/bin to Fish path"
fi

echo "✨ Installation complete! Restart your fish shell or run 'source ~/.config/fish/config.fish'"
echo ""
echo "🔥 NLSH-Pro is now active as a SHELL INTERCEPTOR."
echo "   1. Type a wrong command -> AI fixes it automatically."
echo "   2. Press Alt+L -> AI suggests a command."
echo "   3. Type '!task' -> Forces AI mode."
