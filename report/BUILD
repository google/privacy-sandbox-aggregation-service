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

licenses(["notice"])

load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

package(
    default_visibility = ["//visibility:public"],
)

go_library(
    name = "reporttypes",
    srcs = ["reporttypes.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/report/reporttypes",
    deps = ["@com_lukechampine_uint128//:go_default_library"],
)

go_library(
    name = "reportutils",
    srcs = ["reportutils.go"],
    importpath = "github.com/google/privacy-sandbox-aggregation-service/report/reportutils",
    deps = [
        ":reporttypes",
        "//utils",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

go_test(
    name = "reportutils_test",
    size = "small",
    srcs = ["reportutils_test.go"],
    data = ["test_raw_report_data.csv"],
    embed = [":reportutils"],
    deps = [
        ":reporttypes",
        "//utils",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_lukechampine_uint128//:go_default_library",
    ],
)

filegroup(
    name = "test_data",
    srcs = [
        "test_raw_report_data.csv",
    ],
)
