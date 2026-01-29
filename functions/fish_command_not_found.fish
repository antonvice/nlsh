function fish_command_not_found --on-event fish_command_not_found
    set -l cmd $argv[1]
    set -l is_force_ai 0
    
    if string match -q '!*' "$cmd"
        set is_force_ai 1
    end

    # Ignore short command typos (less than 3 chars) UNLESS it's a forced AI call
    if test $is_force_ai -eq 0; and test (string length "$cmd") -lt 3
        __fish_default_command_not_found_handler $argv
        return
    end

    # Cool orbital scanning animation
    echo -ne " 🛰️  [Orbital Link Active] Scanning..."
    
    # Quick flash animation (simulated)
    # We don't want to block too long, just a split second
    sleep 0.1
    echo -ne "\r\033[K"

    if test $is_force_ai -eq 1
        set_color -o purple
        echo " 🌌 NLSH-Pro | Force-AI Mode Engaged"
        set_color normal
    else
        set_color -o yellow
        echo " ⚡ Command '$cmd' not found. Routing to NLSH-Pro..."
        set_color normal
    end

    set -l suggested_cmd (~/.local/bin/nlsh-pro "$argv")
    if test -n "$suggested_cmd"; and not string match -q "API Error*" "$suggested_cmd"
        # Premium suggestion box
        set_color -o 00ffaf
        echo " ╭────────────────────────────────────────────────────────────────────────────"
        echo -n " │ "
        set_color -o yellow
        echo "Proposed Command:"
        set_color normal
        echo " │   " (set_color -o white)"$suggested_cmd"
        set_color -o 00ffaf
        echo " ╰────────────────────────────────────────────────────────────────────────────"
        set_color normal
        
        echo " Press [Enter] to execute, or any other key to abort..."
        
        # Read with a hidden prompt to avoid "read>" clutter
        read -l -n 1 -P "" confirm
        if test -z "$confirm"
            eval "$suggested_cmd"
        else
            __fish_default_command_not_found_handler $argv
        end
    else
        __fish_default_command_not_found_handler $argv
    end
end
