# Setting up your multi party computation Aggregation Service Environment

The setup is based on [Terraform](https://www.terraform.io/).

## Clone the repository

Clone the repository into a local folder:

```bash
git clone https://github.com/google/privacy-sandbox-aggregation-service;
cd privacy-sandbox-aggregation-service
```

This cloned repostory will be the `project_root` for all following instructions.

## Terraform Setup

The scripts are based on terrafrom version `0.14.4`. We recommend to install a [Terraform version manager](https://github.com/tfutils/tfenv).

Run the following in `<project_root>/terraform`

```bash
tfenv install 0.14.6;
tfenv use 0.14.6
```

## Google Project Setup

Create a [Google Cloud Platfrom](https://cloud.google.com) project and note down the project id.

Install [Google Cloud SDK](https://cloud.google.com/sdk) or if already installed
verify it's up-to-date.

Setup your gcloud. Follow the prompts from `gcloud init`, use the project setup above.

```bash
gcloud init
```

Setup gcloud login and application-default login

```bash
gcloud auth login;
gcloud auth application-default login
```

Enable Artifact Registry Service and Cloud Build APIs

```bash
gcloud services enable artifactregistry.googleapis.com;
gcloud services enable cloudbuild.googleapis.com;
gcloud services enable secretmanager.googleapis.com
```

## Build and Publish Container Images

We need to build 2 sets of container images:

1. A build container image for all bazel builds
1. The `aggregator`, `collector`, and `browser-simulator`container images

### Create a Bucket for Bazel Build Cache

Set a project id as an env variable for the current shell (stay in that shell for all subsequent commands).
Replace `<your-project-id>` with the value from the step above or `gcloud config list --format 'value(core.project)'`.

```bash
export PROJECT_ID=<your-project-id>
```

```bash
gsutil mb -b on gs://$PROJECT_ID-bazelcache
```

### Build Bazel Build Container Image with Cloud Build

Go to `<project_root>/cloudbuild/bazel-build-container` and run:

```bash
gcloud artifacts repositories create container-images \
--repository-format=docker --location=us
```

If above `cmd` fails rerun till it succeeds.

Next, run the Cloud Build job to create the build container

```bash
gcloud builds submit --config=cloudbuild.yaml \
--substitutions=_LOCATION="us",_REPOSITORY="container-images",_IMAGE="bazel-cloud-build-image" .
```

Wait till the build finishes before moving on to the next step.


### Build Aggregator, Collector, Browser-Simulator Container Images with Cloud Build

Return to `<project_root>` directory.

Run the following Cloud Build `cmd`:

```
gcloud builds submit --config=cloudbuild/cloudbuild.yaml \
--substitutions=_LOCATION="us",_REPOSITORY="container-images" \
--async .
```

This will take 15+ minutes, you can check the status [here](https://console.cloud.google.com/cloud-build/builds).
No need to wait for this to finish. Feel free to continue with the next steps.

## Terraform State Backend Setup
Run the following in the `<project_root>/terraform` directory

First we need to pick an environment name (**keep it short, all lowercase, and only "-" as special character**). Replace `<your_env_name>` with your env name.
Make sure `$PROJECT_ID` is still set.

```bash
export ENVIRONMENT=<your_env_name>
```

Next, we need a bucket for the terraform state

```bash
gsutil mb -b on gs://$PROJECT_ID-$ENVIRONMENT
```

Setup the Terrafrom backend

```bash
terraform init -input=false -force-copy -upgrade -verify-plugins=true -backend=true -backend-config="bucket=$PROJECT_ID-$ENVIRONMENT"
```

## Terraform Environment Configuration

Please install [Bazel](https://docs.bazel.build/versions/main/install.html) before you start this section.

### Generate Public/Private Key pairs

Run the following in the `<project_root>` directory

Create a keyring and symetric key. Replace `<your-future-gke-cluster-location>` with the [GCP region](https://cloud.google.com/compute/docs/regions-zones)
you plan to create your GKE cluster in. The default region for the GKE cluster is set to `us-west1`.

```bash
export GKE_CLUSTER_LOCATION=<your-future-gke-cluster-location>;
gcloud kms keyrings create $ENVIRONMENT-packet-keyring --location $GKE_CLUSTER_LOCATION;
gcloud kms keys create packet-key --keyring $ENVIRONMENT-packet-keyring --location $GKE_CLUSTER_LOCATION --purpose=encryption
```

Now create the keys for each origin. You need to run below command twice.
Replace `<replace_with_origin_id>` with e.g. `aggregator1` and in the 2nd run with `aggregator2`.

```bash
ORIGIN=<replace_with_origin_id>; bash bazel run -c opt tools:create_hybrid_key_pair -- \
--key_count=2 \
--private_key_dir=gs://$PROJECT_ID-$ENVIRONMENT/$ORIGIN/private \
--private_key_info_file=gs://$PROJECT_ID-$ENVIRONMENT/keys/$ORIGIN/private-keys.json \
--public_key_info_file=gs://$PROJECT_ID-$ENVIRONMENT/keys/$ORIGIN/public-keys.json \
--kms_key_uri=gcp-kms://projects/$PROJECT_ID/locations/<your-future-gke-cluster-location>/keyRings/$ENVIRONMENT-packet-keyring/cryptoKeys/packet-key \
--secret_project_id=$PROJECT_ID \
-logtostderr=true

```

### Copy and modify terraform configuration (tfvars)

Run the following in directory `<project_root>/terraform`
Copy the [variables/sample.tfvars](variables/sample.tfvars) and make your adjustments

```bash
cp variables/sample.tfvars variables/$ENVIRONMENT.tfvars
```

Use your favorite editor to modify your `enviroment` tfvars.

```bash
open variables/$ENVIRONMENT.tfvars
```

Adjust *at a minimum* the following values:

1. `environment`: replace `privacy-aggregate-sample` with the value picked for `$ENVIRONMENT`
1. `project`:  replace `sample-project` with the value in `$PROJECT_ID`
1. `origins.aggregator[n].private_keys_manifest_uri` with the location of the private keys manifest from above key generation step - make sure they are different for aggregator[1/2]
1. `origins.aggregator[n].public_keys_manifest_uri` with the location of the public keys manifest from above key generation step - make sure they are different for aggregator[1/2]
1. `container_registry` with value `us-docker.pkg.dev/$PROJECT_ID/container-images`
    1. replace `$PROJECT_ID` with the actual value

*Optional*
1. `simulator_settings.enabled` to `true` if you want sample data be generated with the setup of the environment. Note that the sample data will be purged in 7 days (see `modules/gcs/gcs.tf`), and you can follow the `Troubleshooting` section at the end of this document to regenerate it.



## Aggregator Environment Setup

*Cross fingers, hold thumbs, use your lucky charm!*

```bash
terraform init
terraform apply -var-file=variables/$ENVIRONMENT.tfvars
```
Check the plan shown by `terraform apply` and confirm with yes. If it fails the first time, run it again.

The output will show you endpoints you can send data and queries to.

## Send query for aggregation

If you set `simulator_settings.enabled` to true you can run a query against the endpoints listed in the output.

Run the following in directory `<project_root>`. Example query, needs adjustment (replace placeholders marked by `<placeholder>`).
The output of `terraform apply` prints the example query with filled-in
values if `simulator_settings.enabled` is set to `true`.

```bash
GODEBUG=netdns=go bazel run -c opt tools:aggregation_query_tool -- \
--helper_address1 http://<aggregator1-ip>:8080 \
--helper_address2 http://<aggregator2-ip>:8080 \
--partial_report_uri1 gs://$PROJECT_ID-$ENVIRONMENT-collector-data/aggregator1+aggregator2/aggregator1+aggregator2+aggregator1 \
--partial_report_uri2 gs://$PROJECT_ID-$ENVIRONMENT-collector-data/aggregator1+aggregator2/aggregator1+aggregator2+aggregator2 \
--expansion_config_uri gs://$PROJECT_ID-$ENVIRONMENT-collector-data/expansion_configs/config_20bits_1lvl.json \
--result_dir gs://$PROJECT_ID-$ENVIRONMENT/results --key_bit_size 20 \
-logtostderr=true
```

**Wait till the dataflow jobs finish before running the next command**
Dataflow job status can be checked [here](https://console.cloud.google.com/dataflow/jobs)

To merge the partial results (replace placeholders marked by `<placeholder>` )

```bash
UUID=<uuid_from_above_query>; GODEBUG=netdns=go bazel run -c opt tools:dpf_merge_partial_aggregation_pipeline -- \
--partial_histogram_uri1=gs://$PROJECT_ID-$ENVIRONMENT/results/$UUID'_aggregator1' \
--partial_histogram_uri2=gs://$PROJECT_ID-$ENVIRONMENT/results/$UUID'_aggregator2' \
--complete_histogram_uri=gs://$PROJECT_ID-$ENVIRONMENT/results/$UUID'_merged'
```

You can then download the merged result with

```bash
UUID=<uuid_from_above_query>; gsutil cp gs://$PROJECT_ID-$ENVIRONMENT/results/$UUID'_merged' .
```

## Troubleshooting

### Generate sample data

If the sample data gets purged, you can regenerate it in two ways:

1. Set `simulator_settings.enabled = false`, and run `terraform apply`; then set `simulator_settings.enabled = true`, and run `terraform apply` again.


2. Run the follwing in directory `<project_root>` to generate reports and send them to the collector. The `<batch-size>` should be the number defined in `$ENVIRONMENT.tfvars`.

```bash
GODEBUG=netdns=go bazel run -c opt tools:browser_simulator -- \
--address http://<collector-ip>:8080 \
--helper_public_keys_uri1 gs://$PROJECT_ID-$ENVIRONMENT/keys/aggregator1/public-keys.json \
--helper_public_keys_uri2 gs://$PROJECT_ID-$ENVIRONMENT/keys/aggregator2/public-keys.json \
--send_count <batch-size> \
--helper_origin1 aggregator1 \
--helper_origin2 aggregator2 \
-logtostderr=true
```

