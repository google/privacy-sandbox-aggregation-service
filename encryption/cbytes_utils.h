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

#ifndef CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_CBYTES_UTILS_H_
#define CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_CBYTES_UTILS_H_

#include <string>

#include "absl/strings/string_view.h"
#include "encryption/cbytes.h"

namespace convagg {
namespace crypto {

bool StrToCBytes(absl::string_view str, CBytes *out_cb);

bool AllocateCBytes(size_t size, CBytes *out_cb);

}  // namespace crypto
}  // namespace convagg

#endif  // CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_CBYTES_UTILS_H_
