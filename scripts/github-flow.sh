#!/usr/bin/env bash
set -euo pipefail

TICKET_PREFIX="${TICKET_PREFIX:-ZYNQ}"
PROJECT_NUMBER="${PROJECT_NUMBER:-5}"
BASE_BRANCH="${BASE_BRANCH:-master}"

usage() {
  cat <<'EOF'
Usage:
  scripts/github-flow.sh start <issue-number> <feature|fix|refactor|bugfix>
  scripts/github-flow.sh pr <issue-number> [--draft]

Notes:
  - Target Project can be overridden:
    PROJECT_NUMBER=2 scripts/github-flow.sh start 12 feature
  - Branch format is enforced:
    feature/<PREFIX>-00_xxxx
    fix/<PREFIX>-00_xxxx
    refactor/<PREFIX>-00_xxxx
    bugfix/<PREFIX>-00_xxxx
  - Ticket prefix can be overridden:
    TICKET_PREFIX=PROJ scripts/github-flow.sh start 12 feature
  - Requires: gh, git, jq

GitHub Issues Hierarchy:
  Epic    — large initiative, tracked as a GitHub Issue with "epic" label
  Story   — user-facing deliverable within an epic, "story" label
  Task    — implementation unit within a story (or standalone), "task" label

  Branch naming maps from task/story issues:
    feature/ZYNQ-42_add_webhook_retry
    bugfix/ZYNQ-15_fix_pty_leak
    refactor/ZYNQ-23_split_session_manager
EOF
}

