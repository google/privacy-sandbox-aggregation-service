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

load("//tools/build_defs/license:license.bzl", "license")

package(default_applicable_licenses = ["//third_party/privacy_sandbox_aggregation:license"])

license(
    name = "license",
    package_name = "privacy_sandbox_aggregation",
)

licenses(["notice"])

exports_files([
    "LICENSE",
])

load("@bazel_gazelle//:def.bzl", "gazelle")
# gazelle:prefix github.com/google/privacy-sandbox-aggregation-service
gazelle(name = "gazelle")
