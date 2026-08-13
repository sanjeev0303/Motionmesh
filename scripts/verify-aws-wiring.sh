#!/usr/bin/env bash

set -uo pipefail

FAILURES=0

function fail {
    echo "❌ $1"
    ((FAILURES++))
}

function pass {
    echo "✅ $1"
}

ENVIRONMENT=${1:-benchmark}

echo "=== Verifying AWS Infrastructure Wiring for $ENVIRONMENT ==="

cd infra/terraform/envs/$ENVIRONMENT
AWS_REGION=$(terraform output -raw region)

echo "[1/7] Checking ECR Repositories..."
aws ecr describe-repositories --repository-names motionmesh-api --region $AWS_REGION >/dev/null 2>&1 && pass "API Repo exists" || fail "API Repo missing"
aws ecr describe-repositories --repository-names motionmesh-worker --region $AWS_REGION >/dev/null 2>&1 && pass "Worker Repo exists" || fail "Worker Repo missing"

echo "[2/7] Checking Secrets Manager..."
for secret in redis cloudfront-signing clerk stripe; do
    aws secretsmanager describe-secret --secret-id motionmesh/$ENVIRONMENT/$secret --region $AWS_REGION >/dev/null 2>&1 && pass "Secret motionmesh/$ENVIRONMENT/$secret exists" || fail "Secret motionmesh/$ENVIRONMENT/$secret missing"
done

DB_SECRET_ARN=$(terraform output -raw aurora_master_secret_arn)
if [[ -n "$DB_SECRET_ARN" && "$DB_SECRET_ARN" != "None" ]]; then
    aws secretsmanager describe-secret --secret-id "$DB_SECRET_ARN" --region $AWS_REGION >/dev/null 2>&1 && pass "RDS managed secret exists: $DB_SECRET_ARN" || fail "RDS managed secret missing"
else
    fail "RDS managed secret missing in terraform output"
fi

echo "[3/7] Checking S3 Bucket Security..."
BUCKET_ID=$(terraform output -raw bucket_id)
PUBLIC_ACCESS=$(aws s3api get-public-access-block --bucket $BUCKET_ID --region $AWS_REGION --query 'PublicAccessBlockConfiguration' 2>/dev/null || echo "")
if [[ "$PUBLIC_ACCESS" == *'"BlockPublicAcls": true'* ]] && [[ "$PUBLIC_ACCESS" == *'"BlockPublicPolicy": true'* ]]; then
    pass "S3 Block Public Access is fully enabled"
else
    fail "S3 Block Public Access is NOT fully enabled: $PUBLIC_ACCESS"
fi

CORS=$(aws s3api get-bucket-cors --bucket $BUCKET_ID --region $AWS_REGION 2>/dev/null || echo "None")
if [[ "$CORS" != "None" ]]; then
    pass "S3 CORS configuration is active"
else
    fail "S3 CORS configuration is missing"
fi

cd ../../../../

echo "[4/7] Checking Kubernetes External Secrets..."
kubectl get secretstore aws-secretsmanager -n motionmesh >/dev/null 2>&1 && pass "SecretStore exists" || fail "SecretStore missing"
kubectl get externalsecret motionmesh-secrets -n motionmesh -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' | grep "True" >/dev/null && pass "ExternalSecret motionmesh-secrets is Synced" || fail "ExternalSecret motionmesh-secrets is NOT Synced"
kubectl get secret motionmesh-secrets -n motionmesh >/dev/null 2>&1 && pass "K8s Secret motionmesh-secrets was successfully created" || fail "K8s Secret motionmesh-secrets missing"

echo "[5/7] Checking Application SDK Identity Access..."
./scripts/test-pod-identity.sh $ENVIRONMENT || fail "Pod Identity Tests Failed"

echo "[6/7] Checking Active Connections (DB, Redis, NATS)..."
kubectl delete pod diag-db-test -n motionmesh --ignore-not-found 2>/dev/null
kubectl delete pod diag-nats-test -n motionmesh --ignore-not-found 2>/dev/null

kubectl run diag-db-test --image=alpine:3.20 -n motionmesh \
  --overrides='{"spec": {"containers": [{"name": "diag-db-test", "image": "alpine:3.20", "command": ["sleep", "300"], "envFrom": [{"secretRef": {"name": "motionmesh-secrets"}}]}]}}' \
  --restart=Never >/dev/null 2>&1

kubectl run diag-nats-test --image=natsio/nats-box:latest -n motionmesh --restart=Never --command -- sleep 300 >/dev/null 2>&1

kubectl wait --for=condition=Ready pod/diag-db-test -n motionmesh --timeout=60s >/dev/null 2>&1 || fail "diag-db-test pod failed to start"
kubectl wait --for=condition=Ready pod/diag-nats-test -n motionmesh --timeout=60s >/dev/null 2>&1 || fail "diag-nats-test pod failed to start"

echo "-> Installing DB tools..."
kubectl exec diag-db-test -n motionmesh -- apk add postgresql-client redis >/dev/null 2>&1

echo "-> Testing Postgres connection using injected DATABASE_URL..."
kubectl exec diag-db-test -n motionmesh -- sh -c 'psql $DATABASE_URL -c "\q"' >/dev/null 2>&1 && pass "Postgres Connected" || fail "Postgres Connection Failed"

