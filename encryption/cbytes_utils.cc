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

#include "encryption/cbytes_utils.h"

#include <cstddef>
#include <cstring>
#include <string>

#include "absl/strings/string_view.h"
#include "encryption/cbytes.h"

namespace convagg {
namespace crypto {

bool StrToCBytes(absl::string_view str, CBytes *out_cb) {
  size_t size = str.size();
  // The memory will be freed in the GO code.
  char *copy = static_cast<char *>(malloc(size));
  if (copy == nullptr) {
    return false;
  }
  memcpy(copy, str.data(), size);
  out_cb->c = copy;
  out_cb->l = size;
  return true;
}

bool AllocateCBytes(size_t size, CBytes *out_cb) {
  out_cb->c = static_cast<char *>(malloc(size));
  if (out_cb->c == nullptr) {
    return false;
  }
  out_cb->l = size;
  return true;
}

}  // namespace crypto
}  // namespace convagg
