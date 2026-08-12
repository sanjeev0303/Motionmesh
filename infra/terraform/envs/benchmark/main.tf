module "vpc" {
  source             = "../../modules/vpc"
  name               = "motionmesh-${var.environment}"
  cidr               = var.vpc_cidr
  azs                = var.availability_zones
  public_subnets     = ["10.1.1.0/24", "10.1.2.0/24"]
  private_subnets    = ["10.1.3.0/24", "10.1.4.0/24"]
  database_subnets   = ["10.1.5.0/24", "10.1.6.0/24"]
  single_nat_gateway = true
  tags = {
    Environment = var.environment
  }
}

module "eks" {
  source          = "../../modules/eks"
  environment     = var.environment
  cluster_name    = "motionmesh-${var.environment}"
  vpc_id          = module.vpc.vpc_id
  subnet_ids      = module.vpc.private_subnets
  cluster_version = "1.30"
}

module "aurora" {
  source        = "../../modules/aurora"
  environment   = var.environment
  vpc_id        = module.vpc.vpc_id
  subnet_ids    = module.vpc.database_subnets
  database_name = "motionmesh"
}

module "elasticache" {
  source      = "../../modules/elasticache"
  environment = var.environment
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.database_subnets
}

module "s3" {
  source      = "../../modules/s3"
  environment = var.environment
  bucket_name = "motionmesh-assets-${var.environment}"
}

module "cloudfront" {
  source           = "../../modules/cloudfront"
  environment      = var.environment
  s3_bucket_domain = module.s3.bucket_domain_name
}

module "alb" {
  source      = "../../modules/alb"
  environment = var.environment
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.public_subnets
}

module "waf" {
  source      = "../../modules/waf"
  environment = var.environment
  alb_arn     = module.alb.arn
}

module "iam" {
  source       = "../../modules/iam"
  environment  = var.environment
  cluster_name = module.eks.cluster_id
}

module "monitoring" {
  source       = "../../modules/monitoring"
  environment  = var.environment
  cluster_name = module.eks.cluster_id
}
