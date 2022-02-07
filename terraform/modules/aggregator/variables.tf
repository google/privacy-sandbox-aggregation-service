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

variable "project" {
  type        = string
  description = "Google Cloud project identifier"
}

variable "environment" {
  type        = string
  description = "system deployment identifier"
}

variable "origin" {
  type        = string
  description = "Origin identifier"
}

variable "kubernetes_namespace" {
  type        = string
  description = "Kubernetes namespace to deploy aggregator in"
}

variable "service_account_email" {
  type        = string
  description = "Service Account email with permissions to access cloud services"
}

variable "service_account_name" {
  type        = string
  description = "Service Account name with permissions to access cloud services"
}

variable "dataflow_region" {
  type        = string
  description = "Dataflow region to run jobs in"
}

variable "dataflow_temp_bucket" {
  type        = string
  description = "Temp location bucket for Dataflow jobs"
}

variable "dataflow_staging_bucket" {
  type        = string
  description = "Staging location bucket for Dataflow jobs"
}

variable "private_keys_manifest_uri" {
  type        = string
  description = "URI to private decryption keys manifest"
}

variable "pubsub_topic" {
  type        = string
  description = "Pub/Sub topic for query job requests"
}

variable "pipeline_runner" {
  type        = string
  description = "Apache Beam pipeline runner type"
}

variable "storage_location" {
  type        = string
  description = "Encrypted reports storage location"
}

variable "workspace_location" {
  type        = string
  description = "Processing workspace location"
}

variable "shared_location" {
  type        = string
  description = "location for sharing data between aggregators"
}

variable "container_registry" {
  type        = string
  description = "Container registry for aggregator container image"
}

variable "aggregator_image" {
  type        = string
  description = "Aggregator container image identifier"
}

variable "aggregator_version" {
  type        = string
  description = "Aggregator container image version"
}

variable "aggregator_worker_count" {
  type        = number
  description = "Count of aggregator instances in worker pool"
}

variable "pubsub_settings" {
  type = object({
    min_retry_delay       = string
    max_retry_delay       = string
    ack_deadline_seconds  = number
    max_delivery_attempts = number
  })
  description = "Pub/Sub subscription config settings"
}
