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

licenses(["notice"])

proto_library(
    name = "crypto_proto",
    srcs = ["crypto.proto"],
    deps = ["@com_github_google_distributed_point_functions//dpf:distributed_point_function_proto"],
)

go_proto_library(
    name = "crypto_go_proto",
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto",
    protos = [":crypto_proto"],
    deps = ["@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto"],
)

cc_proto_library(
    name = "crypto_cc_proto",
    deps = [":crypto_proto"],
)

cc_library(
    name = "cbytes",
    hdrs = ["cbytes.h"],
)

cc_library(
    name = "cbytes_utils",
    srcs = ["cbytes_utils.cc"],
    hdrs = ["cbytes_utils.h"],
    deps = [
        ":cbytes",
        "@com_google_absl//absl/strings",
    ],
)

go_library(
    name = "cryptoio",
    srcs = ["cryptoio.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio",
    deps = [
        ":crypto_go_proto",
        ":ioutils",
        ":standardencrypt",
        "@com_github_google_tink_go//aead:go_default_library",
        "@com_github_google_tink_go//core/registry:go_default_library",
        "@com_github_google_tink_go//integration/gcpkms:go_default_library",
        "@com_github_google_tink_go//keyset:go_default_library",
        "@com_github_google_tink_go//tink:go_default_library",
        "@com_github_pborman_uuid//:uuid",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "cryptoio_test",
    size = "small",
    srcs = ["cryptoio_test.go"],
    data = [
        "@tink_base//testdata:credentials",
    ],
    embed = [":cryptoio"],
    deps = [
        ":crypto_go_proto",
        ":standardencrypt",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_library(
    name = "standardencrypt",
    srcs = ["standardencrypt.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt",
    visibility = ["//visibility:private"],
    deps = [
        ":crypto_go_proto",
        "@com_github_google_tink_go//hybrid:go_default_library",
        "@com_github_google_tink_go//insecurecleartextkeyset:go_default_library",
        "@com_github_google_tink_go//keyset:go_default_library",
    ],
)

go_test(
    name = "standardencrypt_test",
    size = "small",
    srcs = ["standardencrypt_test.go"],
    embed = [":standardencrypt"],
    deps = [
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_library(
    name = "ioutils",
    srcs = ["ioutils.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils",
    deps = [
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_ugorji_go_codec//:go_default_library",
        "@com_google_cloud_go//secretmanager/apiv1:go_default_library",
        "@com_google_cloud_go_storage//:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_genproto//googleapis/cloud/secretmanager/v1:go_default_library",
    ],
)

go_test(
    name = "ioutils_test",
    size = "small",
    srcs = ["ioutils_test.go"],
    embed = [":ioutils"],
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

cc_library(
    name = "distributed_point_function_c_bridge",
    srcs = ["distributed_point_function_c_bridge.cc"],
    hdrs = ["distributed_point_function_c_bridge.h"],
    deps = [
        ":cbytes",
        ":cbytes_utils",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_cc_proto",
        "@com_google_absl//absl/numeric:int128",
        "@com_google_absl//absl/status",
        "@com_google_absl//absl/status:statusor",
        "@com_google_absl//absl/strings",
        "@com_google_absl//absl/types:span",
    ],
)

cc_test(
    name = "distributed_point_function_c_bridge_test",
    srcs = ["distributed_point_function_c_bridge_test.cc"],
    deps = [
        ":cbytes",
        ":cbytes_utils",
        ":distributed_point_function_c_bridge",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_cc_proto",
        "@com_github_google_googletest//:gtest_main",
        "@com_google_absl//absl/random",
        "@com_google_absl//absl/status",
    ],
)

go_library(
    name = "incrementaldpf",
    srcs = ["incrementaldpf.go"],
    cdeps = [
        ":distributed_point_function_c_bridge",  # keep
    ],
    cgo = True,
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf",
    visibility = ["//visibility:private"],
    deps = [
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "incrementaldpf_test",
    size = "small",
    srcs = ["incrementaldpf_test.go"],
    embed = [":incrementaldpf"],
    deps = [
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_library(
    name = "dpfaggregator",
    srcs = ["dpfaggregator.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator",
    deps = [
        ":crypto_go_proto",
        ":distributednoise",
        ":incrementaldpf",
        ":ioutils",
        ":reporttypes",
        ":standardencrypt",
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
        ":crypto_go_proto",
        ":cryptoio",
        ":incrementaldpf",
        ":ioutils",
        ":reporttypes",
        ":standardencrypt",
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

go_binary(
    name = "dpf_generate_raw_conversion",
    srcs = ["dpf_generate_raw_conversion.go"],
    deps = [
        ":cryptoio",
        ":dpfdataconverter",
        ":ioutils",
        ":reporttypes",
        "@com_github_golang_glog//:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "create_hybrid_key_pair",
    srcs = ["create_hybrid_key_pair.go"],
    deps = [
        ":cryptoio",
        ":ioutils",
        "@com_github_golang_glog//:go_default_library",
    ],
)

go_library(
    name = "distributednoise",
    srcs = ["distributednoise.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/distributednoise",
    deps = [
        "@org_gonum_v1_gonum//floats:go_default_library",
        "@org_gonum_v1_gonum//stat/distuv:go_default_library",
    ],
)

go_test(
    name = "distributednoise_test",
    size = "small",
    srcs = ["distributednoise_test.go"],
    embed = [":distributednoise"],
    deps = [
        "@com_github_grd_stat//:go_default_library",
        "@org_gonum_v1_gonum//floats:go_default_library",
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
        ":crypto_go_proto",
        ":ioutils",
        ":reporttypes",
        ":standardencrypt",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/transforms/stats:go_default_library",
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
        ":crypto_go_proto",
        ":cryptoio",
        ":ioutils",
        ":reporttypes",
        ":standardencrypt",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_library(
    name = "onepartydataconverter",
    srcs = ["onepartydataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartydataconverter",
    deps = [
        ":crypto_go_proto",
        ":cryptoio",
        ":ioutils",
        ":reporttypes",
        ":standardencrypt",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "onepartydataconverter_test",
    size = "small",
    srcs = ["onepartydataconverter_test.go"],
    embed = [":onepartydataconverter"],
    deps = [
        ":cryptoio",
        ":onepartyaggregator",
        ":reporttypes",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_binary(
    name = "dpf_aggregate_partial_report_pipeline",
    srcs = ["dpf_aggregate_partial_report_pipeline.go"],
    deps = [
        ":crypto_go_proto",
        ":cryptoio",
        ":dpfaggregator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "dpf_merge_partial_aggregation_pipeline",
    srcs = ["dpf_merge_partial_aggregation_pipeline.go"],
    deps = [
        ":dpfaggregator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "oneparty_aggregate_report_pipeline",
    srcs = ["oneparty_aggregate_report_pipeline.go"],
    deps = [
        ":cryptoio",
        ":onepartyaggregator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_library(
    name = "dpfdataconverter",
    srcs = ["dpfdataconverter.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfdataconverter",
    deps = [
        ":crypto_go_proto",
        ":cryptoio",
        ":dpfaggregator",
        ":incrementaldpf",
        ":ioutils",
        ":reporttypes",
        ":standardencrypt",
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
        ":crypto_go_proto",
        ":cryptoio",
        ":dpfaggregator",
        ":ioutils",
        ":reporttypes",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)
