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
    importpath = "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto",
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
    importpath = "github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio",
    deps = [
        ":crypto_go_proto",
        ":standardencrypt",
        "//shared:reporttypes",
        "//shared:utils",
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
        ":incrementaldpf",
        ":standardencrypt",
        "//shared:reporttypes",
        "//shared:utils",
        "@com_github_bazelbuild_rules_go//go/tools/bazel:go_default_library",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_protobuf//proto",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_library(
    name = "standardencrypt",
    srcs = ["standardencrypt.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt",
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

cc_library(
    name = "distributed_point_function_c_bridge",
    srcs = ["distributed_point_function_c_bridge.cc"],
    hdrs = ["distributed_point_function_c_bridge.h"],
    deps = [
        ":cbytes",
        ":cbytes_utils",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function",
        "@com_github_google_distributed_point_functions//dpf:distributed_point_function_cc_proto",
        "@com_github_google_distributed_point_functions//dpf:int_mod_n",
        "@com_github_google_distributed_point_functions//dpf:tuple",
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
        "@com_google_absl//absl/numeric:int128",
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
    importpath = "github.com/google/privacy-sandbox-aggregation-service/encryption/incrementaldpf",
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
    name = "distributednoise",
    srcs = ["distributednoise.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/encryption/distributednoise",
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
