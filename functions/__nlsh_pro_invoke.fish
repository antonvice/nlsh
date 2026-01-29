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
    set_color -o cyan
    echo -n " 🛰️  [Orbital Link Active] "
    set_color normal
    echo -n "Scanning intent... "
    
    # Call the Go binary
    set -l suggested_cmd (~/.local/bin/nlsh-pro "$current_cmd")
    
    # Clear the scanning line
    echo -ne "\r\033[K"

    if test -n "$suggested_cmd"; and not string match -q "API Error*" "$suggested_cmd"
        # Premium suggestion box
        set_color -o 00ffaf
        echo " ╭────────────────────────────────────────────────────────────────────────────"
        echo -n " │ "
        set_color -o yellow
        echo "SYNAPTIC PROPOSAL:"
        
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
