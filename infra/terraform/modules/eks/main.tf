module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version

  cluster_endpoint_public_access  = true
  cluster_endpoint_private_access = true

  vpc_id     = var.vpc_id
  subnet_ids = var.subnet_ids

  enable_irsa = true

  eks_managed_node_group_defaults = {
    ami_type       = "AL2_x86_64"
    instance_types = ["m5.large"]
    disk_size      = 50
  }

  eks_managed_node_groups = {
    general = {
      min_size     = 2
      max_size     = 5
      desired_size = 2

      instance_types = ["m5.large", "m5a.large"]
      capacity_type  = "ON_DEMAND"

      labels = {
        Environment = var.environment
        NodeGroup   = "general"
      }
    }

    workers = {
      min_size     = 2
      max_size     = 10
      desired_size = 2

      instance_types = ["c5.xlarge", "c5a.xlarge"]
      capacity_type  = "SPOT"

      labels = {
        Environment = var.environment
        NodeGroup   = "workers"
      }

      taints = [
        {
          key    = "workload"
          value  = "worker"
          effect = "NO_SCHEDULE"
        }
      ]
    }
  }

  enable_cluster_creator_admin_permissions = true

  tags = {
    Environment = var.environment
    Terraform   = "true"
  }
}
