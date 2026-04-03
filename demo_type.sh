#!/usr/bin/env bash
# yoink --type / --delay / --no-enter demo
# A real Claude session managed entirely through yoink.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
export PATH="$SCRIPT_DIR:$PATH"
export YOINK_SOCK="/tmp/yoink-demo-$$.sock"

# ── colours ───────────────────────────────────────────────────────────────────
DIM=$'\033[2m'
RST=$'\033[0m'
GRN=$'\033[38;5;84m'

# ── helpers ───────────────────────────────────────────────────────────────────

# Type a command char by char, then execute it
run() {
    printf "${GRN}❯${RST} "
    for ((i=0; i<${#1}; i++)); do
        printf '%s' "${1:$i:1}"
        sleep 0.062
    done
    printf '\n'
    sleep 0.12
    eval "$1"
}

comment() { printf "\n${DIM}# %s${RST}\n" "$1"; sleep 0.5; }

# Drain --new cursor without displaying (swallows input echo and startup noise)
drain() { yoink log "$1" --new > /dev/null 2>&1 || true; }

# ── scene ─────────────────────────────────────────────────────────────────────
clear
sleep 0.4

# 1. spawn the Claude session
comment "start a Claude session in the background"
run "yoink run --alias ai \"./claude-chat.sh\""
sleep 2.5
run "yoink log ai"
drain ai
sleep 0.8

# 2. first question — typing simulation
comment "send a prompt — every character typed individually"
run "yoink send ai \"name 3 essential unix tools, comma separated\" --type --delay 60"
sleep 0.5; drain ai          # drain echo entries before response arrives
sleep 8                      # wait for claude --print response (~5s + buffer)
run "yoink log ai --new"
sleep 1.0

# 3. ask what tools Claude has
comment "ask what tools Claude has available"
run "yoink send ai \"/plugins\" --type --delay 60"
sleep 0.5; drain ai
sleep 8
run "yoink log ai --new"
sleep 1.0

# 4. --no-enter: compose across two sends
comment "--no-enter: hold the line open, then complete it"
run "yoink send ai \"what about Go —\" --type --delay 65 --no-enter"
sleep 0.3
run "yoink send ai \" top 2 tools, one line\" --type --delay 65"
sleep 0.5; drain ai
sleep 8
run "yoink log ai --new"
sleep 1.0

# 5. exit
comment "done — exit the session"
run "yoink send ai \"/exit\" --type --delay 70"
sleep 1.5
run "yoink clean"
sleep 0.4
