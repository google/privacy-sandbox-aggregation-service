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

container_image(
    name = "browser_simulator_image",
    base = "@base_image//image",
    creation_time = "{BUILD_TIMESTAMP}",
    entrypoint = ["/browser_simulator"],
    files = [":browser_simulator"],
    stamp = True,
)

container_push(
    name = "browser_simulator_image_publish",
    format = "Docker",
    image = ":browser_simulator_image",
    registry = "gcr.io",
    repository = "$(GCR_PROJECT)/browser_simulator",
    tag = "$(VERSION)",
)

go_binary(
    name = "browser_simulator",
    srcs = ["browser_simulator.go"],
    x_defs = {"build": "{BUILD_TIMESTAMP}"},
    deps = [
        ":dpfconvert",
        "//encryption:cryptoio",
        "//report:reporttypes",
        "//report:reportutils",
        "//utils",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_hashicorp_go_retryablehttp//:go_default_library",
    ],
)

go_binary(
    name = "create_hybrid_key_pair",
    srcs = ["create_hybrid_key_pair.go"],
    deps = [
        "//encryption:cryptoio",
        "//utils",
        "@com_github_golang_glog//:go_default_library",
    ],
)

go_library(
    name = "dpfconvert",
    srcs = ["dpfconvert.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/tools/dpfconvert",
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "//encryption:standardencrypt",
        "//report:reporttypes",
        "//utils",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_binary(
    name = "simulate_reporter_hierarchical_aggregation",
    srcs = ["simulate_reporter_hierarchical_aggregation.go"],
    deps = [
        "//service:query",
        "@com_github_golang_glog//:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//credentials:go_default_library",
    ],
)

go_binary(
    name = "simulate_reporter_hierarchical_aggregation_pubsub",
    srcs = ["simulate_reporter_hierarchical_aggregation_pubsub.go"],
    deps = [
        "//service:aggregatorservice",
        "//service:query",
        "//utils",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_hashicorp_go_retryablehttp//:go_default_library",
        "@com_github_pborman_uuid//:uuid",
        "@com_google_cloud_go_pubsub//:go_default_library",
    ],
)

go_test(
    name = "dpfconvert_test",
    size = "small",
    srcs = ["dpfconvert_test.go"],
    data = ["//report:test_data"],
    embed = [":dpfconvert"],
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "//pipeline:dpfaggregator",
        "//report:reporttypes",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_library(
    name = "onepartyconvert",
    srcs = ["onepartyconvert.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/tools/onepartyconvert",
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:standardencrypt",
        "//report:reporttypes",
        "//utils",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "onepartyconvert_test",
    size = "small",
    srcs = ["onepartyconvert_test.go"],
    embed = [":onepartyconvert"],
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//pipeline:onepartyaggregator",
        "//report:reporttypes",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)
