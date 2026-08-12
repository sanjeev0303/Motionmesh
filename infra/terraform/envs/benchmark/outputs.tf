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

output "cloudfront_domain_name" {
  value = module.cloudfront.cloudfront_domain_name
}

output "alb_dns_name" {
  value = module.alb.dns_name
}
