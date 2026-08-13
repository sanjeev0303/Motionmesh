#!/usr/bin/env bash

set -euo pipefail

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
kubectl exec diag-api-test -n motionmesh -- aws sts get-caller-identity
echo "API Pod Identity (S3):"
kubectl exec diag-api-test -n motionmesh -- aws s3api head-bucket --bucket $S3_BUCKET_ID

echo "--- Testing Worker Pod Identity ---"
kubectl delete pod diag-worker-test -n motionmesh --ignore-not-found
kubectl run diag-worker-test --image=amazon/aws-cli -n motionmesh --overrides='{"spec": {"serviceAccountName": "motionmesh-worker"}}' --restart=Never --command -- sleep 300
kubectl wait --for=condition=Ready pod/diag-worker-test -n motionmesh --timeout=60s

echo "Worker Pod Identity (STS):"
kubectl exec diag-worker-test -n motionmesh -- aws sts get-caller-identity
echo "Worker Pod Identity (S3):"
kubectl exec diag-worker-test -n motionmesh -- aws s3api put-object --bucket $S3_BUCKET_ID --key benchmark-test/pod-identity-test.txt --body /dev/null
kubectl exec diag-worker-test -n motionmesh -- aws s3api delete-object --bucket $S3_BUCKET_ID --key benchmark-test/pod-identity-test.txt

kubectl delete pod diag-api-test diag-worker-test -n motionmesh --ignore-not-found

echo "=== Pod Identity Verification Complete ==="
