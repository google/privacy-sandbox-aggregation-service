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
    name = "utils",
    srcs = ["utils.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/shared/utils",
    deps = [
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/gcs:go_default_library",
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_bazelbuild_rules_go//go/tools/bazel:go_default_library",
        "@com_github_golang_glog//:go_default_library",
        "@com_github_ugorji_go_codec//:go_default_library",
        "@com_google_cloud_go_pubsub//:go_default_library",
        "@com_google_cloud_go_secretmanager//apiv1:go_default_library",
        "@com_google_cloud_go_storage//:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
        "@org_golang_google_api//iamcredentials/v1:go_default_library",
        "@org_golang_google_api//idtoken:go_default_library",
        "@org_golang_google_genproto//googleapis/cloud/secretmanager/v1:go_default_library",
    ],
)

go_test(
    name = "utils_test",
    size = "small",
    srcs = ["utils_test.go"],
    embed = [":utils"],
    deps = [
        "@com_github_apache_beam//sdks/go/pkg/beam/io/filesystem/local:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_library(
    name = "reporttypes",
    srcs = ["reporttypes.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes",
    deps = [
        "//encryption:crypto_go_proto",
        "@org_golang_google_protobuf//proto",
    ],
)

go_test(
    name = "reporttypes_test",
    size = "small",
    srcs = ["reporttypes_test.go"],
    embed = [":reporttypes"],
    deps = [
        "//encryption:crypto_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp",
    ],
)
