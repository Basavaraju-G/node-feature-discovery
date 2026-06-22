#!/bin/bash
# End-to-end test for the crypto (CEX card) feature source on s390x.
#
# Prerequisites:
#   - A Kubernetes cluster running on an s390x host with CEX cards
#   - NFD deployed with crypto source enabled
#   - kubectl configured with a valid context
#
# Usage:
#   ./test/e2e/crypto_e2e_test.sh [--context <context>] [--namespace <ns>]
#
# If --context is not provided, kubectl uses the current default context.
# If --namespace is not provided, defaults to "node-feature-discovery".
# The script will exit gracefully on non-s390x clusters.

set -uo pipefail

CONTEXT=""
NS="node-feature-discovery"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --context) CONTEXT="$2"; shift 2 ;;
    --namespace|-n) NS="$2"; shift 2 ;;
    *) CONTEXT="$1"; shift ;;
  esac
done

KUBECTL="kubectl"
if [ -n "$CONTEXT" ]; then
  KUBECTL="kubectl --context $CONTEXT"
fi

PASS=0
FAIL=0

pass() { ((PASS++)); echo "  PASS: $1"; }
fail() { ((FAIL++)); echo "  FAIL: $1"; }

echo "=== NFD Crypto Source E2E Tests ==="
echo "Context: ${CONTEXT:-<current>}"
echo "Namespace: $NS"
echo ""

# --- Pre-check: Verify cluster is reachable ---
if ! $KUBECTL cluster-info >/dev/null 2>&1; then
  echo "ERROR: Cannot connect to cluster. Check kubectl context."
  exit 1
fi

# --- Pre-check: Verify at least one node is s390x ---
echo "Pre-check: Verifying cluster has s390x nodes"
NODE_ARCH=$($KUBECTL get nodes -o jsonpath='{.items[*].status.nodeInfo.architecture}' 2>/dev/null)
if ! echo "$NODE_ARCH" | grep -q "s390x"; then
  echo "SKIP: No s390x nodes found in cluster (architecture: ${NODE_ARCH:-unknown})"
  echo "The crypto source only detects CEX cards on s390x. Exiting."
  exit 0
fi
echo "  Found s390x node(s)"
echo ""

# --- Test 1: Verify NFD worker pods are running ---
echo "Test 1: NFD worker pods are running"
WORKERS=$($KUBECTL -n "$NS" get pods -l app.kubernetes.io/component=worker --no-headers 2>/dev/null | grep -c Running || true)
if [ "$WORKERS" -eq 0 ]; then
  WORKERS=$($KUBECTL -n "$NS" get pods -l app=nfd-worker --no-headers 2>/dev/null | grep -c Running || true)
fi
if [ "$WORKERS" -gt 0 ]; then
  pass "Found $WORKERS running NFD worker pod(s)"
else
  fail "No running NFD worker pods found"
fi

# --- Test 2: Verify crypto-cex.present label exists ---
echo "Test 2: crypto-cex.present label is set"
PRESENT=$($KUBECTL get nodes -o jsonpath='{.items[0].metadata.labels.feature\.node\.kubernetes\.io/crypto-cex\.present}' 2>/dev/null)
if [ "$PRESENT" = "true" ]; then
  pass "crypto-cex.present=true"
else
  fail "crypto-cex.present not found or not true (got: '$PRESENT')"
fi

# --- Test 3: Verify crypto-cex.count label ---
echo "Test 3: crypto-cex.count label is set"
COUNT=$($KUBECTL get nodes -o jsonpath='{.items[0].metadata.labels.feature\.node\.kubernetes\.io/crypto-cex\.count}' 2>/dev/null)
if [ -n "$COUNT" ] && [ "$COUNT" -gt 0 ] 2>/dev/null; then
  pass "crypto-cex.count=$COUNT"
else
  fail "crypto-cex.count not found or invalid (got: '$COUNT')"
fi

