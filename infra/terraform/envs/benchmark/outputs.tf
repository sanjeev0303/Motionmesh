output "vpc_id" {
  value = module.vpc.vpc_id
}

output "eks_cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "aurora_endpoint" {
  value = module.aurora.cluster_endpoint
}

output "redis_endpoint" {
  value = module.elasticache.redis_endpoint
}

output "s3_bucket_domain" {
  value = module.s3.bucket_domain_name
}

output "bucket_id" {
  value = module.s3.bucket_id
}

output "bucket_region" {
  value = module.s3.bucket_region
}

output "cloudfront_domain_name" {
  value = module.cloudfront.cloudfront_domain_name
}

output "api_repository_url" {
  value = module.ecr_api.repository_url
}

output "worker_repository_url" {
  value = module.ecr_worker.repository_url
}

output "region" {
  value = data.aws_region.current.name
}

output "web_acl_arn" {
  value = module.waf.web_acl_arn
}

output "acm_certificate_arn" {
  value = var.acm_certificate_arn
}
