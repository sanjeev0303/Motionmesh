module "vpc" {
  source             = "../../modules/vpc"
  name               = "motionmesh-${var.environment}"
  cidr               = var.vpc_cidr
  azs                = var.availability_zones
  public_subnets     = ["10.2.1.0/24", "10.2.2.0/24", "10.2.3.0/24"]
  private_subnets    = ["10.2.4.0/24", "10.2.5.0/24", "10.2.6.0/24"]
  database_subnets       = ["10.2.7.0/24", "10.2.8.0/24", "10.2.9.0/24"]
  single_nat_gateway     = false
  one_nat_gateway_per_az = true
  tags = {
    Environment = var.environment
  }
}

data "aws_region" "current" {}

module "eks" {
  source          = "../../modules/eks"
  environment     = var.environment
  cluster_name    = "motionmesh-${var.environment}"
  vpc_id          = module.vpc.vpc_id
  subnet_ids      = module.vpc.private_subnets
  cluster_version = var.cluster_version
}

module "aurora" {
  source        = "../../modules/aurora"
  environment   = var.environment
  vpc_id        = module.vpc.vpc_id
  subnet_ids    = module.vpc.database_subnets
  database_name  = "motionmesh"
  engine_version = var.aurora_engine_version
  instance_class = var.aurora_instance_class
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
  source              = "../../modules/cloudfront"
  environment         = var.environment
  s3_bucket_domain               = module.s3.bucket_domain_name
  media_domain_name              = var.media_domain_name
  acm_certificate_arn            = var.acm_certificate_arn
  route53_zone_id                = var.route53_zone_id
  cloudfront_signing_private_key = var.cloudfront_signing_private_key
}

module "alb" {
  source           = "../../modules/alb"
  environment      = var.environment
  vpc_id           = module.vpc.vpc_id
  subnet_ids       = module.vpc.public_subnets
  eks_cluster_name = module.eks.cluster_id
  lbc_role_arn     = module.iam.lbc_role_arn
}

module "waf" {
  source      = "../../modules/waf"
  environment = var.environment
}

module "iam" {
  source        = "../../modules/iam"
  environment   = var.environment
  cluster_name  = module.eks.cluster_id
  s3_bucket_arn = module.s3.bucket_arn
}

module "monitoring" {
  source       = "../../modules/monitoring"
  environment  = var.environment
  cluster_name = module.eks.cluster_id
}

module "ecr_api" {
  source          = "../../modules/ecr"
  environment     = var.environment
  repository_name = "motionmesh-api"
}

module "ecr_worker" {
  source          = "../../modules/ecr"
  environment     = var.environment
  repository_name = "motionmesh-worker"
}

module "ecr_captions" {
  source          = "../../modules/ecr"
  environment     = var.environment
  repository_name = "motionmesh-captions"
}
