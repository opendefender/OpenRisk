#!/usr/bin/env bash
# Installs the agent runner (#1252) on the owner's machine, as a systemd *user* timer.
# Run once from the main checkout: bash ops/runner/install.sh
#   - creates the runner's own checkout ../OpenRisk-runner (detached on origin/develop),
#     so a batch never touches the owner's working copy;
#   - installs openrisk-runner.service and .timer into ~/.config/systemd/user;
#   - enables the timer. `loginctl enable-linger $USER` keeps it running when logged out.
# Uninstall: systemctl --user disable --now openrisk-runner.timer
set -euo pipefail
MAIN="$(git rev-parse --show-toplevel)"
RUN="$(dirname "$MAIN")/OpenRisk-runner"
UNIT_DIR="$HOME/.config/systemd/user"

git -C "$MAIN" fetch --quiet origin develop
if [ ! -d "$RUN" ]; then git -C "$MAIN" worktree add --quiet --detach "$RUN" origin/develop; fi
mkdir -p "$UNIT_DIR" "$HOME/.local/state/openrisk-runner" "$HOME/.config/openrisk-runner"

# The units need node, git, gh and the agent CLI. The CLI usually lives in ~/.local/bin,
# which a non-interactive shell (`! …` in a session, cron, ssh) often leaves off PATH.
export PATH="$HOME/.local/bin:$HOME/bin:$PATH"
missing=""
for t in node git gh claude; do command -v "$t" >/dev/null || missing="$missing $t"; done
if [ -n "$missing" ]; then
  echo "install.sh: cannot install the runner, missing on PATH:$missing" >&2
  echo "install.sh: install them, or add their directory to PATH, then run this again." >&2
  exit 1
fi
PATHS="$(for t in node git gh claude; do dirname "$(command -v "$t")"; done | sort -u | paste -sd:)"

sed -e "s|@RUN@|$RUN|g" -e "s|@PATH@|$PATHS:/usr/bin:/bin|g" "$MAIN/ops/runner/openrisk-runner.service" > "$UNIT_DIR/openrisk-runner.service"
cp "$MAIN/ops/runner/openrisk-runner.timer" "$UNIT_DIR/openrisk-runner.timer"
systemctl --user daemon-reload
systemctl --user enable --now openrisk-runner.timer
echo "installed. next runs:"; systemctl --user list-timers openrisk-runner.timer --no-pager
echo "dry run now:  (cd $RUN && node scripts/runner/runner.mjs --dry-run)"
echo "kill switch:  touch ~/.config/openrisk-runner/disabled"
