#!/bin/bash
set -e

cd "$(dirname "$0")/../web"
npm ci
npm run build
