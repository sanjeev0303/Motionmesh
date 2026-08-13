#!/usr/bin/env bash

set -euo pipefail

ENVIRONMENT=${1:-benchmark}
echo "Deploying to AWS Environment: $ENVIRONMENT"

cd infra/terraform/envs/$ENVIRONMENT

echo "1. terraform init"
terraform init -upgrade

echo "1a. terraform validate"
terraform validate

echo "1b. terraform plan"
terraform plan -out=tfplan

echo "2. terraform apply (Foundation)"
terraform apply -auto-approve tfplan

echo "3. get outputs"
export S3_BUCKET_ID=$(terraform output -raw bucket_id)
export S3_BUCKET_REGION=$(terraform output -raw bucket_region)
export CLOUDFRONT_DISTRIBUTION_DOMAIN=$(terraform output -raw cloudfront_domain_name)
export API_IMAGE_URI=$(terraform output -raw api_repository_url)
export WORKER_IMAGE_URI=$(terraform output -raw worker_repository_url)
export AWS_REGION=$(terraform output -raw region)
export WAF_ACL_ARN=$(terraform output -raw web_acl_arn)
export ACM_CERTIFICATE_ARN=$(terraform output -raw acm_certificate_arn 2>/dev/null || echo "MISSING")
export EKS_CLUSTER_NAME=$(terraform output -raw cluster_name)
export VPC_ID=$(terraform output -raw vpc_id)
export DB_SECRET_ARN=$(terraform output -raw aurora_master_secret_arn)
export AURORA_ENDPOINT=$(terraform output -raw aurora_endpoint)
export ALB_SG_ID=$(terraform output -raw alb_security_group_id)
export LBC_ROLE_ARN=$(terraform output -raw lbc_role_arn)
export EXTERNAL_DNS_ROLE_ARN=$(terraform output -raw external_dns_role_arn)

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

echo "4. configure kubeconfig"
aws eks update-kubeconfig --region $AWS_REGION --name $EKS_CLUSTER_NAME

echo "5. verify EKS"
kubectl get nodes

echo "5a. Verify EKS Pod Identity Agent Health"
kubectl rollout status daemonset/eks-pod-identity-agent -n kube-system --timeout=120s

cd ../../../../

echo "6. apply namespace"
kubectl apply -f infra/k8s/namespace.yaml

echo "7. cluster addons (Helm)"
helm repo add eks https://aws.github.io/eks-charts || true
helm repo add external-secrets https://charts.external-secrets.io || true
helm repo add bitnami https://charts.bitnami.com/bitnami || true
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/ || true
helm repo update

echo "7a. Install AWS Load Balancer Controller"
helm upgrade --install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=$EKS_CLUSTER_NAME \
  --set region=$AWS_REGION \
  --set vpcId=$VPC_ID \
  --set serviceAccount.create=true \
  --set serviceAccount.name=aws-load-balancer-controller \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=$LBC_ROLE_ARN \
  --wait

echo "7b. Install External Secrets Operator"
kubectl create namespace external-secrets --dry-run=client -o yaml | kubectl apply -f -
kubectl create sa external-secrets -n external-secrets --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install external-secrets external-secrets/external-secrets \
    -n external-secrets \
    --set serviceAccount.name=external-secrets \
    --set serviceAccount.create=false \
    --wait

echo "7c. Install ExternalDNS"
# Create the service account to match Pod Identity association
kubectl create sa external-dns -n kube-system --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install external-dns bitnami/external-dns \
  -n kube-system \
  --set provider=aws \
  --set aws.region=$AWS_REGION \
  --set aws.zoneType=public \
  --set txtOwnerId=$EKS_CLUSTER_NAME \
  --set domainFilters[0]=motionmesh.com \
  --set policy=sync \
  --set serviceAccount.create=false \
  --set serviceAccount.name=external-dns \
  --wait

echo "7d. Install Metrics Server"
helm upgrade --install metrics-server metrics-server/metrics-server \
  -n kube-system \
  --set apiService.create=true \
  --wait

echo "8. Apply External Secrets Config"
envsubst < infra/k8s/external-secrets.yaml | kubectl apply -f -

if [ "$ENVIRONMENT" == "production" ]; then
    echo "8b. Apply Billing Secrets (Production only)"
    envsubst < infra/k8s/billing-secrets.yaml | kubectl apply -f -
fi

echo "9. wait for External Secrets"
kubectl wait --for=condition=Ready externalsecret/motionmesh-secrets -n motionmesh --timeout=120s

echo "10. ConfigMap"
envsubst < infra/k8s/configmap.yaml | kubectl apply -f -

echo "11. apply NATS"
kubectl apply -f infra/k8s/nats-cluster.yaml

echo "12. wait for NATS"
kubectl rollout status statefulset/nats -n motionmesh --timeout=120s

echo "13. push Git SHA images"
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $(echo $API_IMAGE_URI | cut -d'/' -f1)

# API
docker build -t motionmesh-api -f server/api/Dockerfile .
docker tag motionmesh-api $API_IMAGE_URI:$GIT_SHA
docker push $API_IMAGE_URI:$GIT_SHA

# Worker
docker build -t motionmesh-worker -f server/worker/Dockerfile .
docker tag motionmesh-worker $WORKER_IMAGE_URI:$GIT_SHA
docker push $WORKER_IMAGE_URI:$GIT_SHA

export MIGRATION_IMAGE_URI="$API_IMAGE_URI:$GIT_SHA"

echo "14. run migration"
kubectl delete job motionmesh-db-migration -n motionmesh --ignore-not-found
envsubst < infra/k8s/db-migration-job.yaml | kubectl apply -f -

echo "15. wait for migration SUCCESS"
kubectl wait --for=condition=complete job/motionmesh-db-migration -n motionmesh --timeout=300s

echo "16. deploy API"
export API_IMAGE_URI="$API_IMAGE_URI:$GIT_SHA"
envsubst < infra/k8s/api.yaml | kubectl apply -f -

echo "17. deploy Worker"
export WORKER_IMAGE_URI="$WORKER_IMAGE_URI:$GIT_SHA"
envsubst < infra/k8s/worker.yaml | kubectl apply -f -

echo "18. deploy Ingress"
envsubst < infra/k8s/ingress.yaml | kubectl apply -f -

echo "19. wait for readiness"
kubectl rollout status deployment/api -n motionmesh --timeout=300s
kubectl rollout status deployment/worker -n motionmesh --timeout=300s

echo "20. run smoke test"
./scripts/smoke-test-aws.sh $ENVIRONMENT

echo "21. run AWS wiring verification"
./scripts/verify-aws-wiring.sh $ENVIRONMENT

echo "22. DEPLOYMENT READY"
