#!/usr/bin/env bash
# Installs the platform on an empty cluster: Crossplane, the CloudNativePG
# operator, providers, functions, the AppEnvironment API and admission policy.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=e2e/lib.sh
source e2e/lib.sh

t0=$(now_ms)

log "Crossplane ${CROSSPLANE_CHART_VERSION}"
helm upgrade --install crossplane crossplane \
  --repo https://charts.crossplane.io/stable --version "$CROSSPLANE_CHART_VERSION" \
  --namespace crossplane-system --create-namespace \
  --set image.tag="$CROSSPLANE_IMAGE_TAG" \
  --set "args=${CROSSPLANE_ARGS}" \
  --wait --timeout 5m

log "CloudNativePG operator (chart ${CNPG_CHART_VERSION})"
helm upgrade --install cnpg cloudnative-pg \
  --repo https://cloudnative-pg.github.io/charts --version "$CNPG_CHART_VERSION" \
  --namespace cnpg-system --create-namespace \
  --set image.tag="$CNPG_IMAGE_TAG" \
  --wait --timeout 5m

log "Providers and functions"
kubectl apply -f platform/providers.yaml -f platform/functions.yaml
wait_until 300 "provider and function package CRDs" \
  crd_exists clusterproviderconfigs.kubernetes.m.crossplane.io
wait_until 300 "provider-helm CRDs" crd_exists clusterproviderconfigs.helm.m.crossplane.io
kubectl wait providers.pkg.crossplane.io --all --for=condition=Healthy --timeout=5m
kubectl wait functions.pkg.crossplane.io --all --for=condition=Healthy --timeout=5m
kubectl apply -f platform/provider-configs.yaml

log "AppEnvironment API"
kubectl apply -f apis/appenvironment/definition.yaml
kubectl wait xrd appenvironments.platform.example.io --for=condition=Established --timeout=2m
wait_until 120 "AppEnvironment CRD" crd_exists appenvironments.platform.example.io
kubectl apply -f apis/appenvironment/composition-kind.yaml

log "Teams and admission policy"
kubectl apply -f platform/teams.yaml -f platform/policy/team-policy.yaml

record platform_install_s "$(secs "$t0" "$(now_ms)")" "Platform install on an empty kind cluster (Crossplane, CNPG operator, providers, functions, API, policy)"
kubectl get providers.pkg.crossplane.io,functions.pkg.crossplane.io
