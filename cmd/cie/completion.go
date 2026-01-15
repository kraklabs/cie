// Copyright 2025 KrakLabs
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
//
// For commercial licensing, contact: licensing@kraklabs.com
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kraklabs/cie/internal/errors"
)

// bashCompletionTemplate is the bash completion script for CIE.
//
// It provides command and flag completion for bash shells using the
// bash completion framework.
const bashCompletionTemplate = `#!/bin/bash

# Bash completion script for CIE (Code Intelligence Engine)
# Installation:
#   source <(cie completion bash)
#   Or add to ~/.bashrc:
#   echo 'source <(cie completion bash)' >> ~/.bashrc

_cie_completion() {
    local cur prev commands
    commands="init index status query reset install-hook completion"

    # Current word being completed
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Global flags
    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "--version --mcp --config" -- ${cur}) )
        return 0
    fi

    # First argument: complete commands
    if [ $COMP_CWORD -eq 1 ]; then
        COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
        return 0
    fi

    # Command-specific flag completion
    local cmd="${COMP_WORDS[1]}"
    case "${cmd}" in
        index)
            if [[ ${cur} == -* ]] ; then
                COMPREPLY=( $(compgen -W "--full --force-full-reindex --embed-workers --debug --metrics-addr" -- ${cur}) )
            fi
            ;;
        status)
            if [[ ${cur} == -* ]] ; then
                COMPREPLY=( $(compgen -W "--json" -- ${cur}) )
            fi
            ;;
        query)
            if [[ ${cur} == -* ]] ; then
                COMPREPLY=( $(compgen -W "--json" -- ${cur}) )
            fi
            ;;
        reset)
            if [[ ${cur} == -* ]] ; then
                COMPREPLY=( $(compgen -W "--yes" -- ${cur}) )
            fi
            ;;
        install-hook)
            if [[ ${cur} == -* ]] ; then
                COMPREPLY=( $(compgen -W "--force --remove" -- ${cur}) )
            fi
            ;;
        completion)
            # Complete shell names for completion command
            if [ $COMP_CWORD -eq 2 ]; then
                COMPREPLY=( $(compgen -W "bash zsh fish" -- ${cur}) )
            fi
            ;;
    esac
}

complete -F _cie_completion cie
`

// zshCompletionTemplate is the zsh completion script for CIE.
//
// It provides command and flag completion for zsh shells using the
// zsh completion system.
const zshCompletionTemplate = `#compdef cie

# Zsh completion script for CIE (Code Intelligence Engine)
# Installation:
#   1. Ensure compinit is loaded (add to ~/.zshrc if not present):
#      autoload -U compinit; compinit
#   2. Save this script to a directory in your fpath:
#      cie completion zsh > "${fpath[1]}/_cie"
#   3. Reload completions:
#      rm -f ~/.zcompdump; compinit

_cie() {
    local -a commands
    commands=(
        'init:Create .cie/project.yaml configuration'
        'index:Index the current repository'
        'status:Show project status'
        'query:Execute CozoScript query'
        'reset:Reset local project data'
        'install-hook:Install git post-commit hook'
        'completion:Generate shell completion script'
    )

    _arguments -C \
        '(- *)--version[Show version and exit]' \
        '--mcp[Start as MCP server (JSON-RPC over stdio)]' \
        '--config[Path to .cie/project.yaml]:config file:_files -g "*.yaml"' \
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                index)
                    _arguments \
                        '--full[Force full re-index (ignore incremental)]' \
                        '--force-full-reindex[Force full re-index (ignore incremental)]' \
                        '--embed-workers[Number of embedding workers]:workers:' \
                        '--debug[Enable debug logging]' \
                        '--metrics-addr[Prometheus metrics address]:address:'
                    ;;
                status)
                    _arguments \
                        '--json[Output as JSON]'
                    ;;
                query)
                    _arguments \
                        '--json[Output as JSON]' \
                        '1:cozoscript query:'
                    ;;
                reset)
                    _arguments \
                        '--yes[Skip confirmation prompt]'
                    ;;
                install-hook)
                    _arguments \
                        '--force[Overwrite existing hook]' \
                        '--remove[Remove the hook]'
                    ;;
                completion)
                    _arguments \
                        '1:shell:(bash zsh fish)'
                    ;;
            esac
            ;;
    esac
}

_cie
`

