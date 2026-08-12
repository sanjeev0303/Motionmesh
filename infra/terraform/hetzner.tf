terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.45"
    }
  }
}

variable "hcloud_token" {
  sensitive = true
}

provider "hcloud" {
  token = var.hcloud_token
}

resource "hcloud_server" "api_server" {
  name        = "motionmesh-api"
  image       = "ubuntu-24.04"
  server_type = "cx22"
  location    = "fsn1"

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }
}

resource "hcloud_server" "worker_server" {
  name        = "motionmesh-worker"
  image       = "ubuntu-24.04"
  server_type = "cpx31"
  location    = "fsn1"

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }
}
