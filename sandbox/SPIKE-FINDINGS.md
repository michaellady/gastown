# Spike: macOS sandbox-exec Polecat Isolation

**Bead:** gt-2pb
**Date:** 2026-03-08
**Status:** Complete

## Summary

macOS `sandbox-exec` (Seatbelt) is viable as the exitbox runtime for polecat
isolation on Mac dev laptops. A working `.sb` profile has been created and
tested that restricts a polecat to its worktree with controlled access to
shared Gas Town directories.

## What Works

All core polecat operations succeed inside the sandbox:

| Operation | Status | Notes |
|-----------|--------|-------|
| `claude --version` | OK | Claude CLI launches and runs |
| `gt prime --hook` | OK | Full context loading works |
| `gt done --help` | OK | Command accessible (not tested end-to-end to avoid side effects) |
| `gt --version` | OK | |
| `bd --help` | OK | |
| `git status/log` | OK | After adding `.git` worktree dir and parent dir metadata access |
| `git remote -v` | OK | |
| `git push --dry-run` | OK | SSH key read access works |
| `node --version` | OK | |
| Worktree read/write | OK | |
| Rig `.beads` read | OK | |
| `.gitconfig` read | OK | Read-only |
| `.ssh` read | OK | Read-only (write denied) |

## What Breaks (and Fixes Applied)

### 1. `(import "system.sb")` is required
**Problem:** Without importing Apple's base profile, Seatbelt fails with
SIGABRT (exit 134) even on syntactically valid rules. The base profile
provides foundational type/filter definitions that sandbox-exec expects.

**Fix:** Add `(import "system.sb")` after `(version 1)(deny default)`.

### 2. Git requires `/private/var/select` access
**Problem:** macOS wraps git through Xcode command-line tools. The wrapper
reads `/var/select/developer_dir` (symlink to `/private/var/select/developer_dir`)
to find the Xcode installation. Without this, git fails with "No developer
tools were found."

**Fix:** Add `(subpath "/private/var/select")` to file-read rules.

### 3. Git worktrees need parent `.git` directory access
**Problem:** Polecat worktrees use `gitdir:` references pointing to the main
rig's `.git/worktrees/<name>` directory. Git can't resolve the worktree
without reading the main `.git` directory.

**Fix:** Add `(subpath (string-append (param "TOWN_ROOT") "/" (param "RIG_NAME") "/.git"))`
to read rules.

### 4. Git needs parent directory metadata traversal
**Problem:** Git walks up the directory tree from CWD looking for `.git`.
Without read-metadata access to parent directories (`/Users`, `/Users/mikelady`,
etc.), git fails with "Invalid path '/Users': Operation not permitted".

**Fix:** Add `(allow file-read-metadata (literal "/") (literal "/Users") (subpath (param "_HOME")))`.

### 5. `gt prime` needs mayor registry access
**Problem:** `gt prime` reads `/Users/mikelady/gt/mayor/rigs.json` for the
rig registry. Without access, it emits a warning (non-fatal but noisy).

**Fix:** Add `(subpath (string-append (param "TOWN_ROOT") "/mayor"))` to read rules.

### 6. Parameter name `HOME` conflicts
**Problem:** Seatbelt's `(param "HOME")` may conflict with shell environment
variable expansion or reserved names.

**Fix:** Use `_HOME` as the parameter name (underscore prefix).

## Security Properties Verified

| Property | Verified |
|----------|----------|
| Cannot read ~/Documents | Yes |
| Cannot read ~/Desktop | Yes |
| Cannot read ~/Downloads | Yes |
| Cannot write to ~/.ssh | Yes |
| Cannot write to ~/.gitconfig | Yes |
| Cannot write outside worktree | Yes |
| Cannot write to town root | Yes |
| Cannot create dirs in other polecats | Yes |
| Loopback network (Dolt) accessible | Yes |
| HTTPS outbound (Anthropic API) allowed | Yes |

## Network Policy

The current profile allows:
- **Loopback:** `localhost:*` (Dolt SQL server on any port)
- **Outbound TCP 443:** Required for Claude CLI to reach `api.anthropic.com`
- **DNS (UDP/TCP 53):** Required for hostname resolution
- **Unix sockets:** Required for tmux communication and ssh-agent

**Note:** Outbound TCP 443 is broader than ideal. A tighter policy would
restrict to specific Anthropic IP ranges, but DNS-based filtering isn't
supported by Seatbelt. This is acceptable for the local sandbox use case
since the goal is preventing accidental data leaks, not defending against
a fully compromised agent (which would need the daytona container model).

## Known Limitations

1. **`sandbox-exec` is deprecated.** Apple marks it as deprecated and
   recommends App Sandbox instead. However, it still works on macOS 15+
   (Sequoia) and there's no sign of removal. Many system services still
   use `.sb` profiles.

2. **No per-host network filtering.** Seatbelt can filter by port and
   protocol but not by hostname. Restricting to "loopback only" would
   break Claude CLI (needs Anthropic API). The `*:443` rule is a pragmatic
   compromise.

3. **`/etc/passwd` is readable.** System files under `/private/etc` are
   allowed for process execution. This is standard for sandboxed processes
   and not a security concern.

4. **Read-metadata on home directory.** Git's parent-directory traversal
   requires `file-read-metadata` on the home directory tree. This leaks
   directory names (not contents) which is acceptable.

## Profile Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `WORKTREE` | Absolute path to polecat worktree (RW) | `/Users/me/gt/gastown/polecats/slit/gastown` |
| `TOWN_ROOT` | Absolute path to town root (RO shared) | `/Users/me/gt` |
| `RIG_NAME` | Rig name | `gastown` |
| `_HOME` | User home directory | `/Users/me` |

## Usage

```bash
sandbox-exec -f sandbox/gastown-polecat.sb \
  -D WORKTREE=/path/to/worktree \
  -D TOWN_ROOT=/path/to/gt \
  -D RIG_NAME=gastown \
  -D _HOME=$HOME \
  claude --mode=direct
```

## Next Steps

1. **gt-b18:** Wire `ExecWrapper` plumbing into polecat startup so
   `gt sling` can use `sandbox-exec` automatically
2. **End-to-end test:** Run a full polecat session (spawn, gt prime,
   implement, gt done) inside the sandbox via tmux
3. **CI integration:** Add sandbox profile validation to CI
