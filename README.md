# Multi-Browser Aggregation Service Prototype

The code in here is related to a prototype for aggregating data across browsers with privacy protection. The mechanism is explained [here](https://github.com/WICG/conversion-measurement-api/blob/master/SERVICE.md). Note that the current MPC protocol has some known flaws. This prototype is a proof of concept, and is not the final design.

# How to build

To build the code, you need to install [Bazel](https://bazel.build/) first, and there are detailed instructions in the Go files for running the binaries.
**You can follow our [Terraform setup](terraform/README.md) to setup an environment.**

# Main pipelines

The following pipelines are implemented based on the [IDPF](https://github.com/google/distributed_point_functions) (Incremental Distributed Point Functions) and [Apache Beam](https://beam.apache.org/). As instructed in the Go files, you can run the pipelines locally, or use other runner engines such as Google Cloud Dataflow. For the latter, you need to have a Google Cloud project first.

There are three main pipelines for the DPF protocol:

1. `pipeline/dpf_aggregate_partial_report_pipeline` expands the DPF keys to histograms and combines the histograms to get partial aggregation results.

2. `pipeline/dpf_aggregate_reach_partial_report_pipeline` expands the DPF keys to histograms of tuples and combines the histograms.

3. `tools/dpf_merge_partial_aggregation` shows an example of how the report origins can obtain the complete aggregation result from the DPF partial results.

# Services

1. `service/collector_server` receives the encrypted partial reports sent by the browsers, and batches them according to the specified helper servers.

2. `service/aggregator_server` hosts two services: a. providing the shared helper information, including the location where the other helper can find the intermediate results for inter-helper communication; and b. processing the aggregation request passed by PubSub messages.

3. `service/browser_simulator` simulates the process how the browser creates the partial reports and sends them to the `collector_server` endpoints.

# Query models
With the `aggregator_server` set up, users can query the aggregation results by sending request with binary `tools/aggregation_query_tool`. There are two modes for the aggregation depending on the configuration passed to the query tool.

## Hierarchical query model
The aggregation is finished in multiple rounds corresponding to different hierarchies. For each hierarchy, the partial reports are aggregated to the prefixes with a certain length of the original bucket IDs. After each round, two helpers exchange and merge the noised hierarchical results so they can figure out the prefixes to be further expanded in the next-level hierarchy. Users need to specify the prefix length and the threshold to filter the prefixes with small values for each hierarchy. Example of the configuration([`HierarchicalConfig`](https://github.com/google/privacy-sandbox-aggregation-service/blob/383a29498eaaef00eb3cb7974869a51a5de7f797/service/query.go#L45)):

```
{
  prefix_lengths: [5, 10, 20, 25],
  expansion_threshold_per_prefix: [10, 5, 5, 5]
  privacy_budget_per_prefix: [.2, .1, .3, .4]
}
```
## Direct query model
The aggregation is finished in one round. Users need to specify the bucket IDs they want to have in the results returned by the helpers. IDs are not included in the configuration will be ignored, while all the ones in the configuration will have noised results. Example of the configuration([`DirectConfig`](https://github.com/google/privacy-sandbox-aggregation-service/blob/383a29498eaaef00eb3cb7974869a51a5de7f797/service/query.go#L52)):

```
{
  bucket_ids: [5, 10, 20, 25],
}
```

## Contributing

Contributions to this repository are always welcome and highly encouraged.

See [CONTRIBUTING](/CONTRIBUTING.md) for more information on how to get started.

## License

Apache 2.0 - See [LICENSE](LICENSE) for more information.

# Disclaimer

This is not an officially supported Google product.
