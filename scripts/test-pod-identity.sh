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
kubectl delete pod diag-api-test -n motionmesh --ignore-not-found
kubectl run diag-api-test --image=amazon/aws-cli -n motionmesh --overrides='{"spec": {"serviceAccountName": "motionmesh-api"}}' --restart=Never --command -- sleep 300
kubectl wait --for=condition=Ready pod/diag-api-test -n motionmesh --timeout=60s

echo "API Pod Identity (STS):"
kubectl exec diag-api-test -n motionmesh -- aws sts get-caller-identity | grep "motionmesh-api-$ENVIRONMENT" || ((FAILURES++))
echo "API Pod Identity (S3):"
kubectl exec diag-api-test -n motionmesh -- aws s3api head-bucket --bucket $S3_BUCKET_ID || ((FAILURES++))

echo "--- Testing Worker Pod Identity ---"
kubectl delete pod diag-worker-test -n motionmesh --ignore-not-found 2>/dev/null
kubectl run diag-worker-test --image=amazon/aws-cli -n motionmesh --overrides='{"spec": {"serviceAccountName": "motionmesh-worker"}}' --restart=Never --command -- sleep 300
kubectl wait --for=condition=Ready pod/diag-worker-test -n motionmesh --timeout=60s || ((FAILURES++))

echo "Worker Pod Identity (STS):"
kubectl exec diag-worker-test -n motionmesh -- aws sts get-caller-identity | grep "motionmesh-worker-$ENVIRONMENT" || ((FAILURES++))
echo "Worker Pod Identity (S3 Put):"
kubectl exec diag-worker-test -n motionmesh -- aws s3api put-object --bucket $S3_BUCKET_ID --key benchmark-test/pod-identity-test.txt --body /dev/null || ((FAILURES++))
echo "Worker Pod Identity (S3 Delete):"
kubectl exec diag-worker-test -n motionmesh -- aws s3api delete-object --bucket $S3_BUCKET_ID --key benchmark-test/pod-identity-test.txt || ((FAILURES++))

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
kubectl delete pod diag-api-test diag-worker-test -n motionmesh --ignore-not-found 2>/dev/null || true
kubectl delete pod diag-eso-test -n external-secrets --ignore-not-found 2>/dev/null || true
kubectl delete pod diag-dns-test -n kube-system --ignore-not-found 2>/dev/null || true

if [ $FAILURES -gt 0 ]; then
    echo "❌ Pod Identity Verification FAILED with $FAILURES errors."
    exit 1
else
    echo "✅ Pod Identity Verification Complete: All successful."
    exit 0
fi
