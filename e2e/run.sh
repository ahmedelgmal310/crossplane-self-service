#!/usr/bin/env bash
# End-to-end proof of the self-service platform. Every check either passes or
# fails the job; every timing in the README comes from record() calls here.
#
#   1. RBAC: developers can create AppEnvironments in their team namespace only
#   2. Policy: five invalid claims are rejected by the API server
#   3. Provision: small/no-DB and medium/DB+public claims, timed to Ready=True
#   4. Contents: quota, network policies, ingress, Secret contract per size
#   5. Database: the app reaches PostgreSQL with the delivered Secret;
#      network policy blocks everyone else
#   6. Day 2: resize small -> medium; spec.database is immutable
#   7. Drift: out-of-band delete/edit is reverted, timed
#   8. Delete: everything an environment created is garbage-collected
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=e2e/lib.sh
source e2e/lib.sh

PLATFORMCTL="${PLATFORMCTL:-bin/platformctl}"
SMALL=preview
MEDIUM=checkout-api
SMALL_NS="payments-${SMALL}"
MEDIUM_NS="payments-${MEDIUM}"

diagnostics() {
  log "Diagnostics"
  kubectl get appenvironments -A -o wide || true
  kubectl get managed -A || true
  kubectl get usages.protection.crossplane.io -A || true
  for xr in $(kubectl get appenvironments -n "$TEAM_NS" -o name 2>/dev/null); do
    kubectl describe -n "$TEAM_NS" "$xr" | tail -40 || true
  done
  kubectl get objects.kubernetes.m.crossplane.io -n "$TEAM_NS" -o json 2>/dev/null |
    jq -r '.items[] | select(([.status.conditions[]? | select(.status != "True")] | length) > 0)
      | "\(.metadata.name): \([.status.conditions[]? | "\(.type)=\(.status) \(.reason) \(.message // "")"] | join("; "))"' || true
  kubectl get releases.helm.m.crossplane.io -n "$TEAM_NS" -o yaml | grep -A12 'conditions:' || true
  kubectl get pods -A -o wide | grep -vE 'kube-system' || true
  kubectl get events -A --sort-by=.lastTimestamp | tail -60 || true
  kubectl logs -n crossplane-system deploy/crossplane --tail=60 || true
}
trap 'rc=$?; if [[ $rc -ne 0 ]]; then diagnostics; fi' EXIT

dev_apply() { kubectl apply "${DEV_PAYMENTS[@]}" -f "$@"; }

# ---------------------------------------------------------------------------
log "1. RBAC: a payments developer can only manage AppEnvironments in team-payments"
[[ "$(kubectl auth can-i create appenvironments -n team-payments "${DEV_PAYMENTS[@]}")" == "yes" ]] ||
  fail "payments developer cannot create AppEnvironments in team-payments"
pass "can create appenvironments in team-payments"
for check in "create appenvironments -n team-growth" "create namespaces" \
  "create resourcequotas -n team-payments" "create deployments -n team-payments" \
  "get secrets -n team-payments"; do
  # shellcheck disable=SC2086 # word splitting of the verb/resource is intended
  if [[ "$(kubectl auth can-i $check "${DEV_PAYMENTS[@]}" || true)" != "no" ]]; then
    fail "payments developer should not be allowed to: $check"
  fi
  pass "cannot $check"
done

# ---------------------------------------------------------------------------
log "2. Policy: invalid claims are rejected at admission"
# The policy is enforced as soon as the API server has compiled it; wait for
# that with a server-side dry run, then test for real.
wait_until 60 "admission policy to be enforced" \
  bash -c "! kubectl create --dry-run=server ${DEV_PAYMENTS[*]} -f examples/invalid/missing-team-label.yaml"

expect_rejected() { # <file> <expected message> <kubectl identity...>
  local file=$1 want=$2 out
  shift 2
  if out=$(kubectl create "$@" -f "$file" 2>&1); then
    fail "$file was accepted but must be rejected"
  fi
  grep -qF -- "$want" <<<"$out" || fail "$file rejected for the wrong reason: $out"
  pass "$(basename "$file"): ${out#*: }"
  printf '| `%s` | %s |\n' "$(basename "$file")" "$(tr '\n|' ' /' <<<"$out" | sed 's/  */ /g')" >>"$RESULTS_DIR/rejections.md"
}
: >"$RESULTS_DIR/rejections.md"
expect_rejected examples/invalid/missing-team-label.yaml "must carry the label platform.example.io/team" "${DEV_PAYMENTS[@]}"
expect_rejected examples/invalid/growth-medium.yaml "team growth may request at most size small, got medium" "${DEV_GROWTH[@]}"
expect_rejected examples/invalid/wrong-team-namespace.yaml "does not match the team that owns namespace team-payments" "${DEV_PAYMENTS[@]}"
expect_rejected examples/invalid/unknown-size.yaml 'Unsupported value: "xlarge"' "${DEV_PAYMENTS[@]}"
expect_rejected examples/invalid/public-without-hostname.yaml "spec.hostname is required when spec.exposure is public" "${DEV_PAYMENTS[@]}"
[[ -z "$(kubectl get appenvironments -A --no-headers 2>/dev/null)" ]] || fail "a rejected claim was persisted"
record rejected_claims 5 "Invalid claims rejected at admission (schema, CEL and ValidatingAdmissionPolicy), none persisted"

# ---------------------------------------------------------------------------
log "3. Provision two environments as a developer"
# The small claim is scaffolded by platformctl; it must match the example.
"$PLATFORMCTL" new --name "$SMALL" --team payments --size small --image env-probe:e2e >"$RESULTS_DIR/preview.yaml"
diff -u examples/preview.yaml "$RESULTS_DIR/preview.yaml" || fail "platformctl output drifted from examples/preview.yaml"
pass "platformctl output matches examples/preview.yaml"

declare -A started ready_at
started[$SMALL]=$(now_ms)
dev_apply "$RESULTS_DIR/preview.yaml"
started[$MEDIUM]=$(now_ms)
dev_apply examples/checkout-api.yaml

deadline=$(($(date +%s) + 900))
while [[ -z "${ready_at[$SMALL]:-}" || -z "${ready_at[$MEDIUM]:-}" ]]; do
  for xr in "$SMALL" "$MEDIUM"; do
    if [[ -z "${ready_at[$xr]:-}" ]] && xr_ready "$xr"; then
      ready_at[$xr]=$(now_ms)
      pass "$xr Ready=True after $(secs "${started[$xr]}" "${ready_at[$xr]}")s"
    fi
  done
  (($(date +%s) < deadline)) || fail "environments not Ready within 900s"
  sleep 0.5
done
record ready_small_s "$(secs "${started[$SMALL]}" "${ready_at[$SMALL]}")" "Claim to Ready=True: small, internal, no database"
record ready_medium_db_s "$(secs "${started[$MEDIUM]}" "${ready_at[$MEDIUM]}")" "Claim to Ready=True: medium, public, PostgreSQL (includes chart install, initdb and the app passing a DB readiness query)"

count_composed() { kubectl get objects.kubernetes.m.crossplane.io,releases.helm.m.crossplane.io,usages.protection.crossplane.io \
  -n "$TEAM_NS" -l "crossplane.io/composite=$1" --no-headers 2>/dev/null | wc -l | tr -d ' '; }
small_count=$(count_composed "$SMALL")
medium_count=$(count_composed "$MEDIUM")
record composed_small "$small_count" "Resources composed for the small claim"
record composed_medium "$medium_count" "Resources composed for the medium + database + public claim"
kubectl get appenvironments -n "$TEAM_NS" -o wide

# ---------------------------------------------------------------------------
log "4. Environment contents follow the claim"
jp() { kubectl get "$@" 2>/dev/null; }
[[ "$(jp ns "$SMALL_NS" -o jsonpath='{.metadata.labels.platform\.example\.io/team}')" == "payments" ]] || fail "namespace team label"
[[ "$(jp resourcequota env-quota -n "$SMALL_NS" -o jsonpath='{.spec.hard.pods}')" == "10" ]] || fail "small quota"
[[ "$(jp resourcequota env-quota -n "$MEDIUM_NS" -o jsonpath='{.spec.hard.pods}')" == "20" ]] || fail "medium quota"
[[ "$(jp resourcequota env-quota -n "$MEDIUM_NS" -o jsonpath='{.spec.hard.limits\.memory}')" == "8Gi" ]] || fail "medium memory quota"
pass "quota: small pods=10, medium pods=20 limits.memory=8Gi"
[[ "$(jp deploy app -n "$SMALL_NS" -o jsonpath='{.status.readyReplicas}')" == "1" ]] || fail "small replicas"
[[ "$(jp deploy app -n "$MEDIUM_NS" -o jsonpath='{.status.readyReplicas}')" == "2" ]] || fail "medium replicas"
pass "replicas: small 1, medium 2 (all ready)"
small_np=$(jp networkpolicy -n "$SMALL_NS" -o jsonpath='{.items[*].metadata.name}')
medium_np=$(jp networkpolicy -n "$MEDIUM_NS" -o jsonpath='{.items[*].metadata.name}')
[[ "$small_np" == "allow-dns allow-same-team default-deny" ]] || fail "small network policies: $small_np"
[[ "$medium_np" == "allow-app-to-db allow-db allow-dns allow-ingress-controller allow-same-team default-deny" ]] || fail "medium network policies: $medium_np"
pass "network policies: small [$small_np], medium [$medium_np]"
[[ "$(jp ingress app -n "$MEDIUM_NS" -o jsonpath='{.spec.rules[0].host}')" == "checkout.payments.apps.example.test" ]] || fail "ingress host"
! jp ingress app -n "$SMALL_NS" >/dev/null || fail "internal environment must not have an ingress"
pass "ingress only for exposure=public"
! jp secret app-db -n "$SMALL_NS" >/dev/null || fail "no-database environment must not get app-db"
keys=$(jp secret app-db -n "$MEDIUM_NS" -o json | jq -r '.data | keys | join(",")')
[[ "$keys" == "DATABASE_URL,PGDATABASE,PGHOST,PGPASSWORD,PGPORT,PGUSER" ]] || fail "app-db keys: $keys"
pass "Secret app-db delivered with keys $keys"
[[ "$(jp appenvironment "$MEDIUM" -n "$TEAM_NS" -o jsonpath='{.status.databaseSecret}')" == "app-db" ]] || fail "status.databaseSecret"
[[ "$(jp appenvironment "$MEDIUM" -n "$TEAM_NS" -o jsonpath='{.status.endpoint}')" == "http://app.${MEDIUM_NS}.svc.cluster.local" ]] || fail "status.endpoint"
pass "XR status reports namespace, endpoint and databaseSecret"

# ---------------------------------------------------------------------------
log "5. The app reaches its database with the delivered Secret"
kubectl port-forward -n "$MEDIUM_NS" svc/app 18080:80 >"$RESULTS_DIR/port-forward.log" 2>&1 &
pf=$!
wait_until 30 "port-forward" curl -fsS http://127.0.0.1:18080/healthz
db=$(curl -fsS http://127.0.0.1:18080/db)
write1=$(curl -fsS -X POST http://127.0.0.1:18080/db/write)
write2=$(curl -fsS -X POST http://127.0.0.1:18080/db/write)
kill "$pf" 2>/dev/null || true
echo "$db" | jq . | tee "$RESULTS_DIR/db.json"
[[ "$(jq -r .connected <<<"$db")" == "true" && "$(jq -r .info.user <<<"$db")" == "app" ]] || fail "app is not connected to its database: $db"
rows1=$(jq -r .rows <<<"$write1")
rows2=$(jq -r .rows <<<"$write2")
((rows2 == rows1 + 1)) || fail "writes did not persist: $write1 then $write2"
pass "connected as $(jq -r .info.user <<<"$db") to $(jq -r .info.database <<<"$db") (PostgreSQL $(jq -r .info.server_version <<<"$db")), query $(jq -r .latency_ms <<<"$db") ms; two writes -> rows $rows1, $rows2"
record db_server_version "$(jq -r .info.server_version <<<"$db")" "PostgreSQL version the app connected to via DATABASE_URL from Secret app-db"
record db_query_ms "$(jq -r .latency_ms <<<"$db")" "App query latency to its database (ms)"

# pg_isready from a throwaway pod: exit 0 = accepting, 2 = no response.
pg_probe() { # <name> <namespace> <labels>
  local name=$1 ns=$2 labels=$3
  kubectl run "$name" -n "$ns" --image="$PG_IMAGE" --restart=Never --labels="$labels" \
    --overrides='{"spec":{"securityContext":{"runAsNonRoot":true,"runAsUser":26,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"'"$name"'","image":"'"$PG_IMAGE"'","command":["pg_isready","-h","db-rw.'"$MEDIUM_NS"'.svc","-p","5432","-t","5"],"resources":{"requests":{"cpu":"50m","memory":"32Mi"},"limits":{"cpu":"100m","memory":"64Mi"}},"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}' \
    >/dev/null
  wait_until 120 "$name to finish" bash -c "kubectl get pod $name -n $ns -o jsonpath='{.status.phase}' | grep -qE 'Succeeded|Failed'"
  kubectl get pod "$name" -n "$ns" -o jsonpath='{.status.containerStatuses[0].state.terminated.exitCode}'
  kubectl delete pod "$name" -n "$ns" --wait=false >/dev/null
}
rc=$(pg_probe np-app "$MEDIUM_NS" app.kubernetes.io/name=app)
[[ "$rc" == "0" ]] || fail "pod with the app label in the environment namespace could not reach the database (pg_isready exit $rc)"
pass "app-labelled pod in $MEDIUM_NS -> db:5432 accepted"
rc=$(pg_probe np-other "$MEDIUM_NS" app.kubernetes.io/name=intruder)
[[ "$rc" == "2" ]] || fail "unlabelled pod in the environment namespace reached the database (pg_isready exit $rc)"
pass "other pod in $MEDIUM_NS -> db:5432 blocked (no response)"
rc=$(pg_probe np-outside default app.kubernetes.io/name=app)
[[ "$rc" == "2" ]] || fail "pod in another namespace reached the database (pg_isready exit $rc)"
pass "pod in namespace default -> db:5432 blocked (no response)"
record netpol_checks "3/3" "Network policy probes: app allowed, other pod in namespace blocked, other namespace blocked"

# ---------------------------------------------------------------------------
log "6. Day 2: resize ${SMALL} small -> medium"
t=$(now_ms)
kubectl patch appenvironment "$SMALL" -n "$TEAM_NS" "${DEV_PAYMENTS[@]}" --type merge -p '{"spec":{"size":"medium"}}'
resized() {
  [[ "$(jp resourcequota env-quota -n "$SMALL_NS" -o jsonpath='{.spec.hard.pods}')" == "20" ]] &&
    [[ "$(jp deploy app -n "$SMALL_NS" -o jsonpath='{.status.readyReplicas}')" == "2" ]] &&
    xr_ready "$SMALL"
}
wait_until 300 "resize to reconcile" resized
record resize_s "$(secs "$t" "$(now_ms)")" "Resize small -> medium: quota updated, second replica ready, XR Ready again"
pass "quota pods=20, 2/2 replicas ready, Ready=True"

if out=$(kubectl patch appenvironment "$MEDIUM" -n "$TEAM_NS" "${DEV_PAYMENTS[@]}" --type merge -p '{"spec":{"database":false}}' 2>&1); then
  fail "turning off the database was accepted"
fi
grep -qF "spec.database is immutable" <<<"$out" || fail "wrong rejection: $out"
pass "spec.database=false rejected: immutable"
if out=$(kubectl label appenvironment "$SMALL" -n "$TEAM_NS" "${DEV_PAYMENTS[@]}" --overwrite platform.example.io/team=growth 2>&1); then
  fail "relabelling the team was accepted"
fi
grep -qE "does not match the team|label is immutable" <<<"$out" || fail "wrong rejection: $out"
pass "team relabel rejected"

# ---------------------------------------------------------------------------
log "7. Drift correction"
t=$(now_ms)
kubectl delete networkpolicy default-deny -n "$SMALL_NS"
wait_until 300 "default-deny NetworkPolicy to be recreated" jp networkpolicy default-deny -n "$SMALL_NS"
record drift_netpol_s "$(secs "$t" "$(now_ms)")" "kubectl delete of the default-deny NetworkPolicy until it exists again"
pass "deleted NetworkPolicy recreated"

t=$(now_ms)
kubectl patch resourcequota env-quota -n "$SMALL_NS" --type merge -p '{"spec":{"hard":{"pods":"999"}}}'
quota_reverted() { [[ "$(jp resourcequota env-quota -n "$SMALL_NS" -o jsonpath='{.spec.hard.pods}')" == "20" ]]; }
wait_until 300 "ResourceQuota edit to be reverted" quota_reverted
record drift_quota_s "$(secs "$t" "$(now_ms)")" "Manual quota raise (pods 20 -> 999) until reverted to 20"
pass "hand-edited ResourceQuota reverted"

t=$(now_ms)
kubectl delete objects.kubernetes.m.crossplane.io "${SMALL}-deployment" -n "$TEAM_NS"
app_back() {
  [[ "$(jp objects.kubernetes.m.crossplane.io "${SMALL}-deployment" -n "$TEAM_NS" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')" == "True" ]] &&
    [[ "$(jp deploy app -n "$SMALL_NS" -o jsonpath='{.status.readyReplicas}')" == "2" ]]
}
wait_until 300 "deleted Deployment MR to be recomposed" app_back
record drift_mr_s "$(secs "$t" "$(now_ms)")" "Delete the composed Deployment Object (and with it the Deployment) until both are back and 2/2 ready"
pass "deleted managed resource recomposed by Crossplane"
wait_until 120 "XR Ready after drift" xr_ready "$SMALL"

# ---------------------------------------------------------------------------
log "8. Delete and prove nothing is orphaned"
leftovers() { # <xr> <env-namespace>: prints anything that still exists
  local xr=$1 ns=$2
  jp ns "$ns" -o name || true
  jp objects.kubernetes.m.crossplane.io,releases.helm.m.crossplane.io,usages.protection.crossplane.io \
    -n "$TEAM_NS" -l "crossplane.io/composite=$xr" -o name || true
  jp clusters.postgresql.cnpg.io -A -o json | jq -r --arg ns "$ns" '.items[] | select(.metadata.namespace == $ns) | "cluster/\(.metadata.name)"' || true
  jp pv -o json | jq -r --arg ns "$ns" '.items[] | select(.spec.claimRef.namespace == $ns) | "pv/\(.metadata.name)"' || true
  for kind in networkpolicies resourcequotas secrets deployments services ingresses; do
    jp "$kind" -A -l "platform.example.io/environment=$xr" -o name || true
  done
}
no_leftovers() { [[ -z "$(leftovers "$@")" ]]; }

for xr in "$MEDIUM" "$SMALL"; do
  ns="payments-${xr}"
  t=$(now_ms)
  kubectl delete appenvironment "$xr" -n "$TEAM_NS" "${DEV_PAYMENTS[@]}" --wait=false
  wait_until 600 "$xr to be deleted" bash -c "! kubectl get appenvironment $xr -n $TEAM_NS"
  wait_until 300 "everything $xr created to be gone" no_leftovers "$xr" "$ns"
  record "delete_${xr//-/_}_s" "$(secs "$t" "$(now_ms)")" "Delete $xr until the XR, its namespace, managed resources, PVs and labelled objects are all gone"
  pass "$xr: no orphans (namespace, MRs, Usage, CNPG cluster, PVs, labelled objects)"
done
[[ -z "$(kubectl get managed -A --no-headers 2>/dev/null)" ]] || fail "managed resources left behind"
[[ -z "$(kubectl get usages.protection.crossplane.io -A --no-headers 2>/dev/null)" ]] || fail "usages left behind"
pass "cluster has zero managed resources left"

# ---------------------------------------------------------------------------
log "Summary"
{
  echo "## E2E results"
  echo
  echo "| Measurement | Value | What it measures |"
  echo "|---|---|---|"
  while IFS=$'\t' read -r key value desc; do
    echo "| \`$key\` | **$value** | $desc |"
  done <"$RESULTS_TSV"
  echo
  echo "### Rejected claims"
  echo
  echo "| Claim | API server response |"
  echo "|---|---|"
  cat "$RESULTS_DIR/rejections.md"
} | tee "$RESULTS_DIR/summary.md"
jq -R -s 'split("\n") | map(select(length > 0) | split("\t") | {(.[0]): .[1]}) | add' "$RESULTS_TSV" >"$RESULTS_DIR/results.json"
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then cat "$RESULTS_DIR/summary.md" >>"$GITHUB_STEP_SUMMARY"; fi
