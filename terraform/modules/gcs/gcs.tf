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

resource "google_storage_bucket" "buckets" {
  name                        = var.name
  location                    = var.location
  force_destroy               = true
  uniform_bucket_level_access = true
  # Automatically purge data after 7 days
  # https://cloud.google.com/storage/docs/lifecycle
  dynamic "lifecycle_rule" {
    for_each = var.bucket_lifecycle.enable ? ["yes"] : []
    content {
      action {
        type = var.bucket_lifecycle.type
      }
      condition {
        age = var.bucket_lifecycle.days
      }
    }
  }
}

resource "google_storage_bucket_iam_binding" "bucket_writer" {
  bucket = google_storage_bucket.buckets.name
  role   = "roles/storage.legacyBucketWriter"
  members = [
    "serviceAccount:${var.service_account}"
  ]
}

resource "google_storage_bucket_iam_binding" "bucket_reader" {
  bucket = google_storage_bucket.buckets.name
  role   = "roles/storage.objectViewer"
  members = [
    "serviceAccount:${var.service_account}"
  ]
}
