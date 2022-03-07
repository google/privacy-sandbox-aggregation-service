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

environment = "privacy-aggregate-sample"
project     = "sample-project"

origins = {
  aggregator1 = {
    private_keys_manifest_uri = "gs://sample-project/keys/aggregator1/private-keys.json"
    public_keys_manifest_uri  = "gs://sample-project/keys/aggregator1/public-keys.json"
    # below are optional values with their defaults
    pubsub_topic            = "aggregator1-topic"
    pipeline_runner         = "dataflow"
    dataflow_region         = "us-west1"
    storage_location        = "US"
    workspace_location      = "workspace"
    shared_location         = "shared"
    aggregator_worker_count = 1
  },
  aggregator2 = {
    private_keys_manifest_uri = "gs://sample-project/keys/aggregator2/private-keys.json"
    public_keys_manifest_uri  = "gs://sample-project/keys/aggregator2/public-keys.json"
    # below are optional values with their defaults
    pubsub_topic            = "aggregator2-topic"
    pipeline_runner         = "dataflow"
    dataflow_region         = "us-west1"
    storage_location        = "US"
    workspace_location      = "workspace"
    shared_location         = "shared"
    aggregator_worker_count = 1
  }
}

collector_settings = {
  bucket_name = "collector-data"

  # below are optional values with their defaults
  storage_location = "US"
  batch_size       = 10000
  worker_count     = 1
}

simulator_settings = {
  enabled = false
  count   = 10000
}

container_registry = "us-docker.pkg.dev/sample-project/container-images"
collector_image    = "collector_server"
collector_version  = "latest"
aggregator_image   = "aggregator_server"
aggregator_version = "latest"
simulator_image    = "browser_simulator"
simulator_version  = "latest"

# optional - below are default values
gke_settings = {
  region             = "us-west1"
  location           = "us-west1-a"
  initial_node_count = 1
  min_node_count     = 1
  max_node_count     = 2
  machine_type       = "e2-standard-2"
}

# optional - below are default values
pubsub_settings = {
  min_retry_delay       = "60s"
  max_retry_delay       = "600s"
  ack_deadline_seconds  = 600
  max_delivery_attempts = 5
}

# optional - below are default values
dataflow_settings = {
  temp_location    = "df-temp"
  staging_location = "df-staging"
}
