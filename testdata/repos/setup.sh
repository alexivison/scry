#!/usr/bin/env bash
# setup.sh — Creates fixture Git repositories for end-to-end tests.
# Called by Go tests via TestMain or helper; idempotent (removes target first).
set -euo pipefail

DEST="${1:?usage: setup.sh <target-dir>}"
rm -rf "$DEST"
mkdir -p "$DEST"

GIT="git -c user.name=test -c user.email=test@test.com -c protocol.file.allow=always"

# ---------- simple ----------
setup_simple() {
  local d="$DEST/simple"
  mkdir -p "$d" && cd "$d"
  $GIT init -q
  echo "hello" > greet.txt
  echo "package main" > main.go
  echo "deleteme" > old.txt
  $GIT add .
  $GIT commit -q -m "initial"

  echo "hello world" > greet.txt
  echo -e "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }" > main.go
  rm old.txt
  echo "brand new" > new.txt
  $GIT add .
  $GIT commit -q -m "changes"
}

# ---------- rename ----------
setup_rename() {
  local d="$DEST/rename"
  mkdir -p "$d" && cd "$d"
  $GIT init -q
  echo -e "package util\n\nfunc Helper() {}\n" > util.go
  $GIT add .
  $GIT commit -q -m "initial"

  $GIT mv util.go helper.go
  echo -e "package util\n\nfunc Helper() {}\n\nfunc Extra() {}\n" > helper.go
  $GIT add .
  $GIT commit -q -m "rename and modify"
}

# ---------- binary ----------
setup_binary() {
  local d="$DEST/binary"
  mkdir -p "$d" && cd "$d"
  $GIT init -q
  # NUL bytes force git to detect as binary.
  printf '\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR' > image.png
  $GIT add .
  $GIT commit -q -m "initial"

  printf '\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR\x00\x00MORE' > image.png
  printf '\x00\x01\x02' > data.bin
  $GIT add .
  $GIT commit -q -m "modify binary"
}

# ---------- submodule ----------
setup_submodule() {
  local d="$DEST/submodule"
  local sub="$DEST/_sub_upstream"

  # Create a small upstream repo for the submodule.
  mkdir -p "$sub" && cd "$sub"
  $GIT init -q
  echo "v1" > version.txt
  $GIT add .
  $GIT commit -q -m "sub v1"

  # Main repo with submodule.
  mkdir -p "$d" && cd "$d"
  $GIT init -q
  echo "root" > root.txt
  $GIT add .
  $GIT commit -q -m "initial"

  $GIT submodule add -q "$sub" sub
  $GIT commit -q -m "add submodule"

  # Advance the upstream.
  cd "$sub"
  echo "v2" > version.txt
  $GIT add .
  $GIT commit -q -m "sub v2"

  # Update submodule pointer: fetch new commit, checkout, stage.
  cd "$d"
  $GIT -C sub fetch -q origin
  $GIT -C sub checkout -q FETCH_HEAD
  $GIT add sub
  $GIT commit -q -m "bump submodule"
}

# ---------- large ----------
setup_large() {
  local d="$DEST/large"
  mkdir -p "$d" && cd "$d"
  $GIT init -q
  echo "start" > big.txt
  $GIT add .
  $GIT commit -q -m "initial"

  # Generate a file exceeding 50k lines (portable, no python dependency).
  seq 0 54999 | awk '{print "line " $1}' > big.txt
  $GIT add .
  $GIT commit -q -m "large file"
}

# ---------- linked-worktree ----------
setup_linked_worktree() {
  local d="$DEST/linked-worktree"
  local main_repo="$d/main"
  local wt="$d/wt"

  mkdir -p "$d"
  mkdir -p "$main_repo" && cd "$main_repo"
  $GIT init -q
  echo "base content" > file.txt
  $GIT add .
  $GIT commit -q -m "initial"

  echo "modified" > file.txt
  $GIT add .
  $GIT commit -q -m "second commit"

  # Create a linked worktree.
  $GIT worktree add "$wt" -b wt-branch HEAD~1
}

# ---------- no-divergence ----------
setup_no_divergence() {
  local d="$DEST/no-divergence"
  mkdir -p "$d" && cd "$d"
  $GIT init -q -b main
  echo "same on both sides" > readme.txt
  $GIT add .
  $GIT commit -q -m "initial"

  # Create a branch at the same commit — HEAD == base, empty diff.
  $GIT checkout -q -b feature
}

# ---------- staged-simple ----------
setup_staged_simple() {
  local d="$DEST/staged-simple"
  mkdir -p "$d" && cd "$d"
  $GIT init -q
  echo "package main" > main.go
  echo "old content" > readme.txt
  $GIT add .
  $GIT commit -q -m "initial"

  # Stage some changes but do not commit.
  echo -e "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }" > main.go
  echo "new file" > added.txt
  $GIT add main.go added.txt
}

# ---------- watch-divergence ----------
setup_watch_divergence() {
  local d="$DEST/watch-divergence"
  mkdir -p "$d" && cd "$d"
  $GIT init -q
  echo "line 1" > tracked.txt
  $GIT add .
  $GIT commit -q -m "initial"
  $GIT checkout -q -b feature

  echo "line 2" >> tracked.txt
  $GIT add .
  $GIT commit -q -m "feature work"
}

setup_simple
setup_rename
setup_binary
setup_submodule
setup_large
setup_linked_worktree
setup_no_divergence
setup_staged_simple
setup_watch_divergence

echo "Fixtures created in $DEST"
