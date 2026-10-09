#!/usr/bin/env bash
# Runs both writers and counts stderr lines that are not valid JSON.
set -euo pipefail
cd "$(dirname "$0")"

go build -o repro .
status=0
for mode in original fixed; do
  ./repro "$mode" 2>"out-$mode.log"
  total=$(wc -l <"out-$mode.log")
  invalid=$(jq -R 'fromjson? // "INVALID"' "out-$mode.log" | grep -c '^"INVALID"$' || true)
  printf '%-8s %6d lines, %6d invalid JSON\n' "$mode" "$total" "$invalid"
  if [ "$mode" = fixed ] && [ "$invalid" -ne 0 ]; then
    status=1
  fi
done

echo
echo "Sample of invalid lines (original):"
grep -v '^{.*}$' out-original.log | head -5 || true
exit "$status"
