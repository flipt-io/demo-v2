#!/bin/sh
set -e

echo "Checking if repo already exists..."
if curl -sf http://ogit:8080/onoffinc-features.git/info/refs > /dev/null; then
  echo "Repo already exists, skipping."
  exit 0
fi

echo "Creating repo..."
curl -sf -X POST http://ogit:8080/api/repo \
  -H 'Content-Type: application/json' \
  -d '{"name": "onoffinc-features"}' || true

echo "Cloning and pushing initial content..."
if [ -d /tmp/repo ]; then
  echo "Repo already cloned, skipping."
  exit 0
fi
git clone http://ogit:8080/onoffinc-features.git /tmp/repo
cd /tmp/repo
git config user.email 'init@local'
git config user.name 'init'
mkdir -p default
cp /init/default-features.yaml ./default/features.yaml
git add .
git commit -m 'initial commit'
git push origin main

echo "Done."
