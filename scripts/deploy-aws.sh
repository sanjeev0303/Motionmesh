#!/usr/bin/env bash

set -eo pipefail

ENVIRONMENT=${1:-benchmark}

echo "Deploying to AWS Environment: $ENVIRONMENT"

cd infra/terraform/envs/$ENVIRONMENT
echo "Applying Terraform (make sure you passed TF_VAR_cloudfront_signing_private_key if required)"
terraform apply -auto-approve

echo "Extracting Terraform outputs..."
export S3_BUCKET_ID=$(terraform output -raw bucket_id)
export S3_BUCKET_REGION=$(terraform output -raw bucket_region)
export CLOUDFRONT_DISTRIBUTION_DOMAIN=$(terraform output -raw cloudfront_domain_name)
export API_IMAGE_URI=$(terraform output -raw api_repository_url):latest
export WORKER_IMAGE_URI=$(terraform output -raw worker_repository_url):latest
export AWS_REGION=$(terraform output -raw region)
export WAF_ACL_ARN=$(terraform output -raw web_acl_arn)
export ACM_CERTIFICATE_ARN=$(terraform output -raw acm_certificate_arn 2>/dev/null || echo "MISSING")
# Need to set these for substitution:
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

cd ../../../../

echo "Building and pushing Docker images..."
# API
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $(echo $API_IMAGE_URI | cut -d'/' -f1)
docker build -t motionmesh-api -f server/api/Dockerfile .
docker tag motionmesh-api $API_IMAGE_URI
docker push $API_IMAGE_URI

# Worker
docker build -t motionmesh-worker -f server/worker/Dockerfile .
docker tag motionmesh-worker $WORKER_IMAGE_URI
docker push $WORKER_IMAGE_URI

echo "Applying Kubernetes manifests..."
# Namespace & Service Accounts
kubectl apply -f infra/k8s/namespace.yaml

# External Secrets
envsubst < infra/k8s/external-secrets.yaml | kubectl apply -f -

# ConfigMap
envsubst < infra/k8s/configmap.yaml | kubectl apply -f -

# StatefulSet (NATS)
kubectl apply -f infra/k8s/nats-cluster.yaml

# Deployments
envsubst < infra/k8s/api.yaml | kubectl apply -f -
envsubst < infra/k8s/worker.yaml | kubectl apply -f -

# Migration Job
kubectl delete job motionmesh-db-migration -n motionmesh --ignore-not-found
envsubst < infra/k8s/db-migration-job.yaml | kubectl apply -f -

# Ingress
envsubst < infra/k8s/ingress.yaml | kubectl apply -f -

echo "Waiting for rollout..."
kubectl rollout status deployment/api -n motionmesh
kubectl rollout status deployment/worker -n motionmesh
kubectl wait --for=condition=complete job/motionmesh-db-migration -n motionmesh --timeout=120s

echo "Deployment complete."
