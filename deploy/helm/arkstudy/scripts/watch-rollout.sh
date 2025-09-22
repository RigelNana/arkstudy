#!/usr/bin/env bash
set -euo pipefail

NS="${1:-arkstudy}"
TIMEOUT="${TIMEOUT:-10m}"

echo "[watch] Namespace: ${NS}  Timeout per resource: ${TIMEOUT}"
echo "[watch] Listing targeted resources (Deployments/StatefulSets)"
mapfile -t RES <<<"$(kubectl get deploy,statefulset -n "${NS}" -o name)"

if [[ ${#RES[@]} -eq 0 ]]; then
  echo "[watch] No Deployments or StatefulSets found in namespace ${NS}."
  exit 0
fi

for r in "${RES[@]}"; do
  echo "---- ${r}"
  # Print a short summary first
  kubectl get -n "${NS}" "${r}" -o=custom-columns=KIND:.kind,NAME:.metadata.name,GEN:.metadata.generation --no-headers || true
  # Rollout status
  if ! kubectl rollout status -n "${NS}" "${r}" --timeout="${TIMEOUT}"; then
    echo "[watch][warn] ${r} rollout did not complete in ${TIMEOUT}. Dumping describe and recent events:"
    kubectl describe -n "${NS}" "${r}" || true
    echo "[watch][events] Recent events (last 50):"
    kubectl get events -n "${NS}" --sort-by=.lastTimestamp | tail -n 50 || true
  fi
  echo ""
done

echo "[watch] Pods summary:"
kubectl get pods -n "${NS}" -o wide

echo "[watch] If something is still progressing, you can also run in another terminal:"
echo "  kubectl get pods -n ${NS} -w"
