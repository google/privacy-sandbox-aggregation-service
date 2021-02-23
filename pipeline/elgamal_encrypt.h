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

#ifndef CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_H_
#define CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_H_

#include <string>

#include "util/status.inc"
#include "absl/strings/string_view.h"
#include "crypto/openssl.inc"
#include "pipeline/crypto.pb.h"

namespace convagg {
namespace crypto {

constexpr int kDefaultCurveId = NID_X9_62_prime256v1;

struct ElGamalKeys {
  ElGamalPublicKey public_key;
  ElGamalPrivateKey private_key;
};

// GenerateElGamalKeyPair creates a pair of private and public keys.
private_join_and_compute::StatusOr<ElGamalKeys> GenerateElGamalKeyPair(
    int curve_id = kDefaultCurveId);

// GenerateSecret generates an exponential secret.
private_join_and_compute::StatusOr<std::string> GenerateSecret(int curve_id = kDefaultCurveId);

// Encrypt hashes the message to a point on the elliptic curve, and encrypts the
// point.
//
// Returns INVALID_ERROR if the public key is malformed, or if the curve ID is
// invalid.
private_join_and_compute::StatusOr<ElGamalCiphertext> Encrypt(
    absl::string_view message, const ElGamalPublicKey& public_key,
    int curve_id = kDefaultCurveId);

// Decrypt decrypts the cipertext.
//
// Returns INVALID_ERROR if the ciphertext is malformed, or if the curve
// ID is invalid.
private_join_and_compute::StatusOr<std::string> Decrypt(
    const crypto::ElGamalCiphertext& ciphertext,
    const crypto::ElGamalPrivateKey& private_key,
    int curve_id = kDefaultCurveId);

// ExponentiateOnCiphertext applies exponentiation with a given secret on the
// encrypted message. The message is encrypted with the public key of the other
// helper server.
//
// Returns INVALID_ERROR if the ciphertext or public key is malformed, or if the
// curve ID is invalid.
private_join_and_compute::StatusOr<ElGamalCiphertext> ExponentiateOnCiphertext(
    const ElGamalCiphertext& ciphertext, const ElGamalPublicKey& public_key,
    absl::string_view secret_exponent_str, int curve_id = kDefaultCurveId);

// ExponentiateOnECPoint applies exponentiation with a given secret on the
// decrypted ECPoint represented by a string. The ECPoint is decrypted from the
// message that is exponentiated by the other helper server with its secret.
//
// Returns INVALID_ERROR if the curve ID is invalid.
private_join_and_compute::StatusOr<std::string> ExponentiateOnECPointStr(
    absl::string_view ec_point_str, absl::string_view secret_exponent_str,
    int curve_id = kDefaultCurveId);

}  // namespace crypto
}  // namespace convagg

#endif  // CHROME_PRIVACY_SANDBOX_POTASSIUM_AGGREGATION_INFRA_SERVER_ELGAMAL_ENCRYPT_H_
