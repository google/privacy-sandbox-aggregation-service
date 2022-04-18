#!/bin/bash

echo "Create directories"
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator1
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator2
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator1-shared
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator1-workspace
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator2-shared
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator2-workspace
mkdir -p $WORKSPACE/$PROJECT_ID-$ENVIRONMENT-collector-data

echo "Generate encryption keys"
bazel run -c opt tools:create_hybrid_key_pair -- \
--private_key_dir=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator1 \
--private_key_info_file=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator1/private_keys.json \
--public_key_info_file=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator1/public_keys.json

bazel run -c opt tools:create_hybrid_key_pair -- \
--private_key_dir=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator2 \
--private_key_info_file=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator2/private_keys.json \
--public_key_info_file=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator2/public_keys.json

echo "Deploy the collector"
bazel run -c opt service:collector_server_image
docker run -d --rm -p 8080:8080 -v $WORKSPACE:/root --name collector --user $USER_ID bazel/service:collector_server_image \
--address=:8080 \
--batch_dir=/root/$PROJECT_ID-$ENVIRONMENT-collector-data \
--batch_size=10

echo "Deploy aggregators"
bazel build -c opt pipeline:dpf_aggregate_partial_report_pipeline_static
bazel run -c opt service:aggregator_server_static_image
docker run -d --rm \
--net=host \
-v $WORKSPACE:$WORKSPACE --name aggregator1 \
--user $USER_ID \
--env PUBSUB_EMULATOR_HOST=0.0.0.0:8085 \
--env PUBSUB_PROJECT_ID=fakeproject \
bazel/service:aggregator_server_static_image \
--address=:8081 \
--pubsub_topic=projects/fakeproject/topics/faketopic1 \
--pubsub_subscription=projects/fakeproject/subscriptions/fakesubscription1 \
--origin=aggregator1 \
--private_key_params_uri=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator1/private_keys.json \
--dpf_aggregate_partial_report_binary=/dpf_aggregate_partial_report_pipeline_static \
--workspace_uri=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator1-workspace \
--shared_dir=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator1-shared

docker run -d --rm \
--net=host \
-v $WORKSPACE:$WORKSPACE --name aggregator2 \
--user $USER_ID \
--env PUBSUB_EMULATOR_HOST=0.0.0.0:8085 \
--env PUBSUB_PROJECT_ID=fakeproject \
bazel/service:aggregator_server_static_image \
--address=:8082 \
--pubsub_topic=projects/fakeproject/topics/faketopic2 \
--pubsub_subscription=projects/fakeproject/subscriptions/fakesubscription2 \
--origin=aggregator2 \
--private_key_params_uri=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator2/private_keys.json \
--dpf_aggregate_partial_report_binary=/dpf_aggregate_partial_report_pipeline_static \
--workspace_uri=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator2-workspace \
--shared_dir=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT-aggregator2-shared
