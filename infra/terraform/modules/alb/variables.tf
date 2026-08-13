variable "environment" {
  description = "Environment name"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "subnet_ids" {
  description = "Public subnet IDs"
  type        = list(string)
}

variable "eks_cluster_name" {
  description = "EKS cluster name"
  type        = string
}

variable "lbc_role_arn" {
  description = "IAM role ARN for AWS Load Balancer Controller"
  type        = string
}

variable "aws_region" {
  description = "AWS Region"
  type        = string
  default     = "us-east-1"
}
