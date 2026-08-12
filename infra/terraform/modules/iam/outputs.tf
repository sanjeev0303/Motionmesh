output "vpc_cni_role_arn" {
  description = "ARN of IAM role for VPC CNI service account"
  value       = module.vpc_cni_irsa.iam_role_arn
}
