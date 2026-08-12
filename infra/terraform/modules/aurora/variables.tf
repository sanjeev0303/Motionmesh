variable "environment" {
  description = "Environment name (e.g., benchmark, production)"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID where Aurora will be deployed"
  type        = string
}

variable "subnet_ids" {
  description = "A list of subnet IDs where the Aurora cluster will be provisioned"
  type        = list(string)
}

variable "database_name" {
  description = "Name of the initial database to create"
  type        = string
  default     = "motionmesh"
}