echo "-> Testing Redis connection using injected REDIS_URL..."
kubectl exec diag-db-test -n motionmesh -- sh -c 'redis-cli -u $REDIS_URL PING' | grep PONG >/dev/null 2>&1 && pass "Redis Connected" || fail "Redis Connection Failed"

echo "-> Testing NATS Connection..."
kubectl exec diag-nats-test -n motionmesh -- nats server info -s nats://nats.motionmesh.svc.cluster.local:4222 >/dev/null 2>&1 && pass "NATS Server Connected" || fail "NATS Connection Failed"

# Cleanup
kubectl delete pod diag-db-test diag-nats-test -n motionmesh --ignore-not-found 2>/dev/null || true

echo "[7/7] Checking Routing, ALB, WAF, ACM, and CDN..."
cd infra/terraform/envs/$ENVIRONMENT

ALB_HOST=$(kubectl get ingress motionmesh-api-ingress -n motionmesh -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")
if [[ -n "$ALB_HOST" ]]; then
    pass "ALB successfully provisioned by LBC: $ALB_HOST"
    
    ALB_ARN=$(aws elbv2 describe-load-balancers --region $AWS_REGION --query "LoadBalancers[?DNSName=='$ALB_HOST'].LoadBalancerArn" --output text 2>/dev/null || echo "")
    if [[ -n "$ALB_ARN" && "$ALB_ARN" != "None" ]]; then
        WAF_ASSOC=$(aws wafv2 get-web-acl-for-resource --resource-arn "$ALB_ARN" --region $AWS_REGION --query 'WebACL.ARN' --output text 2>/dev/null || echo "")
        WAF_EXPECTED=$(terraform output -raw web_acl_arn)
        if [[ "$WAF_ASSOC" == "$WAF_EXPECTED" ]]; then
            pass "ALB is protected by exact WAF ACL"
        else
            fail "ALB WAF ACL mismatch. Expected: $WAF_EXPECTED, Got: $WAF_ASSOC"
        fi

        ACM_EXPECTED=$(terraform output -raw acm_certificate_arn 2>/dev/null || echo "")
        if [[ -n "$ACM_EXPECTED" && "$ACM_EXPECTED" != "MISSING" && "$ACM_EXPECTED" != "None" ]]; then
            LISTENER_ARN=$(aws elbv2 describe-listeners --load-balancer-arn $ALB_ARN --region $AWS_REGION --query 'Listeners[?Protocol==`HTTPS`].ListenerArn' --output text 2>/dev/null || echo "")
            if [[ -n "$LISTENER_ARN" && "$LISTENER_ARN" != "None" ]]; then
                CERT_ARN=$(aws elbv2 describe-listener-certificates --listener-arn $LISTENER_ARN --region $AWS_REGION --query 'Certificates[0].CertificateArn' --output text 2>/dev/null || echo "")
                if [[ "$CERT_ARN" == "$ACM_EXPECTED" ]]; then
                    pass "ALB is using correct ACM Certificate"
                else
                    fail "ALB ACM Certificate mismatch. Expected: $ACM_EXPECTED, Got: $CERT_ARN"
                fi
            else
                fail "Could not find ALB HTTPS listener (Wait for LBC to provision it?)"
            fi
        else
            echo "⚠️  ACM Certificate ARN not provided in outputs, skipping validation"
        fi
    else
        fail "Could not find ALB ARN in AWS for hostname $ALB_HOST"
    fi
else
    fail "ALB not provisioned yet (check aws-load-balancer-controller logs)"
fi

API_DOMAIN=$(terraform output -raw api_domain_name 2>/dev/null || echo "")
ZONE_ID=$(terraform output -raw route53_zone_id 2>/dev/null || echo "")
if [[ -n "$API_DOMAIN" && "$API_DOMAIN" != "None" && -n "$ZONE_ID" && "$ZONE_ID" != "None" ]]; then
    RECORD=$(aws route53 list-resource-record-sets --hosted-zone-id $ZONE_ID --query "ResourceRecordSets[?Name=='$API_DOMAIN.'].Name" --output text 2>/dev/null || echo "")
    if [[ "$RECORD" == "$API_DOMAIN." ]]; then
        pass "DNS record for $API_DOMAIN exists in Route53"
    else
        fail "DNS record for $API_DOMAIN missing in Route53"
    fi
fi

CF_DOMAIN=$(terraform output -raw cloudfront_domain_name 2>/dev/null || echo "")
if [[ -n "$CF_DOMAIN" && "$CF_DOMAIN" != "None" ]]; then
    STATUS=$(aws cloudfront list-distributions --query "DistributionList.Items[?DomainName=='$CF_DOMAIN'].Status" --output text 2>/dev/null || echo "")
    if [[ "$STATUS" == "Deployed" || "$STATUS" == "InProgress" ]]; then
        pass "CloudFront distribution exists ($STATUS)"
    else
        fail "CloudFront distribution missing or unknown status: $STATUS"
    fi
fi

cd ../../../../

echo "======================================"
if [ $FAILURES -gt 0 ]; then
    echo "❌ AWS Wiring Verification FAILED with $FAILURES errors."
    exit 1
else
    echo "✅ AWS Wiring Verification Complete: All successful."
    exit 0
fi
