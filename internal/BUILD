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

go_binary(
    name = "generate_test_data_pipeline",
    srcs = ["generate_test_data_pipeline.go"],
    deps = [
        ":dpfprocess",
        ":onepartyprocess",
        "//encryption:cryptoio",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_test(
    name = "dpf_pipeline_integration_test",
    size = "small",
    srcs = ["dpf_pipeline_integration_test.go"],
    data = [
        "//internal:generate_test_data_pipeline",
        "//pipeline:dpf_aggregate_partial_report_pipeline",
        "//report:test_data",
        "//tools:create_hybrid_key_pair",
    ],
    deps = [
        "//pipeline:dpfaggregator",
        "//utils",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "dpf_generate_raw_conversion",
    srcs = ["dpf_generate_raw_conversion.go"],
    deps = [
        "//encryption:cryptoio",
        "//report:reporttypes",
        "//tools:dpfconvert",
        "//utils",
        "@com_github_golang_glog//:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "dpf_merge_partial_aggregation_pipeline",
    srcs = ["dpf_merge_partial_aggregation_pipeline.go"],
    deps = [
        "//pipeline:dpfaggregator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_library(
    name = "dpfprocess",
    srcs = ["dpfprocess.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/internal/dpfprocess",
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:standardencrypt",
        "//pipeline:pipelineutils",
        "//report:reporttypes",
        "//tools:dpfconvert",
        "//utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_library(
    name = "onepartyprocess",
    srcs = ["onepartyprocess.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/internal/onepartyprocess",
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//pipeline:pipelineutils",
        "//report:reporttypes",
        "//report:reportutils",
        "//tools:onepartyconvert",
        "//utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
    ],
)

go_test(
    name = "onepartyprocess_test",
    size = "small",
    srcs = ["onepartyprocess_test.go"],
    data = ["//report:test_data"],
    embed = [":onepartyprocess"],
    deps = [
        "//encryption:cryptoio",
        "//pipeline:onepartyaggregator",
        "//report:reporttypes",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_test(
    name = "dpfprocess_test",
    size = "small",
    srcs = ["dpfprocess_test.go"],
    data = ["//report:test_data"],
    embed = [":dpfprocess"],
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//pipeline:dpfaggregator",
        "//report:reporttypes",
        "//tools:dpfconvert",
        "//utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)
