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

// This is a C wrapper of the C++ functions in elgamal_encrypt.h, which will be
// called from a Go library through CGO.
#ifndef CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_C_BRIDGE_TESTING_H_
#define CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_C_BRIDGE_TESTING_H_

#include "pipeline/cbytes.h"

#ifdef __cplusplus
extern "C" {
#endif

// C interface for C++ ElGamal encryption functions in
// elgamal_encrypt_testing.h. All methods return true on success and false on
// failure, and the results are returned by parameters prefixed with out*.

bool CGetHashedECPointStrForTesting(const struct CBytes *message_c,
                                    struct CBytes *out_hashed_message_c);

#ifdef __cplusplus
}
#endif

#endif  // CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_C_BRIDGE_TESTING_H_
