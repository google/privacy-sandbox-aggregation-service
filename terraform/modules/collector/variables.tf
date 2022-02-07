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

variable "service_account" {
  type        = string
  description = "Service Account with permissions to access cloud services"
}

variable "data_location" {
  type        = string
  description = "Location to store collected encrypted reports"
}

variable "batch_size" {
  type        = number
  description = "Number of encrypted reports per batch"
}

variable "worker_count" {
  type        = number
  description = "Number of workers in pool to collect encrypted reports"
}

variable "container_registry" {
  type        = string
  description = "Container registry for aggregator container image"
}

variable "collector_image" {
  type        = string
  description = "Collector container image identifier"
}

variable "collector_version" {
  type        = string
  description = "Collector container image version"
}

variable "simulator_image" {
  type        = string
  description = "Browser simulator container image identifier"
}

variable "simulator_version" {
  type        = string
  description = "Browser simulator container image version"
}

variable "simulator_settings" {
  type = object({
    enabled = bool
    count   = number
  })
  description = "Browser simulator configuration settings"
}

variable "simulator_origins" {
  type = list(object({
    origin          = string
    public_keys_uri = string
  }))
  description = "Browser simulator origin settings for share encryption and report endpoint"
}
