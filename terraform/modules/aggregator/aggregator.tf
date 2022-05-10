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
  resource_prefix = "${var.project}-${var.environment}-${var.origin}"

  workspace_bucket_name        = "${local.resource_prefix}-${var.workspace_location}"
  shared_bucket_name           = "${local.resource_prefix}-${var.shared_location}"
  dataflow_temp_bucket_name    = "${local.resource_prefix}-${var.dataflow_temp_bucket}"
  dataflow_staging_bucket_name = "${local.resource_prefix}-${var.dataflow_staging_bucket}"

  kubernetes_service_account_name     = "${local.resource_prefix}-k8s-svc-acc"
  service_account_gcp_role_member     = "serviceAccount:${var.project}.svc.id.goog[${var.kubernetes_namespace}/${kubernetes_service_account.aggregator.metadata[0].name}]"
  gcp_workload_identity_pool_provider = ""
  queue                               = module.pubsub.queue
}

resource "kubernetes_service_account" "aggregator" {
  automount_service_account_token = false
  metadata {
    name      = local.kubernetes_service_account_name
    namespace = var.kubernetes_namespace
    annotations = {
      environment                      = var.environment
      "iam.gke.io/gcp-service-account" = var.service_account_email
    }
  }
}

module "gcs" {
  for_each = toset([
    local.workspace_bucket_name,
    local.shared_bucket_name,
    local.dataflow_temp_bucket_name,
  local.dataflow_staging_bucket_name])
  source          = "../../modules/gcs"
  name            = each.value
  location        = var.storage_location
  service_account = var.service_account_email
  bucket_lifecycle = {
    enable = true
    type   = "Delete"
    days   = 7
  }
  # depends_on = [google_kms_crypto_key_iam_binding.bucket_encryption_key]
}

module "pubsub" {
  source                = "../../modules/pubsub"
  environment           = var.environment
  origin                = var.origin
  min_retry_delay       = var.pubsub_settings.min_retry_delay
  max_retry_delay       = var.pubsub_settings.max_retry_delay
  ack_deadline_seconds  = var.pubsub_settings.ack_deadline_seconds
  max_delivery_attempts = var.pubsub_settings.max_delivery_attempts
}

resource "kubernetes_service" "aggregator" {
  wait_for_load_balancer = true
  metadata {
    name      = "aggregator-${var.origin}"
    namespace = var.kubernetes_namespace
  }
  spec {
    port {
      name     = "manifest-endpoint"
      port     = 8080
      protocol = "TCP"
    }
    type = "LoadBalancer"
    # Selector must match the label(s) on kubernetes_deployment.aggregator
    selector = {
      app    = "aggregator-worker"
      origin = var.origin
    }
  }

  lifecycle {
    ignore_changes = [
      metadata[0].annotations["cloud.google.com/neg"]
    ]
  }
}

resource "kubernetes_deployment" "aggregator" {
  timeouts {
    create = "2m"
    delete = "10m"
  }
  metadata {
    name      = "aggregator-${var.origin}"
    namespace = var.kubernetes_namespace
  }
  spec {
    replicas = var.aggregator_worker_count
    selector {
      match_labels = {
        app    = "aggregator-worker"
        origin = var.origin
      }
    }
    template {
      metadata {
        labels = {
          app    = "aggregator-worker"
          origin = var.origin
        }
      }
      spec {
        service_account_name            = local.kubernetes_service_account_name
        automount_service_account_token = true
        container {
          name  = "aggregator-server"
          image = "${var.container_registry}/${var.aggregator_image}:${var.aggregator_version}"
          args = [
            "--address=:8080",
            "--pubsub_topic=${local.queue.topic}",
            "--pubsub_subscription=${local.queue.subscription}",
            "--origin=${var.origin}",
            "--pipeline_runner=dataflow",
            "--dataflow_project=${var.project}",
            "--dataflow_temp_location=${module.gcs[local.dataflow_temp_bucket_name].url}",
            "--dataflow_staging_location=${module.gcs[local.dataflow_staging_bucket_name].url}",
            "--dataflow_region=${var.dataflow_region}",
            "--dataflow_service_account=${var.service_account_email}",
            "--private_key_params_uri=${var.private_keys_manifest_uri}",
            "--workspace_uri=${module.gcs[local.workspace_bucket_name].url}",
            "--shared_dir=${module.gcs[local.shared_bucket_name].url}",
            "-logtostderr=true",
          ]
          port {
            container_port = 8080
            protocol       = "TCP"
          }
          resources {
            requests = {
              memory = "50Mi"
              cpu    = "0.1"
            }
            limits = {
              memory = "2000Mi"
              cpu    = "1.5"
            }
          }
        }
      }
    }
  }
}
