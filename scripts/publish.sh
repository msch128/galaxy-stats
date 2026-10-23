#!/usr/bin/env bash
# Publishes a directory of rendered cards to a branch as a single commit.
#
# Usage: publish.sh <cards-dir> <branch>
#
# Run inside a checkout with push access (actions/checkout does this). The
# branch always holds exactly one commit, so it never grows; nothing is pushed
# when the cards are unchanged.
set -euo pipefail

cards_dir=$1
branch=$2

if [ -z "$(ls -A "$cards_dir")" ]; then
  echo "No cards rendered; nothing to publish." >&2
  exit 1
fi

if git fetch --quiet --depth=1 origin "refs/heads/$branch" 2>/dev/null; then
  previous=$(mktemp -d)
  git archive FETCH_HEAD | tar -x -C "$previous"
  if diff -rq "$previous" "$cards_dir" >/dev/null; then
    echo "Cards unchanged; nothing to publish."
    exit 0
  fi
fi

export GIT_INDEX_FILE
GIT_INDEX_FILE=$(mktemp -u)
git --work-tree="$cards_dir" add --all
tree=$(git write-tree)
commit=$(
  GIT_AUTHOR_NAME="github-actions[bot]" \
  GIT_AUTHOR_EMAIL="41898282+github-actions[bot]@users.noreply.github.com" \
  GIT_COMMITTER_NAME="github-actions[bot]" \
  GIT_COMMITTER_EMAIL="41898282+github-actions[bot]@users.noreply.github.com" \
    git commit-tree "$tree" -m "Render galaxy-stats cards"
)
git push --quiet --force origin "$commit:refs/heads/$branch"
echo "Published cards to $branch."
