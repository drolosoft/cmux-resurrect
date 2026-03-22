# cmux-resurrect shell integration
# Add to your .zshrc: source /path/to/cmux-resurrect.zsh

# Aliases
alias crs='cmres save'
alias crr='cmres restore'
alias crl='cmres list'
alias crw='cmres watch'

# Auto-save on terminal exit (add to .zshrc)
cmres-autosave() {
    if command -v cmres &>/dev/null && [[ -S /tmp/cmux.sock ]]; then
        cmres save autosave --description "auto-exit-save" 2>/dev/null
    fi
}

# Completion (available when cmres adds cobra completion support)
# if (( $+commands[cmres] )); then
#     eval "$(cmres completion zsh 2>/dev/null || true)"
# fi
