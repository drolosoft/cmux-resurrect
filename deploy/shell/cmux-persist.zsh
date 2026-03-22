# cmux-persist shell integration
# Add to your .zshrc: source /path/to/cmux-persist.zsh

# Aliases
alias cps='cmux-persist save'
alias cpr='cmux-persist restore'
alias cpl='cmux-persist list'
alias cpw='cmux-persist watch'

# Auto-save on terminal exit (add to .zshrc)
cmux-persist-autosave() {
    if command -v cmux-persist &>/dev/null && [[ -S /tmp/cmux.sock ]]; then
        cmux-persist save autosave --description "auto-exit-save" 2>/dev/null
    fi
}

# Completion
if (( $+commands[cmux-persist] )); then
    eval "$(cmux-persist completion zsh 2>/dev/null || true)"
fi
