# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
load("@io_bazel_rules_docker//container:container.bzl", "container_image", "container_push")

package(
    default_visibility = ["//visibility:public"],
)

licenses(["notice"])

go_binary(
    name = "aggregation_query_tool",
    srcs = ["aggregation_query_tool.go"],
    deps = [
        "//service:aggregatorservice",
        "//service:query",
        "//shared:utils",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_hashicorp_go_retryablehttp//:go_default_library",
        "@com_github_pborman_uuid//:uuid",
        "@com_google_cloud_go_pubsub//:go_default_library",
    ],
)

go_binary(
    name = "browser_simulator",
    srcs = ["browser_simulator.go"],
    x_defs = {"build": "{BUILD_TIMESTAMP}"},
    deps = [
        "//encryption:cryptoio",
        "//pipeline:pipelinetypes",
        "//shared:reporttypes",
        "//shared:utils",
        "//test:dpfdataconverter",
        "//test:onepartydataconverter",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_hashicorp_go_retryablehttp//:go_default_library",
    ],
)

container_image(
    name = "browser_simulator_image",
    base = "@base_image//image",
    creation_time = "{BUILD_TIMESTAMP}",
    entrypoint = ["/browser_simulator"],
    files = [":browser_simulator"],
    stamp = 1,
)

container_push(
    name = "browser_simulator_image_publish",
    format = "Docker",
    image = ":browser_simulator_image",
    registry = "$(REGISTRY)",
    repository = "$(REPOSITORY)/browser_simulator",
    tag = "$(TAG)",
)

go_binary(
    name = "create_hybrid_key_pair",
    srcs = ["create_hybrid_key_pair.go"],
    deps = [
        "//encryption:cryptoio",
        "//shared:utils",
        "@com_github_golang_glog//:go_default_library",
    ],
)

go_binary(
    name = "dpf_generate_raw_conversion",
    srcs = ["dpf_generate_raw_conversion.go"],
    deps = [
        "//encryption:cryptoio",
        "//pipeline:pipelinetypes",
        "//shared:utils",
        "//test:dpfdataconverter",
        "@com_github_golang_glog//:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "dpf_merge_partial_aggregation_pipeline",
    srcs = ["dpf_merge_partial_aggregation_pipeline.go"],
    deps = [
        "//pipeline:dpfaggregator",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)
