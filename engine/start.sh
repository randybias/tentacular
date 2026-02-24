#!/bin/sh
# Tentacular engine startup script.
# Extracts vendored dependencies if present, then launches the Deno engine
# with the appropriate --import-map flag so pods need zero outbound network
# access to resolve remote imports (jsr:, https://).
set -e

IMPORT_MAP_FLAG=""

if [ -f /app/workflow/vendor.tar.gz ]; then
  echo "Extracting vendor dependencies..."
  mkdir -p /tmp/vendor
  tar xzf /app/workflow/vendor.tar.gz -C /tmp/vendor/
  if [ -f /tmp/vendor/vendor/import_map.json ]; then
    IMPORT_MAP_FLAG="--import-map=/tmp/vendor/vendor/import_map.json"
    echo "Using vendored imports: /tmp/vendor/vendor/import_map.json"
  else
    echo "Warning: vendor.tar.gz present but import_map.json not found inside — proceeding without import map"
  fi
fi

exec deno run \
  --no-lock \
  --unstable-net \
  --allow-net \
  --allow-read=/app,/var/run/secrets,/tmp \
  --allow-write=/tmp \
  --allow-env \
  ${IMPORT_MAP_FLAG} \
  engine/main.ts \
  --workflow /app/workflow/workflow.yaml \
  --port 8080
