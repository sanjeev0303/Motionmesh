#!/usr/bin/env bash
set -euo pipefail

ENVIRONMENT=${1:-benchmark}
echo "=== Running Smoke Tests for $ENVIRONMENT ==="

API_DOMAIN="api.motionmesh.com"
if [ "$ENVIRONMENT" == "benchmark" ]; then
    API_DOMAIN="api.motionmesh.com" # Assume this is aliased to ALB in Route53
fi

# For smoke test, we check if API resolves or if we can hit it
# Assuming local /etc/hosts or actual DNS is setup. If not, hit ALB directly.
cd infra/terraform/envs/$ENVIRONMENT
ALB_HOST=$(aws elbv2 describe-load-balancers --region us-east-1 --query "LoadBalancers[0].DNSName" --output text)

echo "Testing API Health Endpoint..."
curl -s --fail http://$ALB_HOST/health || (echo "❌ API Health failed" && exit 1)
echo "✅ API Health Passed"

echo "Testing API Ready Endpoint..."
curl -s --fail http://$ALB_HOST/ready || (echo "❌ API Ready failed" && exit 1)
echo "✅ API Ready Passed"

# Add more mock tests or placeholder for real operations if we had a test account
echo "Smoke Tests Complete!"
