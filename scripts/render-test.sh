#!/usr/bin/env bash
# Composition unit tests. Renders every case in test/render/<case>/ with
# `crossplane composition render` (real functions, real Crossplane render
# engine, in Docker) and compares the output with the committed golden file.
#
#   scripts/render-test.sh            compare, fail on any difference
#   UPDATE_GOLDEN=1 scripts/render-test.sh   rewrite the golden files
#
# Each case directory holds xr.yaml, a one-line `composition` file naming the
# Composition to use, and expected.yaml.
set -euo pipefail
cd "$(dirname "$0")/.."

CROSSPLANE_IMAGE="${CROSSPLANE_IMAGE:-xpkg.crossplane.io/crossplane/crossplane:v2.4.3@sha256:1c66261d805a73cade46e2ac94b665c73aebd4a00844de5fcae4bcadaa2be638}"
OUT="${OUT:-rendered}"
mkdir -p "$OUT"

status=0
for dir in test/render/*/; do
  name=$(basename "$dir")
  composition=$(<"$dir/composition")
  crossplane composition render "$dir/xr.yaml" "$composition" platform/functions.yaml \
    --xrd apis/appenvironment/definition.yaml \
    --crossplane-image "$CROSSPLANE_IMAGE" \
    --include-full-xr >"$OUT/$name.yaml"
  if [[ "${UPDATE_GOLDEN:-}" == "1" ]]; then
    cp "$OUT/$name.yaml" "$dir/expected.yaml"
    echo "updated  $name"
  elif [[ ! -f "$dir/expected.yaml" ]]; then
    echo "MISSING  $name: no expected.yaml (run with UPDATE_GOLDEN=1)"
    status=1
  elif diff -u "$dir/expected.yaml" "$OUT/$name.yaml"; then
    echo "ok       $name ($(grep -c '^kind:' "$OUT/$name.yaml") objects)"
  else
    echo "FAIL     $name: rendered output differs from expected.yaml"
    status=1
  fi
done
exit "$status"
