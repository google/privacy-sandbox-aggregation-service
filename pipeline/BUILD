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
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:distributednoise",
        "//encryption:incrementaldpf",
        "//encryption:standardencrypt",
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/gcs:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
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
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "//encryption:standardencrypt",
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_binary(
    name = "dpf_generate_raw_conversion",
    srcs = ["dpf_generate_raw_conversion.go"],
    deps = [
        ":dpfdataconverter",
        ":reporttypes",
        "//encryption:cryptoio",
        "//utils",
        "@com_github_golang_glog//:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
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
    name = "reporttypes",
    srcs = ["reporttypes.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/reporttypes",
    deps = ["@com_lukechampine_uint128//:go_default_library"],
)

go_library(
    name = "onepartyaggregator",
    srcs = ["onepartyaggregator.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartyaggregator",
    deps = [
        ":pipelineutils",
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:standardencrypt",
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/transforms/stats:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "onepartyaggregator_test",
    size = "small",
    srcs = ["onepartyaggregator_test.go"],
    embed = [":onepartyaggregator"],
    deps = [
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:standardencrypt",
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_library(
    name = "onepartydataconverter",
    srcs = ["onepartydataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartydataconverter",
    deps = [
        ":pipelineutils",
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:standardencrypt",
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "onepartydataconverter_test",
    size = "small",
    srcs = ["onepartydataconverter_test.go"],
    embed = [":onepartydataconverter"],
    deps = [
        ":onepartyaggregator",
        ":reporttypes",
        "//encryption:cryptoio",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "generate_test_data_pipeline",
    srcs = ["generate_test_data_pipeline.go"],
    deps = [
        ":dpfdataconverter",
        ":onepartydataconverter",
        "//encryption:cryptoio",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_test(
    name = "dpf_pipeline_integration_test",
    size = "small",
    srcs = ["dpf_pipeline_integration_test.go"],
    data = [
        "dpf_test_conversion_data.csv",
        ":create_hybrid_key_pair",
        ":dpf_aggregate_partial_report_pipeline",
        ":generate_test_data_pipeline",
    ],
    deps = [
        ":dpfaggregator",
        "//utils",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "dpf_aggregate_partial_report_pipeline",
    srcs = ["dpf_aggregate_partial_report_pipeline.go"],
    deps = [
        ":dpfaggregator",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "dpf_merge_partial_aggregation_pipeline",
    srcs = ["dpf_merge_partial_aggregation_pipeline.go"],
    deps = [
        ":dpfaggregator",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "oneparty_aggregate_report_pipeline",
    srcs = ["oneparty_aggregate_report_pipeline.go"],
    deps = [
        ":onepartyaggregator",
        "//encryption:cryptoio",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_library(
    name = "dpfdataconverter",
    srcs = ["dpfdataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfdataconverter",
    deps = [
        ":dpfaggregator",
        ":pipelineutils",
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "//encryption:standardencrypt",
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "dpfdataconverter_test",
    size = "small",
    srcs = ["dpfdataconverter_test.go"],
    data = [":dpf_test_conversion_data.csv"],
    embed = [":dpfdataconverter"],
    deps = [
        ":dpfaggregator",
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_library(
    name = "pipelineutils",
    srcs = ["pipelineutils.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils",
    deps = [
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
    ],
)

go_test(
    name = "pipelineutils_test",
    size = "small",
    srcs = ["pipelineutils_test.go"],
    embed = [":pipelineutils"],
    deps = [
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
    ],
)

go_binary(
    name = "dpf_aggregate_reach_partial_report_pipeline",
    srcs = ["dpf_aggregate_reach_partial_report_pipeline.go"],
    deps = [
        ":dpfaggregator",
        ":reachaggregator",
        "//encryption:cryptoio",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "generate_reach_test_data_pipeline",
    srcs = ["generate_reach_test_data_pipeline.go"],
    deps = [
        ":reachdataconverter",
        "//encryption:cryptoio",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/x/beamx:go_default_library",
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
        "//utils",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/gcs:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
    ],
)

go_test(
    name = "reachaggregator_test",
    size = "small",
    srcs = ["reachaggregator_test.go"],
    embed = [":reachaggregator"],
    deps = ["@com_github_google_go_cmp//cmp:go_default_library"],
)

go_library(
    name = "reachdataconverter",
    srcs = ["reachdataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/reachdataconverter",
    deps = [
        ":dpfdataconverter",
        ":pipelineutils",
        ":reporttypes",
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/io/textio:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_test(
    name = "reachdataconverter_test",
    size = "small",
    srcs = ["reachdataconverter_test.go"],
    embed = [":reachdataconverter"],
    deps = [
        ":dpfaggregator",
        ":reachaggregator",
        ":reporttypes",
        "//encryption:cryptoio",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam:go_default_library",
        "@com_github_apache_beam_sdks_v2//go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
    ],
)
