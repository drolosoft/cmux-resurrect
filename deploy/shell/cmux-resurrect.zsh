# cmux-resurrect shell integration
# Add to your .zshrc: source /path/to/cmux-resurrect.zsh

# Aliases
alias crs='crex save'
alias crr='crex restore'
alias crl='crex list'
alias crw='crex watch'

# Auto-save on terminal exit (add to .zshrc)
crex-autosave() {
    if command -v crex &>/dev/null && [[ -S /tmp/cmux.sock ]]; then
        crex save autosave --description "auto-exit-save" 2>/dev/null
    fi
}

# Completion (available when crex adds cobra completion support)
# if (( $+commands[crex] )); then
#     eval "$(crex completion zsh 2>/dev/null || true)"
# fi
