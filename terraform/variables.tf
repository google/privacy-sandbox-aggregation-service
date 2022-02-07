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

variable "origins" {
  type = map(object({
    private_keys_manifest_uri = string
    public_keys_manifest_uri  = string
    pubsub_topic              = optional(string)
    pipeline_runner           = optional(string)
    dataflow_region           = optional(string)
    storage_location          = optional(string)
    workspace_location        = optional(string)
    shared_location           = optional(string)
    aggregator_worker_count   = optional(number)
  }))
  description = "Map of origins to per-origin aggregator configuration"

}

variable "collector_settings" {
  type = object({
    bucket_name      = string
    storage_location = string
    batch_size       = number
    worker_count     = number
  })
  default = {
    bucket_name      = ""
    storage_location = "US"
    batch_size       = 10000
    worker_count     = 1
  }
  description = "Encrypted packets collector configuration settings"
}

variable "simulator_settings" {
  type = object({
    enabled = bool
    count   = number
  })
  default = {
    enabled = false
    count   = 10000
  }
  description = "Browser simulator to generate encypted packets configuration settings"
}

variable "container_registry" {
  type        = string
  description = "Container registry for aggregator container image"
}

variable "aggregator_image" {
  type        = string
  default     = "aggregator_server"
  description = "Aggregator container image identifier"
}

variable "aggregator_version" {
  type        = string
  description = "Aggregator container image version"
}

variable "collector_image" {
  type        = string
  default     = "collector_server"
  description = "Collector container image identifier"
}

variable "collector_version" {
  type        = string
  description = "Collector container image version"
}

variable "simulator_image" {
  type        = string
  default     = "browser_simulator"
  description = "Browser simulator container image identifier"
}

variable "simulator_version" {
  type        = string
  description = "Browser simulator container image version"
}

variable "gke_settings" {
  type = object({
    region             = string
    location           = string
    initial_node_count = number
    min_node_count     = number
    max_node_count     = number
    machine_type       = string
  })
  default = {
    region             = "us-west1"
    location           = "us-west1-a"
    initial_node_count = 3
    min_node_count     = 1
    max_node_count     = 3
    machine_type       = "e2-standard-2"
  }
  description = "Google Kubernetes cluster settings"
}

variable "pubsub_settings" {
  type = object({
    min_retry_delay       = string
    max_retry_delay       = string
    ack_deadline_seconds  = number
    max_delivery_attempts = number
  })
  default = {
    min_retry_delay       = "60s"
    max_retry_delay       = "600s"
    ack_deadline_seconds  = 600
    max_delivery_attempts = 5
  }
  description = "Pub/Sub subscription settings"
}

variable "dataflow_settings" {
  type = object({
    temp_location    = string
    staging_location = string
  })
  default = {
    temp_location    = "df-temp"
    staging_location = "df-staging"
  }
  description = "Dataflow job settings"
}
