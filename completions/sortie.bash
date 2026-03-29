_sortie() {
    local cur prev words cword
    _init_completion || return

    local commands="scan watch history undo rules config status trash validate man help completion"
    local global_flags="--help --version --config --verbose -v"

    if [[ $cword -eq 1 ]]; then
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "$global_flags" -- "$cur"))
        else
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        fi
        return
    fi

    case "${words[1]}" in
    scan)
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "--dry-run --help" -- "$cur"))
        else
            COMPREPLY=($(compgen -d -- "$cur"))
        fi
        ;;
    watch)
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "--dry-run --debounce --help" -- "$cur"))
        fi
        ;;
    history)
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "--limit -n --help" -- "$cur"))
        fi
        ;;
    undo)
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "--last --help" -- "$cur"))
        fi
        ;;
    rules)
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "--global --help" -- "$cur"))
        elif [[ $cword -eq 2 ]]; then
            COMPREPLY=($(compgen -W "test" -- "$cur") $(compgen -d -- "$cur"))
        elif [[ "${words[2]}" == "test" ]]; then
            COMPREPLY=($(compgen -f -- "$cur"))
        else
            COMPREPLY=($(compgen -d -- "$cur"))
        fi
        ;;
    config)
        if [[ $cword -eq 2 ]]; then
            COMPREPLY=($(compgen -W "init path" -- "$cur"))
        fi
        ;;
    trash)
        if [[ $cword -eq 2 ]]; then
            COMPREPLY=($(compgen -W "purge" -- "$cur"))
        fi
        ;;
    validate)
        if [[ "$cur" == -* ]]; then
            COMPREPLY=($(compgen -W "--global --help" -- "$cur"))
        else
            COMPREPLY=($(compgen -d -- "$cur"))
        fi
        ;;
    completion)
        if [[ $cword -eq 2 ]]; then
            COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "$cur"))
        fi
        ;;
    esac
}

complete -F _sortie sortie
