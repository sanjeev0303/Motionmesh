output "dns_name" {
  description = "The DNS name of the load balancer"
  value       = module.alb.dns_name
}

output "arn" {
  description = "The ARN of the load balancer"
  value       = module.alb.arn
}
