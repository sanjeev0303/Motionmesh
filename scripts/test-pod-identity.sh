#!/usr/bin/env bash

set -euo pipefail

FAILURES=0

function check_success {
    if [ $? -eq 0 ]; then
        echo "✅ SUCCESS"
    else
        echo "❌ FAILED"
        ((FAILURES++))
    fi
}

ENVIRONMENT=${1:-benchmark}

cd infra/terraform/envs/$ENVIRONMENT
AWS_REGION=$(terraform output -raw region)
S3_BUCKET_ID=$(terraform output -raw bucket_id)
cd ../../../../

echo "=== Testing Pod Identity for $ENVIRONMENT ==="

API_POD=$(kubectl get pods -n motionmesh -l app=api -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
WORKER_POD=$(kubectl get pods -n motionmesh -l app=worker -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [[ -z "$API_POD" || -z "$WORKER_POD" ]]; then
    echo "❌ API or Worker pod not found. Please ensure they are deployed."
    exit 1
fi

echo "--- Testing API Pod Identity ---"
echo "Asserting token file presence:"
kubectl exec $API_POD -n motionmesh -- env | grep AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE || ((FAILURES++))

echo "Running Application-Level Diagnostics in API Pod:"
kubectl exec $API_POD -n motionmesh -- /app/diagnostic || ((FAILURES++))

echo "--- Testing Worker Pod Identity ---"
echo "Asserting token file presence:"
kubectl exec $WORKER_POD -n motionmesh -- env | grep AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE || ((FAILURES++))

echo "Running Application-Level Diagnostics in Worker Pod:"
kubectl exec $WORKER_POD -n motionmesh -- /app/diagnostic || ((FAILURES++))

echo "--- Testing ESO Pod Identity ---"
kubectl delete pod diag-eso-test -n external-secrets --ignore-not-found 2>/dev/null
kubectl run diag-eso-test --image=amazon/aws-cli -n external-secrets --overrides='{"spec": {"serviceAccountName": "external-secrets"}}' --restart=Never --command -- sleep 300
kubectl wait --for=condition=Ready pod/diag-eso-test -n external-secrets --timeout=60s || ((FAILURES++))
echo "ESO Pod Identity (STS):"
kubectl exec diag-eso-test -n external-secrets -- aws sts get-caller-identity | grep "motionmesh-external-secrets-$ENVIRONMENT" || ((FAILURES++))

echo "--- Testing ExternalDNS Pod Identity ---"
kubectl delete pod diag-dns-test -n kube-system --ignore-not-found 2>/dev/null
kubectl run diag-dns-test --image=amazon/aws-cli -n kube-system --overrides='{"spec": {"serviceAccountName": "external-dns"}}' --restart=Never --command -- sleep 300
kubectl wait --for=condition=Ready pod/diag-dns-test -n kube-system --timeout=60s || ((FAILURES++))
echo "ExternalDNS Pod Identity (STS):"
kubectl exec diag-dns-test -n kube-system -- aws sts get-caller-identity | grep "motionmesh-external-dns-$ENVIRONMENT" || ((FAILURES++))

# Cleanup
# Cleanup
kubectl delete pod diag-eso-test -n external-secrets --ignore-not-found 2>/dev/null || true
kubectl delete pod diag-dns-test -n kube-system --ignore-not-found 2>/dev/null || true

if [ $FAILURES -gt 0 ]; then
    echo "❌ Pod Identity Verification FAILED with $FAILURES errors."
    exit 1
else
    echo "✅ Pod Identity Verification Complete: All successful."
    exit 0
fi
