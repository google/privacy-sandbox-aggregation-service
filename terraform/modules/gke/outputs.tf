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

output "cluster_name" {
  value       = google_container_cluster.cluster.name
  description = "Google Kubernetes cluster name"
}

output "cluster_endpoint" {
  value       = "https://${google_container_cluster.cluster.endpoint}"
  description = "Kubernetes cluster API endpoint"
}

output "certificate_authority_data" {
  value       = google_container_cluster.cluster.master_auth.0.cluster_ca_certificate
  description = "Kubernetes cluster cluster CA certificate"
}

output "kms_keyring" {
  value       = google_kms_key_ring.keyring.id
  description = "Kubernetes cluster KMS Keyring identifier"
}

output "token" {
  value       = data.google_client_config.current.access_token
  description = "Kubernetes cluster access token"
}
