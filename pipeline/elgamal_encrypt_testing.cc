// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include "pipeline/elgamal_encrypt_testing.h"

#include "crypto/context.h"
#include "crypto/ec_group.h"
#include "crypto/ec_point.h"
#include "util/status.inc"
#include "absl/strings/string_view.h"

namespace {
using ::private_join_and_compute::Context;
using ::private_join_and_compute::ECGroup;
using ::private_join_and_compute::StatusOr;
}  // namespace

namespace convagg {
namespace crypto {

StatusOr<std::string> GetHashedECPointStrForTesting(absl::string_view message,
                                                    int curve_id) {
  Context context;
  ASSIGN_OR_RETURN(auto ec_group, ECGroup::Create(curve_id, &context));
  ASSIGN_OR_RETURN(auto message_hashed_to_curve,
                   ec_group.GetPointByHashingToCurveSha256(message));
  return message_hashed_to_curve.ToBytesCompressed();
}

}  // namespace crypto
}  // namespace convagg
