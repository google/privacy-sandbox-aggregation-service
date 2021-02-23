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

#ifndef CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_CBYTES_H_
#define CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_CBYTES_H_

#ifdef __cplusplus
extern "C" {
#endif

// CharLen records the binary data with a char array and the length of original
// data. The char array needs to be freed by the Go callers.
struct CBytes {
  char *c;
  int l;
};

#ifdef __cplusplus
}
#endif

#endif  // CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_CBYTES_H_
