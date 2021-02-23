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

#include "pipeline/elgamal_encrypt_testing_c_bridge.h"

#include <string>

#include "pipeline/cbytes_utils.h"
#include "pipeline/elgamal_encrypt_testing.h"

using convagg::crypto::GetHashedECPointStrForTesting;
using convagg::crypto::StrToCBytes;

bool CGetHashedECPointStrForTesting(const struct CBytes *message_c,
                                    struct CBytes *out_hashed_message_c) {
  std::string message = std::string(message_c->c, message_c->l);
  auto maybe_hashed_message = GetHashedECPointStrForTesting(message);
  if (!maybe_hashed_message.ok()) {
    return false;
  }
  return StrToCBytes(maybe_hashed_message.value(), out_hashed_message_c);
}
