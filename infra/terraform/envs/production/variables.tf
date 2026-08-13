variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.2.0.0/16"
}

variable "availability_zones" {
  description = "List of availability zones"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b", "us-east-1c"]
}

variable "cluster_version" {
  description = "EKS cluster version"
  type        = string
  default     = "1.32"
}

variable "aurora_engine_version" {
  description = "Aurora PostgreSQL engine version"
  type        = string
  default     = "15.8"
}

variable "aurora_instance_class" {
  description = "Aurora instance class"
  type        = string
  default     = "db.r6g.large"
}

variable "media_domain_name" {
  description = "The custom media domain name (e.g., media.motionmesh.com)"
  type        = string
  default     = ""
}

variable "acm_certificate_arn" {
  description = "The ARN of the ACM certificate to use for the custom domain"
  type        = string
  default     = ""
}

variable "route53_zone_id" {
  description = "The ID of the Route53 hosted zone to create the alias record in"
  type        = string
  default     = ""
}
