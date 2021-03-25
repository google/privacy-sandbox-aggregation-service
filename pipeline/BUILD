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

licenses(["notice"])

go_library(
    name = "conversion",
    srcs = ["conversion.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/conversion",
    deps = [
        ":crypto_go_proto",
        ":cryptoio",
        ":elgamalencrypt",
        ":ioutils",
        ":standardencrypt",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

go_library(
    name = "conversionaggregator",
    srcs = ["conversionaggregator.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/conversionaggregator",
    visibility = [
        "//third_party/privacy_sandbox_aggregation/service:__subpackages__",
    ],
    deps = [
        ":conversion",
        ":crypto_go_proto",
        ":secretshare",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/gcs:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/transforms/stats:go_default_library",
        "@com_github_google_differential_privacy_privacy_on_beam//pbeam:go_default_library",
        "@org_golang_google_protobuf//proto",
    ],
)

proto_library(
    name = "crypto_proto",
    srcs = ["crypto.proto"],
    visibility = [
        "//third_party/privacy_sandbox_aggregation/service:__subpackages__",
    ],
)

go_proto_library(
    name = "crypto_go_proto",
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto",
    protos = [":crypto_proto"],
    visibility = [
        "//third_party/privacy_sandbox_aggregation/service:__subpackages__",
    ],
)

cc_proto_library(
    name = "crypto_cc_proto",
    deps = [":crypto_proto"],
)

go_library(
    name = "browsersimulator",
    srcs = ["browsersimulator.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/browsersimulator",
    deps = [
        ":crypto_go_proto",
        ":cryptoio",
        ":elgamalencrypt",
        ":ioutils",
        ":secretshare",
        ":standardencrypt",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
        "@com_github_pborman_uuid//:uuid",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "browsersimulator_test",
    size = "small",
    srcs = ["browsersimulator_test.go"],
    embed = [":browsersimulator"],
    deps = [
        ":conversion",
        ":conversionaggregator",
        ":crypto_go_proto",
        ":cryptoio",
        ":elgamalencrypt",
        ":secretshare",
        ":standardencrypt",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_pborman_uuid//:uuid",
    ],
)

go_library(
    name = "secretshare",
    srcs = ["secretshare.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/secretshare",
)

go_test(
    name = "secretshare_test",
    size = "small",
    srcs = ["secretshare_test.go"],
    embed = [":secretshare"],
    deps = ["@com_github_google_go_cmp//cmp:go_default_library"],
)

cc_library(
    name = "cc_elgamal_encrypt",
    srcs = ["elgamal_encrypt.cc"],
    hdrs = ["elgamal_encrypt.h"],
    deps = [
        ":crypto_cc_proto",
        "@com_google_absl//absl/strings",
        "@private_join_and_compute//crypto:bn_util",
        "@private_join_and_compute//crypto:ec_util",
        "@private_join_and_compute//crypto:elgamal",
        "@private_join_and_compute//crypto:openssl_includes",
        "@private_join_and_compute//util:status_includes",
    ],
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
    ],
)

cc_library(
    name = "cc_elgamal_encrypt_testing",
    testonly = True,
    srcs = ["elgamal_encrypt_testing.cc"],
    hdrs = ["elgamal_encrypt_testing.h"],
    deps = [
        ":cc_elgamal_encrypt",
        "@com_google_absl//absl/strings",
        "@private_join_and_compute//crypto:bn_util",
        "@private_join_and_compute//crypto:ec_util",
        "@private_join_and_compute//util:status_includes",
    ],
)

cc_library(
    name = "elgamal_encrypt_c_bridge",
    srcs = ["elgamal_encrypt_c_bridge.cc"],
    hdrs = ["elgamal_encrypt_c_bridge.h"],
    deps = [
        ":cbytes",
        ":cbytes_utils",
        ":cc_elgamal_encrypt",
        "@com_google_absl//absl/strings",
    ],
)

cc_library(
    name = "elgamal_encrypt_testing_c_bridge",
    testonly = True,
    srcs = ["elgamal_encrypt_testing_c_bridge.cc"],
    hdrs = ["elgamal_encrypt_testing_c_bridge.h"],
    deps = [
        ":cbytes",
        ":cbytes_utils",
        ":cc_elgamal_encrypt_testing",
    ],
)

go_library(
    name = "elgamalencrypt",
    srcs = ["elgamalencrypt.go"],
    cdeps = [
        ":elgamal_encrypt_c_bridge",  # keep
    ],
    cgo = True,
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt",
    deps = [":crypto_go_proto"],
)

go_test(
    name = "elgamalencrypt_test",
    size = "small",
    srcs = ["elgamalencrypt_test.go"],
    embed = [":elgamalencrypt"],
    deps = [
        ":elgamalencrypttesting",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)

go_library(
    name = "elgamalencrypttesting",
    testonly = True,
    srcs = ["elgamalencrypttesting.go"],
    cdeps = [
        ":elgamal_encrypt_testing_c_bridge",  # keep
    ],
    cgo = True,
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypttesting",
)

go_library(
    name = "cryptoio",
    srcs = ["cryptoio.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio",
    visibility = [
        "//third_party/privacy_sandbox_aggregation/service:__subpackages__",
    ],
    deps = [
        ":crypto_go_proto",
        ":elgamalencrypt",
        ":standardencrypt",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "cryptoio_test",
    size = "small",
    srcs = ["cryptoio_test.go"],
    embed = [":cryptoio"],
    deps = [
        ":elgamalencrypt",
        ":elgamalencrypttesting",
        ":standardencrypt",
    ],
)

go_library(
    name = "standardencrypt",
    srcs = ["standardencrypt.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt",
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

go_test(
    name = "conversion_test",
    size = "small",
    srcs = ["conversion_test.go"],
    embed = [":conversion"],
    deps = [
        ":crypto_go_proto",
        ":elgamalencrypt",
        ":elgamalencrypttesting",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
    ],
)

go_test(
    name = "conversionaggregator_test",
    size = "small",
    srcs = ["conversionaggregator_test.go"],
    embed = [":conversionaggregator"],
    deps = [
        ":browsersimulator",
        ":conversion",
        ":crypto_go_proto",
        ":elgamalencrypt",
        ":elgamalencrypttesting",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/passert:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/testing/ptest:go_default_library",
    ],
)

go_binary(
    name = "aggregate_partial_report",
    srcs = ["aggregate_partial_report.go"],
    deps = [
        ":conversion",
        ":conversionaggregator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "exponentiate_conversion_key",
    srcs = ["exponentiate_conversion_key.go"],
    deps = [
        ":conversion",
        ":cryptoio",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "generate_partial_report",
    srcs = ["generate_partial_report.go"],
    deps = [
        ":browsersimulator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_binary(
    name = "merge_partial_aggregation",
    srcs = ["merge_partial_aggregation.go"],
    deps = [
        ":conversionaggregator",
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/log:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/x/beamx:go_default_library",
    ],
)

go_library(
    name = "ioutils",
    srcs = ["ioutils.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils",
    deps = [
        "@com_github_apache_beam//sdks/go/pkg/beam:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/textio:go_default_library",
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

go_proto_library(
    name = "distributed_point_function_go_proto",
    protos = ["//third_party/private_statistics/dpf:distributed_point_function_proto"],
)
