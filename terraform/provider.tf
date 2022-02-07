# Copyright 2022, Google LLC.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

terraform {
  backend "gcs" {}

  required_version = ">= 0.14.4"

  # https://www.terraform.io/docs/language/expressions/type-constraints.html#experimental-optional-object-type-attributes
  experiments = [module_variable_optional_attrs]

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.7.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.2.0"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 2.1.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.3.2"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.1.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.1.0"
    }
  }
}

provider "google" {
  # This will use "Application Default Credentials". Run `gcloud auth
  # application-default login` to generate them.
  # https://www.terraform.io/docs/providers/google/guides/provider_reference.html#credentials
  region  = "us-central1"
  project = var.project
}

provider "google-beta" {
  # Duplicate settings from the non-beta provider
  region  = "us-central1"
  project = var.project
}

provider "kubernetes" {
  host                   = local.kubernetes_cluster.endpoint
  cluster_ca_certificate = base64decode(local.kubernetes_cluster.certificate_authority_data)
  token                  = local.kubernetes_cluster.token
}

provider "helm" {
  kubernetes {
    host                   = local.kubernetes_cluster.endpoint
    cluster_ca_certificate = base64decode(local.kubernetes_cluster.certificate_authority_data)
    token                  = local.kubernetes_cluster.token
  }
}
