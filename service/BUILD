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

proto_library(
    name = "service_proto",
    srcs = ["service.proto"],
    deps = ["//pipeline:crypto_proto"],
)

go_proto_library(
    name = "service_go_grpc_proto",
    compilers = ["@io_bazel_rules_go//proto:go_grpc"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto",
    proto = ":service_proto",
    deps = [
        "//pipeline:crypto_go_proto",
    ],
)

go_binary(
    name = "server",
    srcs = ["server.go"],
    deps = [
        ":service",
        ":service_go_grpc_proto",
        "@com_github_golang_glog//:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//reflection:go_default_library",
    ],
)

go_library(
    name = "service",
    srcs = ["service.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/service",
    deps = [
        ":service_go_grpc_proto",
        "//pipeline:cryptoio",
    ],
)

go_binary(
    name = "create_crypto_key",
    srcs = ["create_crypto_key.go"],
    deps = [
        ":service_go_grpc_proto",
        "//pipeline:crypto_go_proto",
        "//pipeline:cryptoio",
        "@com_github_golang_glog//:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_binary(
    name = "simulate_reporter_partial_aggregation",
    srcs = ["simulate_reporter_partial_aggregation.go"],
    deps = [
        ":service_go_grpc_proto",
        "@com_github_golang_glog//:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_sync//errgroup:go_default_library",
    ],
)

go_binary(
    name = "simulate_reporter_hierarchical_aggregation",
    srcs = ["simulate_reporter_hierarchical_aggregation.go"],
    deps = [
        ":query",
        ":service_go_grpc_proto",
        "//pipeline:crypto_go_proto",
        "@com_github_golang_glog//:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_library(
    name = "query",
    srcs = ["query.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/query",
    deps = [
        ":service_go_grpc_proto",
        "//pipeline:crypto_go_proto",
        "//pipeline:cryptoio",
        "//pipeline:dpfaggregator",
        "//pipeline:ioutils",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_github_pborman_uuid//:uuid",
        "@org_golang_x_sync//errgroup:go_default_library",
        "@org_gonum_v1_gonum//floats:go_default_library",
    ],
)

go_test(
    name = "query_test",
    size = "small",
    srcs = ["query_test.go"],
    embed = [":query"],
    deps = [
        "//pipeline:dpfaggregator",
        "@com_github_google_go_cmp//cmp:go_default_library",
    ],
)

go_library(
    name = "collectorservice",
    srcs = ["collectorservice.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/service/collectorservice",
    deps = [
        "//pipeline:crypto_go_proto",
        "//pipeline:dpfbrowsersimulator",
        "//pipeline:ioutils",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_ugorji_go_codec//:go_default_library",
    ],
)

go_binary(
    name = "collector_server",
    srcs = ["collector_server.go"],
    gc_linkopts = [
        "-extldflags",
        "-static",
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
    base = "@static_base//image",
    creation_time = "{BUILD_TIMESTAMP}",
    entrypoint = ["/collector_server"],
    files = [":collector_server"],
    stamp = True,
)

container_push(
    name = "collector_server_image_publish",
    format = "Docker",
    image = ":collector_server_image",
    registry = "gcr.io",
    repository = "$(GCR_PROJECT)/collector_server",
    tag = "$(VERSION)",
)

go_binary(
    name = "browser_simulator",
    srcs = ["browser_simulator.go"],
    gc_linkopts = [
        "-extldflags",
        "-static",
        "-X",
        "main.version=$(VERSION)",
    ],
    x_defs = {"build": "{BUILD_TIMESTAMP}"},
    deps = [
        ":collectorservice",
        "//pipeline:cryptoio",
        "//pipeline:dpfbrowsersimulator",
        "//pipeline:ioutils",
        "@com_github_golang_glog//:go_default_library",
        "@com_google_cloud_go//compute/metadata:go_default_library",
    ],
)

container_image(
    name = "browser_simulator_image",
    base = "@static_base//image",
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

go_test(
    name = "collectorservice_test",
    size = "small",
    srcs = ["collectorservice_test.go"],
    embed = [":collectorservice"],
    deps = [
        "//pipeline:crypto_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)
