#!/usr/bin/env bash
# Checks that this build can be picked up by a fresh session on another model.
#
# The next session reads CLAUDE.md and STATE.md and nothing else — no
# conversation, no memory of how any of this was built. That is deliberate: a
# new session on a new model costs almost nothing to start, while switching
# models inside one re-sends the whole conversation. The price of that is that
# anything not written down is gone, so this checks it was written down.
#
# Run it at every model handover, and at the end of any session that might be
# the last one. It changes nothing; it only refuses.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

BRANCH="claude/goklay-design-system-9z500x"
problems=()
note() { problems+=("$1"); }

# 1. Nothing uncommitted. An uncommitted file is a file the next session will
#    never see.
if [[ -n "$(git status --porcelain)" ]]; then
  note "there are uncommitted changes — commit them, they do not survive this session"
fi

# 2. On the right branch, and pushed. A commit that exists only here is a commit
#    that exists nowhere: this container is reclaimed when the session ends.
current="$(git rev-parse --abbrev-ref HEAD)"
if [[ "${current}" != "${BRANCH}" ]]; then
  note "on branch ${current}, expected ${BRANCH}"
elif ! git diff --quiet "${BRANCH}" "origin/${BRANCH}" 2>/dev/null; then
  note "the branch is ahead of origin — push it"
fi

# 3. STATE.md says where to resume, and says something that is not last week.
for field in current_phase current_task current_branch; do
  grep -q "^${field}:" STATE.md || note "STATE.md has no ${field}"
done
phase="$(sed -n 's/^current_phase: *//p' STATE.md | head -1)"
task="$(sed -n 's/^current_task: *//p' STATE.md | head -1)"
[[ -n "${phase}" && -n "${task}" ]] || note "STATE.md does not say what to do next"
grep -q "^## ${phase} tasks" STATE.md ||
  note "STATE.md has no '## ${phase} tasks' section for the phase in progress"

# 4. The phase that was just finished left a technical note behind. Every
#    finished backend phase has one; a missing one means the next session has to
#    read the code to learn what the decisions were.
missing_docs=()
for module in geo config identity user merchant catalogue discovery cart pricing order dispatch; do
  [[ -f "docs/technical/${module}.md" ]] || missing_docs+=("${module}")
done
if (( ${#missing_docs[@]} > 0 )); then
  note "docs/technical is missing: ${missing_docs[*]}"
fi

# 5. CLAUDE.md is the other half of the handover.
[[ -f CLAUDE.md ]] || note "CLAUDE.md is gone — the next session has no working agreement"

printf '\n'
if (( ${#problems[@]} > 0 )); then
  printf '\033[31mhandover: not ready\033[0m\n'
  printf '  - %s\n' "${problems[@]}"
  exit 1
fi

printf '\033[32mhandover: ready.\033[0m %s is at %s, pushed to %s.\n' \
  "$(git rev-parse --short HEAD)" "${task}" "${BRANCH}"
cat <<'MSG'

Tell the user, in one line, which phase just finished and which model the next
one wants. Then they open a NEW session on that model with:

    Read CLAUDE.md and STATE.md, then continue from current_task.

A new session is cheaper than switching the model inside this one: a switch
re-sends the whole conversation, a new session reads two files.
MSG
