variable "environment" {
  description = "Environment name"
  type        = string
}

variable "s3_bucket_domain" {
  description = "The domain name of the S3 bucket acting as the origin"
  type        = string
}

variable "media_domain_name" {
  description = "The custom domain name for media delivery (e.g., media.motionmesh.com)"
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
