#!/usr/bin/env bash

set -euo pipefail

ENVIRONMENT=${1:-benchmark}

echo "=== Verifying AWS Infrastructure Wiring for $ENVIRONMENT ==="

cd infra/terraform/envs/$ENVIRONMENT
AWS_REGION=$(terraform output -raw region)

echo "[1/6] Checking ECR Repositories..."
aws ecr describe-repositories --repository-names motionmesh-api --region $AWS_REGION >/dev/null && echo "✅ API Repo exists" || echo "❌ API Repo missing"
aws ecr describe-repositories --repository-names motionmesh-worker --region $AWS_REGION >/dev/null && echo "✅ Worker Repo exists" || echo "❌ Worker Repo missing"

echo "[2/6] Checking Secrets Manager..."
for secret in database redis cloudfront-signing clerk stripe; do
    aws secretsmanager describe-secret --secret-id motionmesh/$ENVIRONMENT/$secret --region $AWS_REGION >/dev/null 2>&1 && echo "✅ Secret motionmesh/$ENVIRONMENT/$secret exists" || echo "❌ Secret motionmesh/$ENVIRONMENT/$secret missing"
done

echo "[3/6] Checking S3 Bucket Security..."
BUCKET_ID=$(terraform output -raw bucket_id)
PUBLIC_ACCESS=$(aws s3api get-public-access-block --bucket $BUCKET_ID --region $AWS_REGION --query 'PublicAccessBlockConfiguration' 2>/dev/null || echo "")
if [[ "$PUBLIC_ACCESS" == *'"BlockPublicAcls": true'* ]] && [[ "$PUBLIC_ACCESS" == *'"BlockPublicPolicy": true'* ]]; then
    echo "✅ S3 Block Public Access is fully enabled"
else
    echo "❌ S3 Block Public Access is NOT fully enabled: $PUBLIC_ACCESS"
fi

CORS=$(aws s3api get-bucket-cors --bucket $BUCKET_ID --region $AWS_REGION 2>/dev/null || echo "None")
if [[ "$CORS" != "None" ]]; then
    echo "✅ S3 CORS configuration is active"
else
    echo "❌ S3 CORS configuration is missing"
fi

echo "[4/6] Checking Kubernetes External Secrets..."
kubectl get secretstore aws-secretsmanager -n motionmesh >/dev/null 2>&1 && echo "✅ SecretStore exists" || echo "❌ SecretStore missing"
kubectl get externalsecret motionmesh-secrets -n motionmesh -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' | grep "True" >/dev/null && echo "✅ ExternalSecret motionmesh-secrets is Synced" || echo "❌ ExternalSecret motionmesh-secrets is NOT Synced"
kubectl get secret motionmesh-secrets -n motionmesh >/dev/null 2>&1 && echo "✅ K8s Secret motionmesh-secrets was successfully created" || echo "❌ K8s Secret motionmesh-secrets missing"

echo "[5/6] Checking Pod Identity IAM Assumes..."
API_ROLE=$(kubectl get sa motionmesh-api -n motionmesh -o jsonpath='{.metadata.annotations.eks\.amazonaws\.com/role-arn}' 2>/dev/null || echo "")
if [[ -n "$API_ROLE" ]]; then
    echo "✅ API ServiceAccount has Pod Identity mapping annotation: $API_ROLE"
else
    echo "⚠️  API ServiceAccount lacks role annotation (this is fine if using EKS Pod Identity Association directly in cluster)"
fi

echo "[6/6] Checking AWS Load Balancer Controller (WAF/ALB)..."
# Check if ingress has an IP address
ALB_HOST=$(kubectl get ingress motionmesh-api-ingress -n motionmesh -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")
if [[ -n "$ALB_HOST" ]]; then
    echo "✅ ALB successfully provisioned by LBC: $ALB_HOST"
else
    echo "❌ ALB not provisioned yet (check 'kubectl logs -n kube-system -l app.kubernetes.io/name=aws-load-balancer-controller')"
fi

echo "=== Verification Complete ==="
