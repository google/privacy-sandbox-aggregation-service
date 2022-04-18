# Local test for the MPC Aggregation Service

## Install dependencies

A couple of tools will be used in this workflow. Please install them before you start:

* [Docker](https://docs.docker.com/get-docker/)
* [Bazel](https://docs.bazel.build/versions/main/install.html)
* [Python](https://cloud.google.com/python/docs/setup)
* [Google Cloud CLI] (https://cloud.google.com/sdk/docs)
* [Java JRE](https://docs.oracle.com/goldengate/1212/gg-winux/GDRAD/java.htm#BGBFJHAB) (version 7 or higher)

## Set up local PubSub emulator

Then install the emulator from a command prompt:

```bash
gcloud components install pubsub-emulator
gcloud components update
```

Start the local emulator:

```bash
gcloud beta emulators pubsub start --project=fakeproject
```
From another terminal, clone the emulator tools repo and get to the directory:

```bash
git clone https://github.com/googleapis/python-pubsub.git
cd python-pubsub/samples/snippets/
```
Make sure the Python PubSub package is installed:

```bash
pip install google-cloud-pubsub
```
Create two topics and subscriptions, which will be listened by two helper servers respectively.

```bash
$(gcloud beta emulators pubsub env-init)
python publisher.py fakeproject create faketopic1
python subscriber.py fakeproject create faketopic1 fakesubscription1
python publisher.py fakeproject create faketopic2
python subscriber.py fakeproject create faketopic2 fakesubscription2
```

## Set up servers

Clone the repository into a local folder:

```bash
git clone https://github.com/google/privacy-sandbox-aggregation-service;
cd privacy-sandbox-aggregation-service
```

Config the environment:

```bash
export WORKSPACE="/path/to/workspace"
export PROJECT_ID=fakeproject
export ENVIRONMENT=myenv
export USER_ID=$(id -u $USER)
$(gcloud beta emulators pubsub env-init)
```

Run the script test/local_deploy.sh to set up all the servers:

```bash
sh ./test/local_deploy.sh
```

This script creates the file system and encryption keys, then sets up three servers:

1. a collector which receives the reports from browsers and batches them;
2. two aggregators which aggregate the batches and write the results to the location specified by the queries.


## Simulate the aggregation workflow

Send reports with the browser simulator to the collector:

```bash
bazel run -c opt tools:browser_simulator -- \
--address=http://0.0.0.0:8080 \
--helper_public_keys_uri1=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator1/public_keys.json \
--helper_public_keys_uri2=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/keys/aggregator2/public_keys.json \
--send_count=10 \
--helper_origin1=aggregator1 \
--helper_origin2=aggregator2
```

The encrypted payloads are batched by the collector, they can be found in:

```bash
echo $WORKSPACE/$PROJECT_ID-$ENVIRONMENT-collector-data/aggregator1+aggregator2
```

Besides the payloads, we also need to define the aggregation configuration, which tells the aggregators how the hierarchies are defined:

```bash
echo '{"PrefixLengths":[20],"PrivacyBudgetPerPrefix":[1],"ExpansionThresholdPerPrefix":[1]}' >> $WORKSPACE/$PROJECT_ID-$ENVIRONMENT-collector-data/config_20bits_1lvl.json
```

Now we can send a query to the aggregators:

```bash
bazel run -c opt tools:aggregation_query_tool -- \
--helper_address1=http://0.0.0.0:8081 \
--helper_address2=http://0.0.0.0:8082 \
--partial_report_uri1=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT-collector-data/aggregator1+aggregator2/aggregator1+aggregator2+aggregator1 \
--partial_report_uri2=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT-collector-data/aggregator1+aggregator2/aggregator1+aggregator2+aggregator2 \
--expansion_config_uri=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT-collector-data/config_20bits_1lvl.json \
--result_dir=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results \
--key_bit_size=32
```

Each query will have an unique ID, which will be returned by the above command. Now, check the result directory and wait for the partial results from both aggregators:

```bash
export UUID=<uuid_from_above_query>
echo $WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results/$UUID'_aggregator1'
echo $WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results/$UUID'_aggregator2'
```

When both files are ready, we can merge the partial results:

```bash
bazel run -c opt tools:dpf_merge_partial_aggregation_pipeline -- \
--partial_histogram_uri1=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results/$UUID'_aggregator1' \
--partial_histogram_uri2=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results/$UUID'_aggregator2' \
--complete_histogram_uri=$WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results/$UUID'_merged'
```

And the complete result is in the file:

```bash
echo $WORKSPACE/$PROJECT_ID-$ENVIRONMENT/results/$UUID'_merged'
```

## Clean up

Stop the servers

```bash
docker stop collector;
docker stop aggregator1;
docker stop aggregator2
```

Remove files

```bash
rm -rf $WORKSPACE/$PROJECT_ID-$ENVIRONMENT*
```
