packer {
  required_plugins {
    amazon = {
      version = ">= 0.0.2"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

variable "region" {
  description = "The region to build the image in"
  type        = string
  default     = "us-west-2"
}

variable "instance_type" {
  description = "The instance type Packer will use for the builder"
  type        = string
  default     = "t4g.small"
}

variable "root_volume_size_gb" {
  type    = number
  default = 8
}

variable "ebs_delete_on_termination" {
  description = "Indicates whether the EBS volume is deleted on instance termination."
  type        = bool
  default     = true
}

variable "global_tags" {
  description = "Tags to apply to everything"
  type        = map(string)
  default     = {}
}

variable "ami_tags" {
  description = "Tags to apply to the AMI"
  type        = map(string)
  default     = {}
}

source "amazon-ebs" "ubuntu" {
  ami_name      = "bcs-cloud-controller-ubuntu-jammy-arm64-${formatdate("YYYYMMDDhhmm", timestamp())}"
  instance_type = var.instance_type
  region        = var.region
  source_ami_filter {
    filters = {
      name                = "ubuntu/images/*ubuntu-jammy-*-arm64-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"]
  }
  ssh_username = "ubuntu"
  tags = {
    Name = "${var.ami_tags.Name}-${local.timestamp}"
  }
  #  tags = merge(
  #    var.global_tags,
  #    var.ami_tags,
  #    {
  #      OS_Version    = "ubuntu-jammy"
  #      Release       = "Latest"
  #      Base_AMI_Name = "{{ .SourceAMIName }}"
  #    }
  #  )
  launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = "${var.root_volume_size_gb}"
    volume_type           = "gp3"
    delete_on_termination = "${var.ebs_delete_on_termination}"
  }
}

build {
  name = "bcs-cloud-controller"
  sources = [
    "source.amazon-ebs.ubuntu"
  ]

  provisioner "shell" {
    inline = [
      # fix device not yet seeded or device model not acknowledged
      "sudo apt purge snapd -y",
      "sudo apt autoremove -y",
      "sudo apt install snapd -y", 
      # https://forum.snapcraft.io/t/too-early-for-operations-solved/12243/20
      "sudo snap install microk8s --classic",
    ]
  }
}
