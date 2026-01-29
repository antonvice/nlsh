function __nlsh_pro_invoke
    set -l current_cmd (commandline)
    if test -z "$current_cmd"
        set_color -o purple
        echo " 🌌 NLSH-PRO | NEURAL COMMAND LINK v1.0"
        echo " 💡 Hint: Type intent in plain English or a partial command."
        set_color normal
        commandline -f repaint
        return
    end

    # Cool orbital scanning animation
    set -l frames '▖' '▘' '▝' '▗'
    set -l color_frames 5f00ff 8700ff af00ff d700ff
    
    # We'll run the scanning message
    set_color -o cyan
    echo -n " 🛰️  [Orbital Link Active] "
    set_color normal
    echo -n "Scanning intent... "
    
    # Start the Go process in background or just run it
    # For a "cool" effect we can't easily do a real spinner while blocked, 
    # but we can simulate a very fast one or just a premium static line.
    
    # Actually, let's do a premium static status and then the result.
    set -l suggested_cmd (~/.local/bin/nlsh-pro "$current_cmd")
    
    # Clear the scanning line
    echo -ne "\r\033[K"

    if test -n "$suggested_cmd"; and not string match -q "API Error*" "$suggested_cmd"
        # Premium suggestion box with gradients (simulated with fish colors)
        set_color -o 00ffaf
        echo " ╭────────────────────────────────────────────────────────────────────────────"
        echo -n " │ "
        set_color -o yellow
        echo "Proposed Command::"
        
        # Highlight the command line
        set_color normal
        echo " │   " (set_color -o white)"$suggested_cmd"
        
        set_color -o 00ffaf
        echo " ╰────────────────────────────────────────────────────────────────────────────"
        
        set_color normal
        set_color -o 5fafff
        echo -n "  [⏎] Execute "
        set_color 87d7ff
        echo -n " [L] Refine "
        set_color afafff
        echo -n " [Any] Abort"
        set_color normal
        
        # Read a single character
        read -n 1 -l action
        
        if test -z "$action" # Enter
            commandline -r "$suggested_cmd"
            commandline -f execute
        else if test "$action" = "l" -o "$action" = "L"
            commandline -r "$suggested_cmd"
            commandline -f repaint
        else
            commandline -f repaint
        end
    else
        set_color -o red
        echo " ❌ Neural link failed: $suggested_cmd"
        set_color normal
        commandline -f repaint
    end
end

# Key Bindings
# Bound to Alt+L (Option+L on macOS)
bind \el __nlsh_pro_invoke
# Bound to Alt+Enter (Option+Return on macOS)
bind \e\r __nlsh_pro_invoke

# Enhanced Command Not Found Handler
function fish_command_not_found
    set -l cmd $argv[1]
    
    # Ignore short command typos (less than 3 chars)
    if test (string length "$cmd") -lt 3
        __fish_default_command_not_found_handler $argv
        return
    end

    set_color -o yellow
    echo " ⚡ Command '$cmd' not found. Routing to NLSH-Pro..."
    set_color normal

    set -l suggested_cmd (~/.local/bin/nlsh-pro "$argv")
    if test -n "$suggested_cmd"; and not string match -q "API Error*" "$suggested_cmd"
        set_color -o green
        echo " ✨ AI suggests: $suggested_cmd"
        set_color normal
        echo " Press [Enter] to execute, or any other key to abort..."
        
        read -l -n 1 confirm
        if test -z "$confirm"
            eval "$suggested_cmd"
        end
    else
        __fish_default_command_not_found_handler $argv
    end
end
