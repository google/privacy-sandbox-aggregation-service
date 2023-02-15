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

package(
    default_visibility = ["//visibility:public"],
)

licenses(["notice"])

go_library(
    name = "dpfaggregator",
    srcs = ["dpfaggregator.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator",
    deps = [
        ":pipelineutils",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:distributednoise",
        "//encryption:incrementaldpf",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/gcs:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "dpfaggregator_test",
    size = "small",
    srcs = ["dpfaggregator_test.go"],
    embed = [":dpfaggregator"],
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "//encryption:standardencrypt",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_library(
    name = "pipelinetypes",
    srcs = ["pipelinetypes.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelinetypes",
    deps = ["@com_lukechampine_uint128//:go_default_library"],
)

go_library(
    name = "onepartyaggregator",
    srcs = ["onepartyaggregator.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartyaggregator",
    deps = [
        ":pipelineutils",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:distributednoise",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/transforms/stats:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_test(
    name = "onepartyaggregator_test",
    size = "small",
    srcs = ["onepartyaggregator_test.go"],
    embed = [":onepartyaggregator"],
    deps = [
        ":pipelinetypes",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:standardencrypt",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "dpf_aggregate_partial_report_pipeline",
    srcs = ["dpf_aggregate_partial_report_pipeline.go"],
    deps = [
        ":dpfaggregator",
        ":pipelineutils",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "dpf_aggregate_partial_report_pipeline_static",
    srcs = ["dpf_aggregate_partial_report_pipeline.go"],
    gc_linkopts = [
        "-extldflags",
        "-static",
    ],
    deps = [
        ":dpfaggregator",
        ":pipelineutils",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "oneparty_aggregate_report_pipeline",
    srcs = ["oneparty_aggregate_report_pipeline.go"],
    deps = [
        ":onepartyaggregator",
        ":pipelineutils",
        "//encryption:cryptoio",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_library(
    name = "pipelineutils",
    srcs = ["pipelineutils.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils",
    deps = [
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
    ],
)

go_test(
    name = "pipelineutils_test",
    size = "small",
    srcs = ["pipelineutils_test.go"],
    embed = [":pipelineutils"],
    deps = [
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
    ],
)

go_binary(
    name = "dpf_aggregate_reach_partial_report_pipeline",
    srcs = ["dpf_aggregate_reach_partial_report_pipeline.go"],
    deps = [
        ":dpfaggregator",
        ":pipelineutils",
        ":reachaggregator",
        "//encryption:cryptoio",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
        "@com_google_cloud_go_profiler//:go_default_library",
    ],
)

go_library(
    name = "reachaggregator",
    srcs = ["reachaggregator.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/reachaggregator",
    deps = [
        ":dpfaggregator",
        ":pipelineutils",
        "//encryption:crypto_go_proto",
        "//encryption:distributednoise",
        "//encryption:incrementaldpf",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/gcs:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_test(
    name = "reachaggregator_test",
    size = "small",
    srcs = ["reachaggregator_test.go"],
    embed = [":reachaggregator"],
    deps = [
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)
