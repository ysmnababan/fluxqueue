#!/bin/sh
set -eu

OUT_FILE="${1:-/prom_targets/worker_targets.json}"
NETWORK_NAME="${2:-monitoring}"
WORKER_NAME_SUBSTR="${3:-worker}"

TMP_FILE="${OUT_FILE}.tmp"

mkdir -p "$(dirname "$OUT_FILE")"

echo "[" >"$TMP_FILE"
echo '  { "targets": [' >>"$TMP_FILE"

first=1
count=0

for cname in $(docker ps --format '{{.Names}}' | grep "$WORKER_NAME_SUBSTR" || true); do
  ip=$(docker inspect -f \
    "{{with index .NetworkSettings.Networks \"$NETWORK_NAME\"}}{{.IPAddress}}{{end}}" \
    "$cname" 2>/dev/null || true)

  if [ -n "$ip" ]; then
    if [ "$first" -eq 1 ]; then
      printf '    "%s:8090"' "$ip" >>"$TMP_FILE"
      first=0
    else
      printf ',\n    "%s:8090"' "$ip" >>"$TMP_FILE"
    fi
    count=$(expr "$count" + 1)
  fi
done

echo '' >>"$TMP_FILE"
echo '  ], "labels": { "job": "worker" } }' >>"$TMP_FILE"
echo ']' >>"$TMP_FILE"

if [ "$count" -eq 0 ]; then
  echo "[]" >"$TMP_FILE"
fi

mv "$TMP_FILE" "$OUT_FILE"

echo "Wrote $count worker targets to $OUT_FILE"
