variable "environment" {
  description = "Environment name"
  type        = string
}

variable "s3_bucket_domain" {
  description = "The domain name of the S3 bucket acting as the origin"
  type        = string
}
