#!/usr/bin/env bash
# @cpt:impl cpt-katapult-flow-local-dev-env-seed
# Seeds demo data into the local Katapult dev environment.
set -euo pipefail

API_BASE="${API_BASE:-http://localhost:30080}"
API_TOKEN="${API_TOKEN:-test-operator-token}"
NAMESPACE="katapult-system"

log() { echo "==> $*"; }

# --- Wait for all workloads to be ready ---
log "Waiting for deployments and daemonsets to be ready..."
kubectl rollout status deployment/katapult-controlplane -n "$NAMESPACE" --timeout=120s
kubectl rollout status deployment/katapult-web -n "$NAMESPACE" --timeout=120s
kubectl rollout status deployment/postgres -n "$NAMESPACE" --timeout=120s
kubectl rollout status deployment/minio -n "$NAMESPACE" --timeout=120s
kubectl rollout status daemonset/katapult-agent -n "$NAMESPACE" --timeout=120s

# --- Wait for agents to register ---
log "Waiting for agents to register with control plane..."
for i in $(seq 1 60); do
  AGENTS=$(curl -sf -H "Authorization: Bearer ${API_TOKEN}" \
    "${API_BASE}/api/v1alpha1/agents" 2>/dev/null || echo '{"items":[]}')
  COUNT=$(echo "$AGENTS" | grep -o '"id"' | wc -l | tr -d ' ')
  if [ "$COUNT" -ge 1 ]; then
    log "Found $COUNT registered agent(s)"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: No agents registered after 60s"
    exit 1
  fi
  sleep 1
done

# --- Get worker node names ---
WORKERS=($(kubectl get nodes --no-headers -l '!node-role.kubernetes.io/control-plane' -o custom-columns=':metadata.name'))
if [ "${#WORKERS[@]}" -lt 2 ]; then
  echo "ERROR: Need at least 2 worker nodes, found ${#WORKERS[@]}"
  exit 1
fi
WORKER1="${WORKERS[0]}"
WORKER2="${WORKERS[1]}"
log "Using worker nodes: $WORKER1 (source), $WORKER2 (destination)"

# --- Create PVCs bound to specific worker nodes ---
log "Creating demo PVCs..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: demo-src-data
  namespace: $NAMESPACE
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 64Mi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: demo-dst-data
  namespace: $NAMESPACE
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: local-path
  resources:
    requests:
      storage: 128Mi
EOF

# --- Bind PVCs to specific nodes using busybox pods ---
log "Binding PVCs to worker nodes via busybox pods..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: pvc-binder-src
  namespace: $NAMESPACE
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: $WORKER1
  containers:
    - name: binder
      image: busybox:latest
      command: ["/bin/sh", "-c", "echo bound && sleep 5"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: demo-src-data
---
apiVersion: v1
kind: Pod
metadata:
  name: pvc-binder-dst
  namespace: $NAMESPACE
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: $WORKER2
  containers:
    - name: binder
      image: busybox:latest
      command: ["/bin/sh", "-c", "echo bound && sleep 5"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: demo-dst-data
EOF

log "Waiting for binder pods to complete..."
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/pvc-binder-src -n "$NAMESPACE" --timeout=120s
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/pvc-binder-dst -n "$NAMESPACE" --timeout=120s

# --- Populate source PVC with sample files ---
log "Populating source PVC with sample data..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: pvc-writer
  namespace: $NAMESPACE
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: $WORKER1
  containers:
    - name: writer
      image: busybox:latest
      command:
        - /bin/sh
        - -c
        - |
          mkdir -p /data/documents /data/images /data/logs
          echo "Hello from Katapult local dev!" > /data/README.txt
          dd if=/dev/urandom of=/data/documents/report.bin bs=1024 count=32 2>/dev/null
          dd if=/dev/urandom of=/data/images/photo.bin bs=1024 count=16 2>/dev/null
          dd if=/dev/urandom of=/data/logs/app.log bs=1024 count=8 2>/dev/null
          echo "Sample config content" > /data/documents/config.txt
          echo "done"
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: demo-src-data
EOF

kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/pvc-writer -n "$NAMESPACE" --timeout=120s
log "Source PVC populated"

# --- Clean up helper pods ---
log "Cleaning up helper pods..."
kubectl delete pod pvc-binder-src pvc-binder-dst pvc-writer -n "$NAMESPACE" --ignore-not-found

# --- Wait for agents to discover PVCs ---
log "Waiting for agents to discover PVCs..."
for i in $(seq 1 30); do
  AGENTS=$(curl -sf -H "Authorization: Bearer ${API_TOKEN}" \
    "${API_BASE}/api/v1alpha1/agents" 2>/dev/null || echo '{}')
  if echo "$AGENTS" | grep -q "demo-src-data"; then
    log "Agents discovered PVCs"
    break
  fi
  if [ "$i" -eq 30 ]; then
    log "Warning: agents may not have discovered PVCs yet (will happen on next heartbeat)"
    break
  fi
  sleep 2
done

# --- Create a sample transfer ---
log "Creating sample transfer..."
TRANSFER_RESP=$(curl -sf -X POST \
  -H "Authorization: Bearer ${API_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "source_cluster": "kind",
    "source_pvc": "demo-src-data",
    "destination_cluster": "kind",
    "destination_pvc": "demo-dst-data"
  }' \
  "${API_BASE}/api/v1alpha1/transfers" 2>/dev/null || echo "")

if [ -n "$TRANSFER_RESP" ]; then
  log "Sample transfer created"
else
  log "Warning: could not create sample transfer (API may not support it yet)"
fi

log "Seed data complete!"
