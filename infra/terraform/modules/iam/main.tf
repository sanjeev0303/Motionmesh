# Create basic roles needed for worker nodes if any custom permissions are required.
# By default, EKS module handles standard roles, but we can provision additional roles for IRSA.

# -----------------------------------------------------------------------------
# VPC CNI IRSA
# -----------------------------------------------------------------------------
module "vpc_cni_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.30"

  role_name             = "vpc-cni-${var.environment}"
  attach_vpc_cni_policy = true
  vpc_cni_enable_ipv4   = true

  oidc_providers = {
    main = {
      provider_arn               = data.aws_iam_openid_connect_provider.eks.arn
      namespace_service_accounts = ["kube-system:aws-node"]
    }
  }

  tags = {
    Environment = var.environment
  }
}

# -----------------------------------------------------------------------------
# AWS Load Balancer Controller IRSA
# -----------------------------------------------------------------------------
module "load_balancer_controller_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.30"

  role_name                              = "aws-load-balancer-controller-${var.environment}"
  attach_load_balancer_controller_policy = true

  oidc_providers = {
    main = {
      provider_arn               = data.aws_iam_openid_connect_provider.eks.arn
      namespace_service_accounts = ["kube-system:aws-load-balancer-controller"]
    }
  }

  tags = {
    Environment = var.environment
  }
}

# -----------------------------------------------------------------------------
# Pod Identity: Trust Policy
# -----------------------------------------------------------------------------
data "aws_iam_policy_document" "pod_identity_trust" {
  statement {
    effect = "Allow"
    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }
    actions = [
      "sts:AssumeRole",
      "sts:TagSession"
    ]
  }
}

# -----------------------------------------------------------------------------
# API Role
# -----------------------------------------------------------------------------
resource "aws_iam_role" "api" {
  name               = "motionmesh-api-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "api_s3" {
  name = "s3-access"
  role = aws_iam_role.api.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:HeadObject",
          "s3:PutObject",
          "s3:AbortMultipartUpload",
          "s3:ListBucket"
        ]
        Resource = [
          var.s3_bucket_arn,
          "${var.s3_bucket_arn}/*"
        ]
      }
    ]
  })
}

resource "aws_eks_pod_identity_association" "api" {
  cluster_name    = var.cluster_name
  namespace       = "motionmesh"
  service_account = "motionmesh-api"
  role_arn        = aws_iam_role.api.arn
}

# -----------------------------------------------------------------------------
# Worker Role
# -----------------------------------------------------------------------------
resource "aws_iam_role" "worker" {
  name               = "motionmesh-worker-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "worker_s3" {
  name = "s3-access"
  role = aws_iam_role.worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:HeadObject",
          "s3:PutObject",
          "s3:AbortMultipartUpload",
          "s3:ListBucket",
          "s3:DeleteObject"
        ]
        Resource = [
          var.s3_bucket_arn,
          "${var.s3_bucket_arn}/*"
        ]
      }
    ]
  })
}

resource "aws_eks_pod_identity_association" "worker" {
  cluster_name    = var.cluster_name
  namespace       = "motionmesh"
  service_account = "motionmesh-worker"
  role_arn        = aws_iam_role.worker.arn
}

data "aws_eks_cluster" "this" {
  name = var.cluster_name
}

data "aws_iam_openid_connect_provider" "eks" {
  url = data.aws_eks_cluster.this.identity[0].oidc[0].issuer
}

# -----------------------------------------------------------------------------
# External Secrets Role
# -----------------------------------------------------------------------------
resource "aws_iam_role" "external_secrets" {
  name               = "motionmesh-external-secrets-${var.environment}"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "external_secrets" {
  name = "secrets-access"
  role = aws_iam_role.external_secrets.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:DescribeSecret"
        ]
        Resource = [
          "arn:aws:secretsmanager:*:${data.aws_caller_identity.current.account_id}:secret:motionmesh/${var.environment}/*",
          "arn:aws:secretsmanager:*:${data.aws_caller_identity.current.account_id}:secret:rds!cluster-*"
        ]
      }
    ]
  })
}

data "aws_caller_identity" "current" {}

resource "aws_eks_pod_identity_association" "external_secrets" {
  cluster_name    = var.cluster_name
  namespace       = "external-secrets"
  service_account = "external-secrets"
  role_arn        = aws_iam_role.external_secrets.arn
}