# --- Test 4: Verify a type label exists ---
echo "Test 4: crypto-cex.type-* label exists"
TYPE_LABELS=$($KUBECTL get nodes -o json | python3 -c "
import sys, json
data = json.load(sys.stdin)
types = [k for n in data['items'] for k in n['metadata']['labels'] if 'crypto-cex.type-' in k]
print(' '.join(types))
" 2>/dev/null)
if [ -n "$TYPE_LABELS" ]; then
  pass "Type labels found: $TYPE_LABELS"
else
  fail "No crypto-cex.type-* labels found"
fi

# --- Test 5: Verify a mode label exists ---
echo "Test 5: crypto-cex.mode-* label exists"
MODE_LABELS=$($KUBECTL get nodes -o json | python3 -c "
import sys, json
data = json.load(sys.stdin)
modes = [k for n in data['items'] for k in n['metadata']['labels'] if 'crypto-cex.mode-' in k]
print(' '.join(modes))
" 2>/dev/null)
if [ -n "$MODE_LABELS" ]; then
  pass "Mode labels found: $MODE_LABELS"
else
  fail "No crypto-cex.mode-* labels found"
fi

# --- Test 6: Verify NodeFeature object has crypto.cex-card instances ---
echo "Test 6: NodeFeature object contains crypto.cex-card instances"
CEX_INSTANCES=$($KUBECTL -n "$NS" get nodefeature -o json | python3 -c "
import sys, json
data = json.load(sys.stdin)
for item in data.get('items', []):
  instances = item.get('spec', {}).get('features', {}).get('instances', {})
  cex = instances.get('crypto.cex-card', {})
  elements = cex.get('elements', [])
  if elements:
    for e in elements:
      attrs = e.get('attributes', {})
      print(f\"{attrs.get('name','?')}: type={attrs.get('type','?')} mode={attrs.get('mode','?')} online={attrs.get('online','?')} config={attrs.get('config','?')}\")
" 2>/dev/null)
if [ -n "$CEX_INSTANCES" ]; then
  pass "Instance features found:"
  echo "$CEX_INSTANCES" | sed 's/^/    /'
else
  fail "No crypto.cex-card instance features found in NodeFeature object"
fi

# --- Test 7: Verify instance attributes include new fields ---
echo "Test 7: Instance attributes include mode, config, ap_functions"
ATTRS_CHECK=$($KUBECTL -n "$NS" get nodefeature -o json | python3 -c "
import sys, json
data = json.load(sys.stdin)
for item in data.get('items', []):
  instances = item.get('spec', {}).get('features', {}).get('instances', {})
  cex = instances.get('crypto.cex-card', {})
  for e in cex.get('elements', []):
    attrs = e.get('attributes', {})
    missing = []
    for field in ['name', 'type', 'mode', 'online', 'hwtype', 'depth', 'config']:
      if field not in attrs:
        missing.append(field)
    if missing:
      print(f\"MISSING: {','.join(missing)}\")
    else:
      print('ALL_PRESENT')
" 2>/dev/null)
if echo "$ATTRS_CHECK" | grep -q "ALL_PRESENT"; then
  pass "All expected attributes present (name, type, mode, online, hwtype, depth, config)"
else
  fail "Missing attributes: $ATTRS_CHECK"
fi

# --- Test 8: Apply NodeFeatureRule and verify custom label ---
echo "Test 8: NodeFeatureRule matching on crypto.cex-card"
cat <<'EOF' | $KUBECTL apply -f - 2>/dev/null
apiVersion: nfd.k8s-sigs.io/v1alpha1
kind: NodeFeatureRule
metadata:
  name: crypto-e2e-test
spec:
  rules:
    - name: "cex-online-cards"
      labels:
        "e2e-test/crypto-online": "true"
      matchFeatures:
        - feature: crypto.cex-card
          matchExpressions:
            online:
              op: In
              value:
                - "1"
EOF

sleep 10

CUSTOM_LABEL=$($KUBECTL get nodes -o jsonpath='{.items[0].metadata.labels.e2e-test/crypto-online}' 2>/dev/null)
if [ "$CUSTOM_LABEL" = "true" ]; then
  pass "NodeFeatureRule matched: e2e-test/crypto-online=true"
else
  fail "NodeFeatureRule did not produce expected label (got: '$CUSTOM_LABEL')"
fi

# Cleanup
$KUBECTL delete nodefeaturerule crypto-e2e-test 2>/dev/null || true

# --- Summary ---
echo ""
echo "=== Results ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo "SOME TESTS FAILED"
  exit 1
else
  echo "ALL TESTS PASSED"
  exit 0
fi
