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
#
# Parts originated from https://github.com/abetterinternet/prio-server/tree/main/terraform

# Activate required services.
resource "google_project_service" "compute" {
  service                    = "compute.googleapis.com"
  disable_dependent_services = true
}

resource "google_project_service" "container" {
  service                    = "container.googleapis.com"
  disable_dependent_services = true
}

resource "google_project_service" "kms" {
  service = "cloudkms.googleapis.com"
}

resource "time_sleep" "wait_for_services" {
  depends_on = [
    google_project_service.container,
    google_project_service.compute,
    google_project_service.kms
  ]

  create_duration = "60s"
}

resource "google_project_service" "dataflow" {
  service = "dataflow.googleapis.com"
}

data "google_project" "project" {}
data "google_client_config" "current" {}
data "google_compute_default_service_account" "default" {

  depends_on = [
    google_project_service.compute
  ]
}

data "terraform_remote_state" "state" {
  backend = "gcs"

  workspace = var.environment

  config = {
    bucket = "${var.project}-${var.environment}"
  }
}

module "gke" {
  source      = "./modules/gke"
  environment = var.environment
  project     = var.project
  settings    = var.gke_settings

  depends_on = [
    time_sleep.wait_for_services
  ]
}

locals {
  project = data.google_project.project.project_id

  svc_account_roles = [
    "roles/pubsub.publisher",
    "roles/pubsub.subscriber",
    "roles/storage.objectCreator",
    "roles/storage.objectViewer",
    "roles/dataflow.developer",
    "roles/dataflow.worker",
    "roles/secretmanager.secretAccessor",
    "roles/cloudkms.cryptoKeyDecrypter",
    "roles/iam.serviceAccountUser"
  ]

  kubernetes_cluster = {
    name                       = module.gke.cluster_name
    endpoint                   = module.gke.cluster_endpoint
    certificate_authority_data = module.gke.certificate_authority_data
    token                      = module.gke.token
    kubectl_command            = "gcloud container clusters get-credentials ${module.gke.cluster_name} --region ${var.gke_settings.region} --project ${var.project}"
  }
  resource_prefix       = "${var.project}-${var.environment}"
  collector_bucket_name = "${local.resource_prefix}-${var.collector_settings.bucket_name}"
  bucket_to_create = contains(var.components, "collector")? ["${local.collector_bucket_name}"] : []
  simulator_origins = [
    for k, v in var.origins : {
      origin          = k
      public_keys_uri = v.public_keys_manifest_uri
    }
  ]

  aggregator_to_create = (
    contains(var.components, "aggregator1") && contains(var.components, "aggregator2") ?
    {aggregator1=var.origins["aggregator1"], aggregator2=var.origins["aggregator2"]} :
    (
      contains(var.components, "aggregator1") ?
      {aggregator1=var.origins["aggregator1"]} :
      (
        contains(var.components, "aggregator2") ?
        {aggregator2=var.origins["aggregator2"]} : {}
      )
    )
  )
}

# One kubernetes namespace per origin
resource "kubernetes_namespace" "namespaces" {
  for_each = local.aggregator_to_create
  metadata {
    name = each.key
    annotations = {
      environment = var.environment
    }
  }

  depends_on = [module.gke]
}

resource "google_service_account" "privacyaggregate_uber_svc_account" {
  account_id   = "${var.environment}-uber-svc-acc"
  display_name = "privacyaggregate-${var.environment}-uber-svc-account"
}

resource "google_service_account_iam_member" "gce-default-account-iam" {
  service_account_id = data.google_compute_default_service_account.default.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.privacyaggregate_uber_svc_account.email}"
}

resource "google_project_iam_member" "privacyaggregate_uber_svc_account" {
  for_each = toset(local.svc_account_roles)
  # service_account_id = google_service_account.privacyaggregate_uber_svc_account.name
  project = var.project
  role    = each.value
  member  = "serviceAccount:${google_service_account.privacyaggregate_uber_svc_account.email}"
}

module "gcs" {
  for_each = toset(local.bucket_to_create)
  source          = "./modules/gcs"
  name            = each.value
  location        = var.collector_settings.storage_location
  service_account = google_service_account.privacyaggregate_uber_svc_account.email
  bucket_lifecycle = {
    enable = true
    type   = "Delete"
    days   = 7
  }
}

module "aggregator" {
  for_each                  = local.aggregator_to_create
  source                    = "./modules/aggregator"
  environment               = var.environment
  project                   = var.project
  origin                    = each.key
  kubernetes_namespace      = each.key
  service_account_email     = google_service_account.privacyaggregate_uber_svc_account.email
  service_account_name      = google_service_account.privacyaggregate_uber_svc_account.name
  private_keys_manifest_uri = each.value.private_keys_manifest_uri
  dataflow_region           = coalesce(each.value.dataflow_region, "us-central1")
  dataflow_temp_bucket      = var.dataflow_settings.temp_location
  dataflow_staging_bucket   = var.dataflow_settings.staging_location
  pubsub_topic              = coalesce(each.value.pubsub_topic, "${each.key}-topic")
  pipeline_runner           = coalesce(each.value.pipeline_runner, "dataflow")
  storage_location          = coalesce(each.value.storage_location, "US")
  workspace_location        = coalesce(each.value.workspace_location, "workspace")
  shared_location           = coalesce(each.value.shared_location, "shared")
  container_registry        = var.container_registry
  aggregator_image          = var.aggregator_image
  aggregator_version        = var.aggregator_version
  aggregator_worker_count   = each.value.aggregator_worker_count
  pubsub_settings           = var.pubsub_settings

