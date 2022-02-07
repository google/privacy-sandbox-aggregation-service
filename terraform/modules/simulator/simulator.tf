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

locals {
  sample_expansion_config_content = jsonencode({
    PrefixLengths               = [tonumber(20)]
    PrivacyBudgetPerPrefix      = [1]
    ExpansionThresholdPerPrefix = [1]
  })
  resource_prefix                 = "${var.project}-${var.environment}-${kubernetes_namespace.simulator.metadata[0].name}"
  kubernetes_service_account_name = "${local.resource_prefix}-k8s-svc-acc"
  service_account_gcp_role_member = "serviceAccount:${var.project}.svc.id.goog[${kubernetes_namespace.simulator.metadata[0].name}/${kubernetes_service_account.simulator.metadata[0].name}]"
}

resource "google_storage_bucket_object" "sample_expansion_confg" {
  name          = "expansion_configs/config_20bits_1lvl.json"
  bucket        = var.expansion_config_bucket
  content_type  = "application/json"
  cache_control = "no-cache"
  content       = local.sample_expansion_config_content
}

resource "kubernetes_namespace" "simulator" {
  metadata {
    name = "simulator"
    annotations = {
      environment = var.environment
    }
  }
}

resource "kubernetes_service_account" "simulator" {
  automount_service_account_token = false
  metadata {
    name      = local.kubernetes_service_account_name
    namespace = kubernetes_namespace.simulator.metadata[0].name
    annotations = {
      environment                      = var.environment
      "iam.gke.io/gcp-service-account" = var.service_account
    }
  }
}

resource "kubernetes_job" "simulator" {
  timeouts {
    create = "2m"
    delete = "10m"
  }
  metadata {
    name      = "simulator-${var.environment}"
    namespace = kubernetes_namespace.simulator.metadata[0].name
  }
  spec {
    template {
      metadata {
        labels = {
          app = "simulator"
          env = var.environment
        }
      }
      spec {
        service_account_name            = local.kubernetes_service_account_name
        automount_service_account_token = true
        container {
          name  = "simulator"
          image = "${var.container_registry}/${var.simulator_image}:${var.simulator_version}"
          args = [
            "--address=${var.collector_uri}",
            "--helper_public_keys_uri1=${var.simulator_origins[0].public_keys_uri}",
            "--helper_public_keys_uri2=${var.simulator_origins[1].public_keys_uri}",
            "--helper_origin1=${var.simulator_origins[0].origin}",
            "--helper_origin2=${var.simulator_origins[1].origin}",
            "--send_count=${ceil(var.send_count / 20)}",
            "--key_bit_size=20",
            "--conversion_raw=263717,20",
            "--concurrency=20",
            "-logtostderr=true",
          ]
          resources {
            requests = {
              cpu    = "0.5"
              memory = "100Mi"
            }
            limits = {
              cpu    = "4"
              memory = "2000Mi"
            }
          }
        }
      }
    }
    parallelism = 10
    completions = 20
  }
  wait_for_completion = false
}
