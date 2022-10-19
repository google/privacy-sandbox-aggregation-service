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

package(
    default_visibility = ["//visibility:public"],
)

go_test(
    name = "pipeline_integration_test",
    size = "small",
    srcs = ["pipeline_integration_test.go"],
    data = [
        "dpf_test_conversion_data.csv",
        ":generate_test_data_pipeline",
        "//pipeline:dpf_aggregate_partial_report_pipeline",
        "//pipeline:oneparty_aggregate_report_pipeline",
        "//tools:create_hybrid_key_pair",
    ],
    deps = [
        "//pipeline:dpfaggregator",
        "//pipeline:onepartyaggregator",
        "//shared:utils",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_library(
    name = "dpfdataconverter",
    srcs = ["dpfdataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/test/dpfdataconverter",
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "//encryption:standardencrypt",
        "//pipeline:pipelinetypes",
        "//pipeline:pipelineutils",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
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
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:incrementaldpf",
        "//pipeline:dpfaggregator",
        "//pipeline:pipelinetypes",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_binary(
    name = "generate_reach_test_data_pipeline",
    srcs = ["generate_reach_test_data_pipeline.go"],
    deps = [
        ":reachdataconverter",
        "//encryption:cryptoio",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "generate_test_data_pipeline",
    srcs = ["generate_test_data_pipeline.go"],
    deps = [
        ":dpfdataconverter",
        ":onepartydataconverter",
        "//encryption:cryptoio",
        "//shared:reporttypes",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_library(
    name = "onepartydataconverter",
    srcs = ["onepartydataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/test/onepartydataconverter",
    deps = [
        "//encryption:crypto_go_proto",
        "//encryption:cryptoio",
        "//encryption:standardencrypt",
        "//pipeline:pipelinetypes",
        "//pipeline:pipelineutils",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
    ],
)

go_test(
    name = "onepartydataconverter_test",
    size = "small",
    srcs = ["onepartydataconverter_test.go"],
    embed = [":onepartydataconverter"],
    deps = [
        "//encryption:cryptoio",
        "//pipeline:onepartyaggregator",
        "//pipeline:pipelinetypes",
        "//shared:reporttypes",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_library(
    name = "reachdataconverter",
    srcs = ["reachdataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/test/reachdataconverter",
    deps = [
        ":dpfdataconverter",
        "//encryption:crypto_go_proto",
        "//encryption:incrementaldpf",
        "//pipeline:pipelinetypes",
        "//pipeline:pipelineutils",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
    ],
)

go_test(
    name = "reachdataconverter_test",
    size = "small",
    srcs = ["reachdataconverter_test.go"],
    embed = [":reachdataconverter"],
    deps = [
        "//encryption:cryptoio",
        "//pipeline:dpfaggregator",
        "//pipeline:pipelinetypes",
        "//pipeline:reachaggregator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "create_fake_firestore_jobs",
    srcs = ["create_fake_firestore_jobs.go"],
    deps = [
        "//service:jobmonitor",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_pborman_uuid//:uuid",
        "@com_google_cloud_go_firestore//:go_default_library",
    ],
)
