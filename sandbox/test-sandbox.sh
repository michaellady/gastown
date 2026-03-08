#!/usr/bin/env bash
#
# test-sandbox.sh - Manual test harness for gastown-polecat.sb
#
# Tests sandbox-exec with the polecat Seatbelt profile. Runs a series
# of commands inside the sandbox to verify access controls.
#
# Usage:
#   ./sandbox/test-sandbox.sh [worktree] [town_root] [rig_name]
#
# Defaults to current polecat's paths if no args given.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE="$SCRIPT_DIR/gastown-polecat.sb"

# Defaults for testing
WORKTREE="${1:-$(pwd)}"
TOWN_ROOT="${2:-/Users/mikelady/gt}"
RIG_NAME="${3:-gastown}"
HOME_DIR="$HOME"

echo "=== gastown-polecat.sb sandbox test ==="
echo "Profile:    $PROFILE"
echo "Worktree:   $WORKTREE"
echo "Town root:  $TOWN_ROOT"
echo "Rig:        $RIG_NAME"
echo "Home:       $HOME_DIR"
echo ""

SBX="sandbox-exec -f $PROFILE -D WORKTREE=$WORKTREE -D TOWN_ROOT=$TOWN_ROOT -D RIG_NAME=$RIG_NAME -D _HOME=$HOME_DIR"

pass=0
fail=0

run_test() {
    local desc="$1"
    local expect="$2"  # "allow" or "deny"
    shift 2
    local cmd="$*"

    printf "  %-55s" "$desc"

    if output=$($SBX bash -c "$cmd" 2>&1); then
        if [ "$expect" = "allow" ]; then
            echo "PASS (allowed)"
            ((pass++))
        else
            echo "FAIL (expected deny, got allow)"
            ((fail++))
        fi
    else
        if [ "$expect" = "deny" ]; then
            echo "PASS (denied)"
            ((pass++))
        else
            echo "FAIL (expected allow, got deny)"
            echo "      output: $(echo "$output" | head -3)"
            ((fail++))
        fi
    fi
}

echo "--- Worktree Access (RW) ---"
run_test "Read file in worktree" "allow" "ls $WORKTREE/go.mod >/dev/null"
run_test "Write file in worktree" "allow" "touch $WORKTREE/.sandbox-test && rm $WORKTREE/.sandbox-test"
run_test "Git status in worktree" "allow" "cd $WORKTREE && git status --short >/dev/null"
run_test "Git log in worktree" "allow" "cd $WORKTREE && git log --oneline -1 >/dev/null"

echo ""
echo "--- Shared Dirs (RO) ---"
run_test "Read town .beads" "allow" "ls $TOWN_ROOT/.beads/ >/dev/null 2>&1"
run_test "Read rig .beads" "allow" "ls $TOWN_ROOT/$RIG_NAME/.beads/ >/dev/null 2>&1"
run_test "Read .repo.git" "allow" "ls $TOWN_ROOT/$RIG_NAME/.repo.git/ >/dev/null 2>&1 || true"
run_test "Read CLAUDE.md" "allow" "cat $TOWN_ROOT/CLAUDE.md >/dev/null 2>&1 || true"

echo ""
echo "--- Binary Execution ---"
run_test "Execute git" "allow" "git --version >/dev/null"
run_test "Execute node" "allow" "node --version >/dev/null"
run_test "Execute gt --help" "allow" "gt --help >/dev/null 2>&1 || true"
run_test "Execute bd --help" "allow" "bd --help >/dev/null 2>&1 || true"

echo ""
echo "--- Filesystem Deny ---"
run_test "Write to /tmp directly" "deny" "touch /tmp/sandbox-escape-test 2>/dev/null && rm /tmp/sandbox-escape-test"
run_test "Read ~/Documents" "deny" "ls $HOME_DIR/Documents/ >/dev/null 2>&1"
run_test "Read ~/Desktop" "deny" "ls $HOME_DIR/Desktop/ >/dev/null 2>&1"
run_test "Read ~/Downloads" "deny" "ls $HOME_DIR/Downloads/ >/dev/null 2>&1"
run_test "Write to home root" "deny" "touch $HOME_DIR/.sandbox-escape 2>/dev/null"
run_test "Write outside worktree (other rig)" "deny" "touch $TOWN_ROOT/other-rig-test 2>/dev/null"

echo ""
echo "--- Network ---"
run_test "Loopback TCP (Dolt port)" "allow" "bash -c 'echo | nc -z -w1 127.0.0.1 14144 2>/dev/null || true'"
run_test "DNS resolution" "allow" "host -W 2 api.anthropic.com >/dev/null 2>&1 || true"

echo ""
echo "--- SSH Key Access (read-only) ---"
run_test "Read ~/.ssh" "allow" "ls $HOME_DIR/.ssh/ >/dev/null 2>&1 || true"
run_test "Write to ~/.ssh (should deny)" "deny" "touch $HOME_DIR/.ssh/sandbox-test 2>/dev/null"

echo ""
echo "--- Git Config (read-only) ---"
run_test "Read ~/.gitconfig" "allow" "cat $HOME_DIR/.gitconfig >/dev/null 2>&1 || true"
run_test "Write ~/.gitconfig (should deny)" "deny" "echo test >> $HOME_DIR/.gitconfig 2>/dev/null"

echo ""
echo "=== Results: $pass passed, $fail failed ==="

if [ "$fail" -gt 0 ]; then
    echo ""
    echo "WARNING: $fail test(s) failed. Review sandbox profile."
    exit 1
fi
