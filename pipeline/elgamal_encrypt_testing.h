/*
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#ifndef CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_TESTING_H_
#define CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_TESTING_H_

#include <string>

#include "util/status.inc"
#include "absl/strings/string_view.h"
#include "pipeline/elgamal_encrypt.h"

namespace convagg {
namespace crypto {

// GetHashedECPointStr gets the string representation of the ECPoint hashed from
// the given message. This method is used for testing the C++ code, and will
// also be exposed for Go tests.
private_join_and_compute::StatusOr<std::string> GetHashedECPointStrForTesting(
    absl::string_view message, int curve_id = kDefaultCurveId);

}  // namespace crypto
}  // namespace convagg

#endif  // CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_TESTING_H_
