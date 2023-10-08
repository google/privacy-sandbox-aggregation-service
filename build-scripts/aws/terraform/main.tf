# codebuild main.tf
#
# This file is meant to show an example of what environment-specific
# configuration is necessary in each environment. Terraform backend
# configuration cannot reference Terraform variables so this file must be
# customized for each environment.

terraform {
  # Note: the following lines should be uncommented in order to store Terraform
  # state in a remote backend.

  backend "s3" {
    bucket = "gps-as-artifacts-tf-state"
    key    = "dev.tfstate"
    region = "eu-central-1"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.0"
    }
  }
}
