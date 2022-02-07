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

data "google_project" "project" {}

locals {
  # GCP PubSub creates a service account for each project used to move
  # undeliverable messages from subscriptons to a dead letter topic
  # https://cloud.google.com/pubsub/docs/dead-letter-topics#granting_forwarding_permissions
  pubsub_service_account = "serviceAccount:service-${data.google_project.project.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

resource "google_pubsub_topic" "origin" {
  name = "${var.environment}-aggregator-${var.origin}"
}

resource "google_pubsub_subscription" "origin" {
  name                 = "${var.environment}-aggregator-${var.origin}"
  topic                = google_pubsub_topic.origin.name
  ack_deadline_seconds = var.ack_deadline_seconds

  retry_policy {
    minimum_backoff = var.min_retry_delay
    maximum_backoff = var.max_retry_delay
  }

  # no expiration on subscription
  # https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/pubsub_subscription#expiration_policy
  expiration_policy {
    ttl = ""
  }
  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.dead_letter.id
    max_delivery_attempts = var.max_delivery_attempts
  }
}

resource "google_pubsub_subscription_iam_binding" "origin" {
  subscription = google_pubsub_subscription.origin.name
  role         = "roles/pubsub.subscriber"
  members = [
    local.pubsub_service_account
  ]
}

# Dead letter topic to which undeliverable messages are sent
resource "google_pubsub_topic" "dead_letter" {
  name = "${google_pubsub_topic.origin.name}-dead-letter"
}

resource "google_pubsub_topic_iam_binding" "dead_letter" {
  topic = google_pubsub_topic.dead_letter.name
  role  = "roles/pubsub.publisher"
  members = [
    local.pubsub_service_account
  ]
}

# Subscription on the dead letter topic enabling alerting and debugging
# on undeliverable / unprocessable messages
resource "google_pubsub_subscription" "dead_letter" {
  name                 = google_pubsub_topic.dead_letter.name
  topic                = google_pubsub_topic.dead_letter.name
  ack_deadline_seconds = 600
  # no expiration on DLQ subscription
  expiration_policy {
    ttl = ""
  }
}
