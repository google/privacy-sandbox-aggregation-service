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

output "kubeconfig" {
  value       = "Run this command to update your kubectl config: ${local.kubernetes_cluster.kubectl_command}"
  description = "Command to setup kubectl config"
}

output "collector_service_ip" {
  value       = contains(var.components, "collector") ? "Collector IP is ${module.collector[0].service_ip}" : null
  description = "Collector service IP address"
}

locals {
  aggregator_service_ips = [for v in module.aggregator : v.service_ip]
  aggregator1_ip = (
    contains(var.components, "aggregator1") ?
    local.aggregator_service_ips[0] :
    var.origins["aggregator1"].ip_address
  )
  aggregator2_ip = (
    contains(var.components, "aggregator2") ?
    (
      contains(var.components, "aggregator1") ?
      local.aggregator_service_ips[1] :
      local.aggregator_service_ips[0]
    ) :
    var.origins["aggregator2"].ip_address
  )
  example_query          = <<EOT
  cd .. && bazel run -c opt tools:aggregation_query_tool -- \
  --helper_address1 http://${local.aggregator1_ip}:8080 \
  --helper_address2 http://${local.aggregator2_ip}:8080 \
  --partial_report_uri1 gs://${var.project}-${var.environment}-collector-data/aggregator1+aggregator2/aggregator1+aggregator2+aggregator1 \
  --partial_report_uri2 gs://${var.project}-${var.environment}-collector-data/aggregator1+aggregator2/aggregator1+aggregator2+aggregator2 \
  --expansion_config_uri gs://${var.project}-${var.environment}-collector-data/expansion_configs/config_20bits_1lvl.json \
  --result_dir gs://${var.project}-${var.environment}/results --key_bit_size 20 \
  -logtostderr=true
  EOT
  example_query_availible = var.simulator_settings.enabled && contains(var.components, "collector") && local.aggregator1_ip != "" && local.aggregator2_ip != ""
}

output "example_query" {
  value       = local.example_query_availible ? local.example_query : null
  description = "Example Query to run against aggregators if simulator was enabled"
}

output "uber_service_account" {
  value = "Service account: ${google_service_account.privacyaggregate_uber_svc_account.email}"
  description = "Sevice account for granting permissions"
}

output "aggregator1_ip" {
  value = local.aggregator1_ip != "" && !local.example_query_availible ? "Aggregator1 IP: ${local.aggregator1_ip}" : null
  description = "IP address of aggregator1"
}

output "aggregator2_ip" {
  value = local.aggregator2_ip != "" && !local.example_query_availible ? "Aggregator2 IP: ${local.aggregator2_ip}" : null
  description = "IP address of aggregator2"
}