  depends_on = [module.gke, kubernetes_namespace.namespaces, google_service_account_iam_binding.binding]
}

locals {
  collector_k8s_accounts = (
    contains(var.components, "collector") ?
    ["serviceAccount:${var.project}.svc.id.goog[collector/${var.project}-${var.environment}-collector-k8s-svc-acc]",
     "serviceAccount:${var.project}.svc.id.goog[simulator/${var.project}-${var.environment}-simulator-k8s-svc-acc]"] :
    []
  )
  service_account_gcp_role_members = concat(
    [for k, v in local.aggregator_to_create :
    "serviceAccount:${var.project}.svc.id.goog[${k}/${var.project}-${var.environment}-${k}-k8s-svc-acc]"],
    local.collector_k8s_accounts
  )
}

# Allows the Kubernetes service account to impersonate the GCP service account.
resource "google_service_account_iam_binding" "binding" {
  service_account_id = google_service_account.privacyaggregate_uber_svc_account.name
  role               = "roles/iam.workloadIdentityUser"
  members            = local.service_account_gcp_role_members

  depends_on = [
    google_service_account.privacyaggregate_uber_svc_account
  ]
}

locals {
  collector_role_members = (
    contains(var.components, "collector") ?
    module.collector[0].service_account_gcp_role_members :
    []
  )
}

resource "google_service_account_iam_binding" "tokens" {
  service_account_id = google_service_account.privacyaggregate_uber_svc_account.name
  role               = "roles/iam.serviceAccountTokenCreator"
  members            = concat([for v in module.aggregator : v.service_account_gcp_role_member], local.collector_role_members)

  depends_on = [module.aggregator, module.collector]
}


module "collector" {
  count = contains(var.components, "collector") ? 1 : 0
  source             = "./modules/collector"
  environment        = var.environment
  project            = var.project
  service_account    = google_service_account.privacyaggregate_uber_svc_account.email
  container_registry = var.container_registry
  collector_image    = var.collector_image
  collector_version  = var.collector_version
  simulator_image    = var.simulator_image
  simulator_version  = var.simulator_version
  simulator_settings = var.simulator_settings
  simulator_origins  = local.simulator_origins
  data_location      = local.collector_bucket_name
  batch_size         = var.collector_settings.batch_size
  worker_count       = var.collector_settings.worker_count

  depends_on = [module.gke]
}

locals {
  aggregator_service_account1 = var.origins["aggregator1"].service_account
  aggregator_service_account2 = var.origins["aggregator2"].service_account
  result_bucket_name = "${local.resource_prefix}"
  shared_bucket_suffix1 = coalesce(var.origins["aggregator1"].shared_location, "shared")
  shared_bucket_suffix2 = coalesce(var.origins["aggregator2"].shared_location, "shared")
}

// Permission of aggregator1 to read the collected reports.
resource "google_storage_bucket_iam_member" "agg1_report_reader" {
  count = (
    contains(var.components, "collector") && !contains(var.components, "aggregator1") && local.aggregator_service_account1 != "" ? 1 : 0
  )
  bucket = local.collector_bucket_name
  role = "roles/storage.objectViewer"
  member = "serviceAccount:${local.aggregator_service_account1}"
}

// Permission of aggregator2 to read the collected reports.
resource "google_storage_bucket_iam_member" "agg2_report_reader" {
  count = (
    contains(var.components, "collector") && !contains(var.components, "aggregator2") && local.aggregator_service_account2 != "" ? 1 : 0
  )
  bucket = local.collector_bucket_name
  role = "roles/storage.objectViewer"
  member = "serviceAccount:${local.aggregator_service_account2}"
}

// Permission of aggregator1 to write results.
resource "google_storage_bucket_iam_member" "agg1_result_writer" {
  count = (
    contains(var.components, "collector") && !contains(var.components, "aggregator1") && local.aggregator_service_account1 != "" ? 1 : 0
  )
  bucket = local.result_bucket_name
  role = "roles/storage.objectCreator"
  member = "serviceAccount:${local.aggregator_service_account1}"
}

// Permission of aggregator2 to write results.
resource "google_storage_bucket_iam_member" "agg2_result_writer" {
  count = (
    contains(var.components, "collector") && !contains(var.components, "aggregator2") && local.aggregator_service_account2 != "" ? 1 : 0
  )
  bucket = local.result_bucket_name
  role = "roles/storage.objectCreator"
  member = "serviceAccount:${local.aggregator_service_account2}"
}

// Permission of aggregator2 to read the shared data.
resource "google_storage_bucket_iam_member" "agg2_shared_data_reader" {
  count = (
    contains(var.components, "aggregator1") && !contains(var.components, "aggregator2") && local.aggregator_service_account2 != "" ? 1 : 0
  )
  bucket = "${var.project}-${var.environment}-aggregator1-${local.shared_bucket_suffix1}"
  role = "roles/storage.objectViewer"
  member = "serviceAccount:${local.aggregator_service_account2}"
}

// Permission of aggregator1 to read the shared data.
resource "google_storage_bucket_iam_member" "agg1_shared_data_reader" {
  count = (
    contains(var.components, "aggregator2") && !contains(var.components, "aggregator1") && local.aggregator_service_account1 != "" ? 1 : 0
  )
  bucket = "${var.project}-${var.environment}-aggregator2-${local.shared_bucket_suffix2}"
  role = "roles/storage.objectViewer"
  member = "serviceAccount:${local.aggregator_service_account1}"
}
