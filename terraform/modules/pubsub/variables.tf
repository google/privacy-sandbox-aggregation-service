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

variable "environment" {
  type        = string
  description = "system deployment identifier"
}

variable "origin" {
  type        = string
  description = "Origin identifier"
}

variable "min_retry_delay" {
  type        = string
  description = "Minimum Pub/Sub message delivery retry delay (e.g. 60s)"
}

variable "max_retry_delay" {
  type        = string
  description = "Maximum Pub/Sub message delivery retry delay (e.g. 600s)"
}

variable "ack_deadline_seconds" {
  type        = number
  description = "Message Delivery acknowledge timeout"
}

variable "max_delivery_attempts" {
  type        = number
  description = "Maximum Pub/Sub message delivery attempts before sending to dead-letter topic"
}
