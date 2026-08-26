#!/bin/bash
# Build script for tiny-claw dashboard frontend.
# This script builds the React frontend and prepares it for embedding into the Go binary.

set -e

SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WEB_DIR="$PROJECT_ROOT/web"

echo "Building tiny-claw dashboard..."
echo "Project root: $PROJECT_ROOT"
echo "Web directory: $WEB_DIR"

cd "$WEB_DIR"

# Check if package.json exists
if [ ! -f "package.json" ]; then
    echo "Error: package.json not found in $WEB_DIR"
    exit 1
fi

# Install dependencies
echo "Installing npm dependencies..."
npm ci

# Run TypeScript type check
echo "Running TypeScript type check..."
npx tsc --noEmit

# Build production bundle
echo "Building production bundle..."
npm run build

# Verify build output
if [ ! -d "dist" ]; then
    echo "Error: dist directory not created"
    exit 1
fi

if [ ! -f "dist/index.html" ]; then
    echo "Error: index.html not found in dist/"
    exit 1
fi

echo "Dashboard build completed successfully!"
echo "Output: $WEB_DIR/dist/"
ls -la dist/
