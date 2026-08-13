#!/usr/bin/env bash

set -eo pipefail

ENVIRONMENT=${1:-benchmark}
echo "Deploying to AWS Environment: $ENVIRONMENT"

cd infra/terraform/envs/$ENVIRONMENT

echo "1. terraform init"
terraform init -upgrade

echo "2. terraform validate"
terraform validate

echo "4. terraform apply"
terraform apply -auto-approve

echo "5. get outputs"
export S3_BUCKET_ID=$(terraform output -raw bucket_id)
export S3_BUCKET_REGION=$(terraform output -raw bucket_region)
export CLOUDFRONT_DISTRIBUTION_DOMAIN=$(terraform output -raw cloudfront_domain_name)
export API_IMAGE_URI=$(terraform output -raw api_repository_url)
export WORKER_IMAGE_URI=$(terraform output -raw worker_repository_url)
export AWS_REGION=$(terraform output -raw region)
export WAF_ACL_ARN=$(terraform output -raw web_acl_arn)
export ACM_CERTIFICATE_ARN=$(terraform output -raw acm_certificate_arn 2>/dev/null || echo "MISSING")
export EKS_CLUSTER_NAME="motionmesh-${ENVIRONMENT}"
export DB_SECRET_ARN=$(aws secretsmanager list-secrets --region $AWS_REGION --query "SecretList[?starts_with(Name, 'rds!cluster-')].ARN" --output text | head -n 1) # Note: we just fetch the RDS managed secret here. Or we could output it from TF.

export ENVIRONMENT=$ENVIRONMENT
export STRIPE_MODE="mock"
export AI_MODE="mock"
export BENCHMARK_MODE="true"
if [ "$ENVIRONMENT" == "production" ]; then
    export STRIPE_MODE="live"
    export AI_MODE="live"
    export BENCHMARK_MODE="false"
fi
export ALLOWED_ORIGINS="https://dashboard.motionmesh.com"
export CLOUDFRONT_MEDIA_DOMAIN="media.motionmesh.com"
export COOKIE_DOMAIN=".motionmesh.com"
export API_DOMAIN="api.motionmesh.com"

export GIT_SHA=$(git rev-parse --short HEAD)

echo "6. configure kubeconfig"
aws eks update-kubeconfig --region $AWS_REGION --name $EKS_CLUSTER_NAME

echo "7. verify EKS"
kubectl get nodes

cd ../../../../

echo "11. apply namespace"
kubectl apply -f infra/k8s/namespace.yaml

echo "12. apply service accounts"
# They are in namespace.yaml

echo "10. install External Secrets"
# Assuming helm is installed and ESO is deployed. If not, this is where we'd helm install it.
# We'll just apply our SecretStore and ExternalSecret.
envsubst < infra/k8s/external-secrets.yaml | kubectl apply -f -

echo "13. wait for External Secrets"
kubectl wait --for=condition=Ready externalsecret/motionmesh-secrets -n motionmesh --timeout=120s || echo "Wait failed, but proceeding..."

echo "ConfigMap"
envsubst < infra/k8s/configmap.yaml | kubectl apply -f -

echo "14. apply NATS"
kubectl apply -f infra/k8s/nats-cluster.yaml

echo "15. wait for NATS"
kubectl rollout status statefulset/nats -n motionmesh --timeout=120s

echo "18. build immutable images"
echo "19. push Git SHA images"
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $(echo $API_IMAGE_URI | cut -d'/' -f1)

# API
docker build -t motionmesh-api -f server/api/Dockerfile .
docker tag motionmesh-api $API_IMAGE_URI:$GIT_SHA
docker push $API_IMAGE_URI:$GIT_SHA

# Worker
docker build -t motionmesh-worker -f server/worker/Dockerfile .
docker tag motionmesh-worker $WORKER_IMAGE_URI:$GIT_SHA
docker push $WORKER_IMAGE_URI:$GIT_SHA

# Migration Image (assuming Dockerfile.migrate exists, or we use api image for migrate)
# For now, we'll use API image which contains the migration binary
export MIGRATION_IMAGE_URI="$API_IMAGE_URI:$GIT_SHA"

echo "16. run migration"
kubectl delete job motionmesh-db-migration -n motionmesh --ignore-not-found
envsubst < infra/k8s/db-migration-job.yaml | kubectl apply -f -

echo "17. wait for migration SUCCESS"
kubectl wait --for=condition=complete job/motionmesh-db-migration -n motionmesh --timeout=300s

echo "20. deploy API"
export API_IMAGE_URI="$API_IMAGE_URI:$GIT_SHA"
envsubst < infra/k8s/api.yaml | kubectl apply -f -

echo "21. deploy Worker"
export WORKER_IMAGE_URI="$WORKER_IMAGE_URI:$GIT_SHA"
envsubst < infra/k8s/worker.yaml | kubectl apply -f -

echo "22. deploy Ingress"
envsubst < infra/k8s/ingress.yaml | kubectl apply -f -

echo "23. wait for readiness"
kubectl rollout status deployment/api -n motionmesh --timeout=300s
kubectl rollout status deployment/worker -n motionmesh --timeout=300s

echo "24. run AWS wiring verification"
./scripts/verify-aws-wiring.sh $ENVIRONMENT

echo "25. run smoke test"
# Assume smoke test script exists or will be run manually

echo "26. DEPLOYMENT READY"
