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

load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")
load("@io_bazel_rules_docker//container:container.bzl", "container_image", "container_push")

licenses(["notice"])

package(
    default_visibility = ["//visibility:public"],
)

go_library(
    name = "query",
    srcs = ["query.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/query",
    deps = [
        "//pipeline:dpfaggregator",
        "//shared:utils",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_gonum_v1_gonum//floats:go_default_library",
    ],
)

go_test(
    name = "query_test",
    size = "small",
    srcs = ["query_test.go"],
    embed = [":query"],
    deps = [
        "//encryption:crypto_go_proto",
        "//pipeline:dpfaggregator",
        "//shared:utils",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_library(
    name = "collectorservice",
    srcs = ["collectorservice.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/collectorservice",
    deps = [
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_golang_glog//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)

go_binary(
    name = "collector_server",
    srcs = ["collector_server.go"],
    gc_linkopts = [
        "-X",
        "main.version=$(VERSION)",
    ],
    x_defs = {"build": "{BUILD_TIMESTAMP}"},
    deps = [
        ":collectorservice",
        "@com_github_golang_glog//:go_default_library",
    ],
)

container_image(
    name = "collector_server_image",
    base = "@base_image//image",
    creation_time = "{BUILD_TIMESTAMP}",
    entrypoint = ["/collector_server"],
    files = [":collector_server"],
    stamp = 1,
)

container_push(
    name = "collector_server_image_publish",
    format = "Docker",
    image = ":collector_server_image",
    registry = "$(REGISTRY)",
    repository = "$(REPOSITORY)/collector_server",
    tag = "$(TAG)",
)

go_test(
    name = "collectorservice_test",
    size = "small",
    srcs = ["collectorservice_test.go"],
    embed = [":collectorservice"],
    deps = [
        "//encryption:crypto_go_proto",
        "//shared:reporttypes",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_protobuf//proto",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_binary(
    name = "aggregator_server",
    srcs = ["aggregator_server.go"],
    gc_linkopts = [
        "-X",
        "main.version=$(VERSION)",
    ],
    x_defs = {"build": "{BUILD_TIMESTAMP}"},
    deps = [
        ":aggregatorservice",
        ":query",
        "@com_github_golang_glog//:go_default_library",
    ],
)

go_library(
    name = "aggregatorservice",
    srcs = ["aggregatorservice.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/aggregatorservice",
    deps = [
        ":query",
        "//pipeline:dpfaggregator",
        "//pipeline:onepartyaggregator",
        "//shared:utils",
        "@com_github_golang_glog//:go_default_library",
        "@com_google_cloud_go_pubsub//:go_default_library",
        "@com_google_cloud_go_storage//:go_default_library",
        "@org_golang_google_api//dataflow/v1b3:go_default_library",
    ],
)

container_image(
    name = "aggregator_server_image",
    base = "@base_image//image",
    creation_time = "{BUILD_TIMESTAMP}",
    entrypoint = ["/aggregator_server"],
    files = [
        ":aggregator_server",
        "//pipeline:dpf_aggregate_partial_report_pipeline",
        "//pipeline:dpf_aggregate_reach_partial_report_pipeline",
    ],
    stamp = 1,
)

container_push(
    name = "aggregator_server_image_publish",
    format = "Docker",
    image = ":aggregator_server_image",
    registry = "$(REGISTRY)",
    repository = "$(REPOSITORY)/aggregator_server",
    tag = "$(TAG)",
)

container_image(
    name = "aggregator_server_debug_image",
    base = "@base_debug_image//image",
    creation_time = "{BUILD_TIMESTAMP}",
    entrypoint = ["/aggregator_server"],
    files = [
        ":aggregator_server",
        "//pipeline:dpf_aggregate_partial_report_pipeline",
    ],
    stamp = 1,
)

container_push(
    name = "aggregator_server_debug_image_publish",
    format = "Docker",
    image = ":aggregator_server_debug_image",
    registry = "$(REGISTRY)",
    repository = "$(REPOSITORY)/aggregator_server",
    tag = "$(TAG)-debug",
)

go_binary(
    name = "aggregator_server_static",
    srcs = ["aggregator_server.go"],
    gc_linkopts = [
        "-X",
        "main.version=$(VERSION)",
        "-extldflags",
        "-static",
    ],
    deps = [
        ":aggregatorservice",
        ":query",
        "@com_github_golang_glog//:go_default_library",
    ],
)

container_image(
    name = "aggregator_server_static_image",
    base = "@alpine_linux_amd64//image",
    creation_time = "{BUILD_TIMESTAMP}",
    entrypoint = ["/aggregator_server_static"],
    files = [
        ":aggregator_server_static",
        "//pipeline:dpf_aggregate_partial_report_pipeline_static",
    ],
    stamp = 1,
)

go_library(
    name = "jobmonitor",
    srcs = ["jobmonitor.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/jobmonitor",
    deps = ["@com_google_cloud_go_firestore//:go_default_library"],
)
