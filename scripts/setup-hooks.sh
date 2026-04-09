#!/bin/bash

# Setup script for Git hooks
# Run this after cloning the repository

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$REPO_ROOT/.git-hooks"
GIT_HOOKS_DIR="$REPO_ROOT/.git/hooks"

echo "🔧 Setting up Git hooks for Version Guard..."

# Check if .git directory exists
if [ ! -d "$REPO_ROOT/.git" ]; then
    echo "❌ Error: .git directory not found. Are you in a Git repository?"
    exit 1
fi

# Make hook scripts executable
if [ -d "$HOOKS_DIR" ]; then
    chmod +x "$HOOKS_DIR"/*
    echo "✅ Made hook scripts executable"
else
    echo "❌ Error: .git-hooks directory not found"
    exit 1
fi

# Create symlinks from .git/hooks to .git-hooks
for hook in "$HOOKS_DIR"/*; do
    hook_name=$(basename "$hook")
    ln -sf "../../.git-hooks/$hook_name" "$GIT_HOOKS_DIR/$hook_name"
    echo "✅ Installed $hook_name hook"
done

echo ""
echo "✅ Git hooks setup complete!"
echo ""
echo "Installed hooks:"
echo "  - pre-push: Runs linting and tests before push"
echo ""
echo "To skip hooks temporarily (not recommended):"
echo "  git push --no-verify"
echo ""
echo "To run checks manually:"
echo "  make lint test"
