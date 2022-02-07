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
# Parts originated in https://github.com/abetterinternet/prio-server/tree/main/terraform

resource "google_container_cluster" "cluster" {
  # The google provider seems to have not been updated to consider VPC-native
  # clusters as GA, even though they are GA and in fact are now the default for
  # new clusters created through the GCP UIs.
  provider = google-beta

  name = "${var.environment}-cluster"
  # Specifying a region and not a zone here creates a regional cluster,
  # with masters across multiple zones.
  location    = var.settings.location
  description = "Privacyaggregate GKE Cluster ${var.environment}"

  # Using self-managed node pool:
  # https://www.terraform.io/docs/providers/google/r/container_cluster.html#remove_default_node_pool
  remove_default_node_pool = true
  initial_node_count       = 1

  release_channel {
    channel = "REGULAR"
  }

  # Use VPC native cluster (see
  # https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips).
  networking_mode = "VPC_NATIVE"
  network         = google_compute_network.network.self_link
  subnetwork      = google_compute_subnetwork.subnet.self_link
  ip_allocation_policy {
    cluster_ipv4_cidr_block  = module.subnets.network_cidr_blocks["kubernetes_cluster"]
    services_ipv4_cidr_block = module.subnets.network_cidr_blocks["kubernetes_services"]
  }
  private_cluster_config {
    # Cluster nodes will only have private IP addresses and don't have a direct
    # route to the Internet.
    enable_private_nodes = true
    # Still give the control plane a public endpoint, which we need for
    # deployment and troubleshooting.
    enable_private_endpoint = false
    # Block to use for the control plane master endpoints. The control plane
    # resides on its own VPC, and this range will be peered with ours to
    # communicate with it.
    master_ipv4_cidr_block = module.subnets.network_cidr_blocks["kubernetes_control_plane"]
  }

  # Enables workload identity, which enables containers to authenticate as GCP
  # service accounts which may then be used to authenticate to AWS S3.
  # https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity
  workload_identity_config {
    workload_pool = "${var.project}.svc.id.goog"
  }

  # This enables KMS encryption of the contents of the Kubernetes cluster etcd
  # instance which among other things, stores Kubernetes secrets, like the keys
  # used by data share processors to sign batches or decrypt ingestion shares.
  # https://cloud.google.com/kubernetes-engine/docs/how-to/encrypting-secrets
  database_encryption {
    state    = "ENCRYPTED"
    key_name = google_kms_crypto_key.etcd_encryption_key.id
  }

  # Enables boot integrity checking and monitoring for nodes in the cluster.
  # More configuration values are defined in node pools below.
  enable_shielded_nodes = true
}

resource "google_container_node_pool" "worker_nodes" {
  # Beta provider is required for the spot VMs feature
  provider = google-beta

  name               = "${var.environment}-node-pool"
  location           = var.settings.location
  cluster            = google_container_cluster.cluster.name
  initial_node_count = var.settings.initial_node_count
  autoscaling {
    min_node_count = var.settings.min_node_count
    max_node_count = var.settings.max_node_count
  }
  node_config {
    disk_size_gb = "25"
    image_type   = "COS_CONTAINERD"
    machine_type = var.settings.machine_type
    oauth_scopes = [
      "storage-ro",
      "logging-write",
      "monitoring"
    ]

    # Opt into spot VMs so that we use spare GCE capacity which costs less
    # https://cloud.google.com/kubernetes-engine/docs/concepts/spot-vms
    spot = true

    # Configures nodes to obtain workload identity from GKE metadata service
    # https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity
    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }
  }
}

resource "random_string" "kms_id" {
  length  = 8
  upper   = false
  number  = false
  special = false
}

# KMS keyring to store etcd encryption key
resource "google_kms_key_ring" "keyring" {
  name = "${var.environment}-${random_string.kms_id.result}-kms-keyring"
  # Keyrings can also be zonal, but ours must be regional to match the GKE
  # cluster.
  location = var.settings.region
}

# KMS key used by GKE cluster to encrypt contents of cluster etcd, crucially to
# protect Kubernetes secrets.
resource "google_kms_crypto_key" "etcd_encryption_key" {
  name     = "${var.environment}-etcd-encryption-key"
  key_ring = google_kms_key_ring.keyring.id
  purpose  = "ENCRYPT_DECRYPT"
  # Rotate database encryption key every 90 days. This doesn't reencrypt
  # existing secrets unless we go touch them.
  # https://cloud.google.com/kubernetes-engine/docs/how-to/encrypting-secrets#key_rotation
  rotation_period = "7776000s"
}

# We need the project _number_ to construct the GKE service account, below.
data "google_project" "project" {}

# Permit the GKE service account to use the KMS key. We construct the service
# account name per the specification in:
# https://cloud.google.com/kubernetes-engine/docs/how-to/encrypting-secrets#grant_permission_to_use_the_key
resource "google_kms_crypto_key_iam_binding" "etcd_encryption_key_iam_binding" {
  crypto_key_id = google_kms_crypto_key.etcd_encryption_key.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  members = [
    "serviceAccount:service-${data.google_project.project.number}@container-engine-robot.iam.gserviceaccount.com"
  ]
}

data "google_client_config" "current" {}


