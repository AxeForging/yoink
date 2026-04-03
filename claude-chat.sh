#!/usr/bin/env bash
# Plain-text Claude session — no TUI, works cleanly with yoink log
set -euo pipefail

PNK=$'\033[38;5;212m'
GRN=$'\033[38;5;84m'
DIM=$'\033[2m'
RST=$'\033[0m'

printf '%s\n' "${PNK}Claude Chat${RST} ${DIM}· plain-text session via yoink${RST}"
printf '%s\n' "${DIM}────────────────────────────────${RST}"
printf '\n'

while IFS= read -r prompt; do
    [[ -z "$prompt" ]] && continue
    if [[ "$prompt" == "/exit" || "$prompt" == "exit" ]]; then
        printf '%s\n' "${DIM}Session ended.${RST}"
        exit 0
    fi
    if [[ "$prompt" == "/plugins" || "$prompt" == "/tools" ]]; then
        printf '%s\n' "${PNK}available tools (from claude --print):${RST}"
        claude --print "list the tools and capabilities you have available in one short paragraph" 2>/dev/null
        printf '\n'
        continue
    fi
    printf '%s ' "${GRN}claude>${RST}"
    claude --print "$prompt" 2>/dev/null
    printf '\n'
done
