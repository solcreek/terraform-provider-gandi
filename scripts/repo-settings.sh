#!/usr/bin/env bash
# Declares the GitHub repository settings this project relies on and
# reconciles them through the GitHub API.
#
#   scripts/repo-settings.sh           # check: report drift, exit 1 if any
#   scripts/repo-settings.sh --apply   # apply the desired settings
#
# Requires gh (authenticated with admin on the repo) and jq.
#
# Why these settings: the OpenTofu Registry indexes every semver git tag and
# treats a published version as immutable (it will not re-index or remove it).
# So a release tag must never move or disappear, and its assets must never be
# replaced; see https://github.com/opentofu/registry/blob/main/POLICY.md.
# Immutable releases are set here rather than with the integrations/github
# provider, which does not support them yet (integrations/terraform-provider-github#2746).
set -euo pipefail

REPO="${REPO:-solcreek/terraform-provider-gandi}"

DESCRIPTION="Terraform/OpenTofu provider for Gandi — domains, nameservers, glue records, DNSSEC keys & LiveDNS via the Gandi v5 API (PAT auth), with a dependency-free client."
TOPICS='["dns","dnssec","domains","gandi","golang","iac","infrastructure-as-code","livedns","nameservers","opentofu","terraform","terraform-provider"]'

# Release tags (v*) may be created, but never updated (moved) or deleted, by
# anyone: there are no bypass actors. Undoing a tag means disabling this
# ruleset on purpose, which is the point.
RULESET_NAME="release-tags"
RULESET=$(jq -n --arg name "$RULESET_NAME" '{
  name: $name,
  target: "tag",
  enforcement: "active",
  bypass_actors: [],
  conditions: {ref_name: {include: ["refs/tags/v*"], exclude: []}},
  rules: [{type: "deletion"}, {type: "update"}]
}')

apply=false
case "${1:-}" in
  --apply) apply=true ;;
  "") ;;
  *) echo "usage: $0 [--apply]" >&2; exit 2 ;;
esac

drift=0
report() { # report <setting> <in-sync?>
  if [[ "$2" == true ]]; then
    echo "ok     $1"
  elif [[ "$apply" == true ]]; then
    echo "apply  $1"
  else
    echo "DRIFT  $1"
    drift=1
  fi
}

# Description and topics.
current_description=$(gh api "repos/$REPO" --jq .description)
in_sync=false; [[ "$current_description" == "$DESCRIPTION" ]] && in_sync=true
report description "$in_sync"
if [[ "$in_sync" == false && "$apply" == true ]]; then
  gh api -X PATCH "repos/$REPO" -f description="$DESCRIPTION" >/dev/null
fi

current_topics=$(gh api "repos/$REPO/topics" --jq '.names | sort')
in_sync=false; [[ "$current_topics" == "$(jq -c 'sort' <<<"$TOPICS")" ]] && in_sync=true
report topics "$in_sync"
if [[ "$in_sync" == false && "$apply" == true ]]; then
  jq -n --argjson names "$TOPICS" '{names: $names}' \
    | gh api -X PUT "repos/$REPO/topics" --input - >/dev/null
fi

# Immutable releases: published release assets cannot be replaced.
enabled=$(gh api "repos/$REPO/immutable-releases" --jq .enabled)
report immutable-releases "$enabled"
if [[ "$enabled" != true && "$apply" == true ]]; then
  gh api -X PUT "repos/$REPO/immutable-releases" >/dev/null
fi

# Release tag ruleset, matched by name.
ruleset_id=$(gh api "repos/$REPO/rulesets" --jq ".[] | select(.name == \"$RULESET_NAME\") | .id")
in_sync=false
if [[ -n "$ruleset_id" ]]; then
  current=$(gh api "repos/$REPO/rulesets/$ruleset_id" | jq -S '{
    name, target, enforcement, bypass_actors,
    conditions: {ref_name: .conditions.ref_name},
    rules: [.rules[] | {type}] | sort_by(.type)
  }')
  desired=$(jq -S '.rules |= sort_by(.type)' <<<"$RULESET")
  [[ "$current" == "$desired" ]] && in_sync=true
fi
report "ruleset $RULESET_NAME" "$in_sync"
if [[ "$in_sync" == false && "$apply" == true ]]; then
  if [[ -n "$ruleset_id" ]]; then
    gh api -X PUT "repos/$REPO/rulesets/$ruleset_id" --input - <<<"$RULESET" >/dev/null
  else
    gh api -X POST "repos/$REPO/rulesets" --input - <<<"$RULESET" >/dev/null
  fi
fi

exit "$drift"
