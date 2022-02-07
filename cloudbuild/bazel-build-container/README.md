# Building a custom bazel build container to work with Apache Beam Go SDK and Google Cloud Dataflow


## Using Docker and Container Registry

Apache Beam Go SDK v2.35.0 run on Cloud Dataflow only supports binaries linked with GLIBC up to 2.29
This build container links all binaries with GLIBC v2.28


To build this container replace set your PROJECT environment variable to your
Google Cloud project.

PROJECT=replace-me docker build -t gcr.io/$PROJECT/bazel-cloud-build-image:`date +%Y%m%d`
PROJECT=replace-me docker push gcr.io/$PROJECT/bazel-cloud-build-image:`date +%Y%m%d`

Adjust the cloudbuild.yaml to use this image for all the bazel build steps

## Using Cloud Build and Artifact Registry

### Enable Artifact Registry Service

```bash
gcloud services enable artifactregistry.googleapis.com
```

### Create a docker repository in artifact registry

```bash
gcloud artifacts repositories create container-images \
--repository-format=docker --location=us
```

### Run the Cloud Build job to create the build container

```bash
gcloud builds submit --config=cloudbuild.yaml \
--substitutions=_LOCATION="us",_REPOSITORY="container-images",_IMAGE="bazel-cloud-build-image" \
--async .
```