// fishCompletionTemplate is the fish completion script for CIE.
//
// It provides command and flag completion for fish shells using the
// fish completion system.
const fishCompletionTemplate = `# Fish completion script for CIE (Code Intelligence Engine)
# Installation:
#   1. Load completions for current session:
#      cie completion fish | source
#   2. Install permanently:
#      cie completion fish > ~/.config/fish/completions/cie.fish

# Commands
complete -c cie -f -n "__fish_use_subcommand" -a "init" -d "Create .cie/project.yaml configuration"
complete -c cie -f -n "__fish_use_subcommand" -a "index" -d "Index the current repository"
complete -c cie -f -n "__fish_use_subcommand" -a "status" -d "Show project status"
complete -c cie -f -n "__fish_use_subcommand" -a "query" -d "Execute CozoScript query"
complete -c cie -f -n "__fish_use_subcommand" -a "reset" -d "Reset local project data (destructive!)"
complete -c cie -f -n "__fish_use_subcommand" -a "install-hook" -d "Install git post-commit hook"
complete -c cie -f -n "__fish_use_subcommand" -a "completion" -d "Generate shell completion script"

# Global flags
complete -c cie -l version -d "Show version and exit"
complete -c cie -l mcp -d "Start as MCP server (JSON-RPC over stdio)"
complete -c cie -l config -d "Path to .cie/project.yaml" -r

# index command flags
complete -c cie -n "__fish_seen_subcommand_from index" -l full -d "Force full re-index (ignore incremental)"
complete -c cie -n "__fish_seen_subcommand_from index" -l force-full-reindex -d "Force full re-index (ignore incremental)"
complete -c cie -n "__fish_seen_subcommand_from index" -l embed-workers -d "Number of embedding workers" -r
complete -c cie -n "__fish_seen_subcommand_from index" -l debug -d "Enable debug logging"
complete -c cie -n "__fish_seen_subcommand_from index" -l metrics-addr -d "Prometheus metrics address" -r

# status command flags
complete -c cie -n "__fish_seen_subcommand_from status" -l json -d "Output as JSON"

# query command flags
complete -c cie -n "__fish_seen_subcommand_from query" -l json -d "Output as JSON"

# reset command flags
complete -c cie -n "__fish_seen_subcommand_from reset" -l yes -d "Skip confirmation prompt"

# install-hook command flags
complete -c cie -n "__fish_seen_subcommand_from install-hook" -l force -d "Overwrite existing hook"
complete -c cie -n "__fish_seen_subcommand_from install-hook" -l remove -d "Remove the hook"

# completion command arguments
complete -c cie -n "__fish_seen_subcommand_from completion" -f -a "bash" -d "Generate bash completion script"
complete -c cie -n "__fish_seen_subcommand_from completion" -f -a "zsh" -d "Generate zsh completion script"
complete -c cie -n "__fish_seen_subcommand_from completion" -f -a "fish" -d "Generate fish completion script"
`

// runCompletion executes the 'completion' CLI command, generating shell-specific
// completion scripts for bash, zsh, or fish shells.
//
// The completion command outputs a shell-specific script to stdout that can be
// sourced to enable tab completion for CIE commands and flags. Each shell has
// different completion syntax and installation requirements.
//
// Usage:
//
//	cie completion [bash|zsh|fish]
//
// Examples:
//
//	cie completion bash                     Output bash completion script
//	source <(cie completion bash)           Load bash completions in current shell
//	cie completion zsh > "${fpath[1]}/_cie" Install zsh completions permanently
//	cie completion fish | source            Load fish completions in current shell
//
// Installation instructions are provided in the help text for each shell.
func runCompletion(args []string, configPath string) {
	fs := flag.NewFlagSet("completion", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie completion <shell>

Description:
  Generate shell completion scripts for bash, zsh, or fish.

  Shell completions allow you to use Tab to autocomplete commands,
  flags, and arguments. This improves discoverability and reduces typing.

Arguments:
  shell    Shell type: bash, zsh, or fish (required)

Examples:
  # Generate bash completion script
  cie completion bash

  # Load bash completions in current shell
  source <(cie completion bash)

  # Install bash completions permanently (Linux)
  cie completion bash > /etc/bash_completion.d/cie

  # Install zsh completions (macOS with Homebrew)
  cie completion zsh > $(brew --prefix)/share/zsh/site-functions/_cie

  # Install fish completions
  cie completion fish > ~/.config/fish/completions/cie.fish

Installation Instructions:

Bash:
  # Load completions in current shell
  source <(cie completion bash)

  # Load completions for each session (add to ~/.bashrc)
  echo 'source <(cie completion bash)' >> ~/.bashrc

Zsh:
  # Enable completion if not already enabled (add to ~/.zshrc)
  echo "autoload -U compinit; compinit" >> ~/.zshrc

  # Install completions permanently
  cie completion zsh > "${fpath[1]}/_cie"

Fish:
  # Load completions in current shell
  cie completion fish | source

  # Install completions permanently
  cie completion fish > ~/.config/fish/completions/cie.fish

Notes:
  After installing completions, restart your shell or source your rc file.
  For persistent installation, add the source command to ~/.bashrc or ~/.zshrc.

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate arguments
	if fs.NArg() != 1 {
		errors.FatalError(errors.NewInputError(
			"Invalid arguments",
			"The completion command requires exactly one argument: the shell name",
			"Run 'cie completion bash', 'cie completion zsh', or 'cie completion fish'",
		), false)
	}

	shell := fs.Arg(0)

	// Generate completion script for the specified shell
	switch shell {
	case "bash":
		fmt.Print(bashCompletionTemplate)
	case "zsh":
		fmt.Print(zshCompletionTemplate)
	case "fish":
		fmt.Print(fishCompletionTemplate)
	default:
		errors.FatalError(errors.NewInputError(
			"Unsupported shell",
			fmt.Sprintf("Shell '%s' is not supported. Valid options: bash, zsh, fish", shell),
			"Run 'cie completion bash', 'cie completion zsh', or 'cie completion fish'",
		), false)
	}
}
