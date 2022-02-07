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
  resource_prefix                 = "${var.project}-${var.environment}-${kubernetes_namespace.collector.metadata[0].name}"
  kubernetes_service_account_name = "${local.resource_prefix}-k8s-svc-acc"
  service_account_gcp_role_member = "serviceAccount:${var.project}.svc.id.goog[${kubernetes_namespace.collector.metadata[0].name}/${kubernetes_service_account.collector.metadata[0].name}]"
}

resource "kubernetes_namespace" "collector" {
  metadata {
    name = "collector"
    annotations = {
      environment = var.environment
    }
  }
}

resource "kubernetes_service_account" "collector" {
  automount_service_account_token = false
  metadata {
    name      = local.kubernetes_service_account_name
    namespace = kubernetes_namespace.collector.metadata[0].name
    annotations = {
      environment                      = var.environment
      "iam.gke.io/gcp-service-account" = var.service_account
    }
  }
}

resource "kubernetes_service" "collector" {
  wait_for_load_balancer = true
  metadata {
    name      = "collector-${var.environment}"
    namespace = kubernetes_namespace.collector.metadata[0].name
  }
  spec {
    port {
      name     = "collector-endpoint"
      port     = 8080
      protocol = "TCP"
    }
    type = "LoadBalancer"
    # Selector must match the label(s) on kubernetes_deployment.collector
    selector = {
      app = "collector-worker"
      env = var.environment
    }
  }

  lifecycle {
    ignore_changes = [
      metadata[0].annotations["cloud.google.com/neg"]
    ]
  }
}

resource "kubernetes_deployment" "collector" {
  timeouts {
    create = "2m"
    delete = "10m"
  }
  metadata {
    name      = "collector-${var.environment}"
    namespace = kubernetes_namespace.collector.metadata[0].name
  }
  spec {
    replicas = var.worker_count
    selector {
      match_labels = {
        app = "collector-worker"
        env = var.environment
      }
    }
    template {
      metadata {
        labels = {
          app = "collector-worker"
          env = var.environment
        }
      }
      spec {
        service_account_name            = local.kubernetes_service_account_name
        automount_service_account_token = true
        container {
          name  = "collector-server"
          image = "${var.container_registry}/${var.collector_image}:${var.collector_version}"
          args = [
            "--address=:8080",
            "--batch_size=${var.batch_size}",
            "--batch_dir=gs://${var.data_location}",
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

module "simulator" {
  count                   = var.simulator_settings.enabled ? 1 : 0
  source                  = "../../modules/simulator"
  project                 = var.project
  environment             = var.environment
  service_account         = var.service_account
  expansion_config_bucket = var.data_location
  collector_uri           = "http://${kubernetes_service.collector.status[0].load_balancer[0].ingress[0].ip}:8080"
  simulator_origins       = var.simulator_origins
  send_count              = var.simulator_settings.count
  container_registry      = var.container_registry
  simulator_image         = var.simulator_image
  simulator_version       = var.simulator_version

  depends_on = [kubernetes_deployment.collector]
}
