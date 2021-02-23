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
#ifndef CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_C_BRIDGE_H_
#define CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_C_BRIDGE_H_

#include "pipeline/cbytes.h"

#ifdef __cplusplus
extern "C" {
#endif

// CCiphertext, CHEPublicKey, and CHEPrivateKey convert types in crypto.proto
// between C++ and C.
struct CElGamalCiphertext {
  struct CBytes u;
  struct CBytes e;
};

struct CElGamalPublicKey {
  struct CBytes g;
  struct CBytes y;
};

struct CElGamalPrivateKey {
  struct CBytes x;
};

struct CElGamalKeys {
  struct CElGamalPublicKey public_key;
  struct CElGamalPrivateKey private_key;
};

// C interface for C++ ElGamal encryption functions in elgamal_encrypt.h.
// All methods return true on success and false on failure, and the results are
// returned by parameters prefixed with out*.
bool CGenerateElGamalKeyPair(struct CElGamalKeys *out_elgamal_keys_c);

bool CGenerateSecret(struct CBytes *out_secret_c);

bool CEncrypt(const struct CBytes *message_c,
              const struct CElGamalPublicKey *public_key_c,
              struct CElGamalCiphertext *out_ciphertext_c);

bool CDecrypt(const struct CElGamalCiphertext *ciphertext_c,
              const struct CElGamalPrivateKey *private_key_c,
              struct CBytes *out_decrypted_c);

bool CExponentiateOnCiphertext(const struct CElGamalCiphertext *ciphertext_c,
                               const struct CElGamalPublicKey *public_key_c,
                               const struct CBytes *secret_exponent_c,
                               struct CElGamalCiphertext *out_result_c);

bool CExponentiateOnECPointStr(const struct CBytes *value_str,
                               const struct CBytes *secret_exponent_c,
                               struct CBytes *out_result_c);

#ifdef __cplusplus
}
#endif

#endif  // CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_C_BRIDGE_H_