is_blank_or_null() {
  local value="${1:-}"
  [[ -z "$value" || "$value" == "null" ]]
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

slugify() {
  local input="$1"
  local slug
  slug="$(printf '%s' "$input" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/_/g; s/^_+//; s/_+$//; s/_+/_/g')"
  slug="${slug:0:40}"
  if [[ -z "$slug" ]]; then
    slug="task"
  fi
  printf '%s' "$slug"
}

ticket_from_issue() {
  local issue_number="$1"
  printf '%s-%02d' "$TICKET_PREFIX" "$issue_number"
}

branch_pattern="^(feature|fix|refactor|bugfix)/${TICKET_PREFIX}-[0-9]+_[a-z0-9][a-z0-9_]*$"

set_project_status() {
  local issue_number="$1"
  local desired_status="$2"
  local owner project_id fields_json field_id option_id item_id

  owner="$(gh repo view --json owner --jq '.owner.login' 2>/dev/null || true)"
  if is_blank_or_null "$owner"; then
    echo "Warning: could not resolve repo owner; skipping Project status update." >&2
    return 0
  fi

  fields_json="$(gh project field-list "$PROJECT_NUMBER" --owner "$owner" --format json 2>/dev/null || true)"
  if is_blank_or_null "$fields_json"; then
    echo "Warning: could not read project fields; skipping Project status update." >&2
    return 0
  fi

  field_id="$(jq -r '.fields[] | select(.name=="Status") | .id // empty' <<<"$fields_json" || true)"
  option_id="$(jq -r --arg s "$desired_status" '.fields[] | select(.name=="Status") | .options[] | select(.name==$s) | .id // empty' <<<"$fields_json" || true)"
  project_id="$(gh project view "$PROJECT_NUMBER" --owner "$owner" --format json --jq '.id // empty' 2>/dev/null || true)"
  item_id="$(gh project item-list "$PROJECT_NUMBER" --owner "$owner" --limit 500 --format json --jq ".items[] | select(.content.number==$issue_number) | .id" 2>/dev/null | head -n1 || true)"

  if is_blank_or_null "$field_id" || is_blank_or_null "$option_id" || is_blank_or_null "$project_id" || is_blank_or_null "$item_id"; then
    echo "Warning: project item or status field not found; skipping Project status update." >&2
    return 0
  fi

  gh project item-edit \
    --id "$item_id" \
    --project-id "$project_id" \
    --field-id "$field_id" \
    --single-select-option-id "$option_id" >/dev/null
}

start_flow() {
  local issue_number="$1"
  local kind="$2"
  local issue_title ticket slug branch

  if [[ ! "$kind" =~ ^(feature|fix|refactor|bugfix)$ ]]; then
    echo "Invalid branch kind: $kind (expected: feature, fix, refactor, bugfix)" >&2
    exit 1
  fi

  if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "Working tree is not clean. Commit or stash changes first." >&2
    exit 1
  fi

  issue_title="$(gh issue view "$issue_number" --json title --jq '.title')"

  # Extract ZYNQ ticket from issue title (e.g. "ZYNQ-05: PTY attach..." → "ZYNQ-05")
  if [[ "$issue_title" =~ ^${TICKET_PREFIX}-([0-9]+) ]]; then
    ticket="${TICKET_PREFIX}-${BASH_REMATCH[1]}"
    # Slug from the part after "ZYNQ-XX: "
    local title_rest="${issue_title#*: }"
    slug="$(slugify "$title_rest")"
  else
    # Fallback: use GitHub issue number if no ZYNQ prefix in title
    ticket="$(ticket_from_issue "$issue_number")"
    slug="$(slugify "$issue_title")"
  fi
  branch="${kind}/${ticket}_${slug}"

  if [[ ! "$branch" =~ $branch_pattern ]]; then
    echo "Generated branch does not match required pattern: $branch" >&2
    exit 1
  fi

  git fetch origin "$BASE_BRANCH"
  git checkout "$BASE_BRANCH"
  git pull --ff-only origin "$BASE_BRANCH"
  git checkout -b "$branch"

  set_project_status "$issue_number" "In Progress"

  echo "Created branch: $branch"
}

open_pr_flow() {
  local issue_number="$1"
  local draft="${2:-}"
  local branch issue_title ticket pr_title tmp_body pr_url

  branch="$(git branch --show-current)"
  if [[ ! "$branch" =~ $branch_pattern ]]; then
    echo "Current branch does not match required pattern: $branch" >&2
    exit 1
  fi

  issue_title="$(gh issue view "$issue_number" --json title --jq '.title')"

  # Extract ZYNQ ticket from issue title if present
  if [[ "$issue_title" =~ ^${TICKET_PREFIX}-([0-9]+) ]]; then
    ticket="${TICKET_PREFIX}-${BASH_REMATCH[1]}"
  else
    ticket="$(ticket_from_issue "$issue_number")"
  fi
  pr_title="[$ticket] $issue_title"

  git push -u origin "$branch"

  tmp_body="$(mktemp)"
  cat >"$tmp_body" <<EOF
## Summary
- ${issue_title}

## Validation
- [ ] \`go build ./...\`
- [ ] \`go test ./...\`
- [ ] \`go vet ./...\`
- [ ] local verification completed

## Linked
- Closes #${issue_number}

## Project
- [ ] Project item is linked
- [ ] Status is \`In Progress\` while review is open
EOF

  if [[ "$draft" == "--draft" ]]; then
    pr_url="$(gh pr create --draft --base "$BASE_BRANCH" --head "$branch" --title "$pr_title" --body-file "$tmp_body")"
  else
    pr_url="$(gh pr create --base "$BASE_BRANCH" --head "$branch" --title "$pr_title" --body-file "$tmp_body")"
  fi
  rm -f "$tmp_body"

  set_project_status "$issue_number" "In Progress"

  echo "PR created: $pr_url"
}

main() {
  require_cmd gh
  require_cmd git
  require_cmd jq

  local cmd="${1:-}"
  case "$cmd" in
    start)
      [[ $# -eq 3 ]] || {
        usage
        exit 1
      }
      start_flow "$2" "$3"
      ;;
    pr)
      [[ $# -ge 2 && $# -le 3 ]] || {
        usage
        exit 1
      }
      open_pr_flow "$2" "${3:-}"
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
