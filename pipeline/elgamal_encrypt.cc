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

#include "pipeline/elgamal_encrypt.h"

#include <utility>

#include "crypto/big_num.h"
#include "crypto/context.h"
#include "crypto/ec_group.h"
#include "crypto/ec_point.h"
#include "crypto/elgamal.h"
#include "util/status.inc"
#include "absl/strings/string_view.h"
#include "pipeline/crypto.pb.h"

namespace {
using ::private_join_and_compute::BigNum;
using ::private_join_and_compute::Context;
using ::private_join_and_compute::ECGroup;
using ::private_join_and_compute::ElGamalDecrypter;
using ::private_join_and_compute::ElGamalEncrypter;
using ::private_join_and_compute::StatusOr;
using ::private_join_and_compute::elgamal::Ciphertext;
using ::private_join_and_compute::elgamal::Exp;
using ::private_join_and_compute::elgamal::GenerateKeyPair;
using ::private_join_and_compute::elgamal::PrivateKey;
using ::private_join_and_compute::elgamal::PublicKey;
}  // namespace

namespace convagg {
namespace crypto {

StatusOr<ElGamalKeys> GenerateElGamalKeyPair(int curve_id) {
  Context context;
  ASSIGN_OR_RETURN(auto ec_group, ECGroup::Create(curve_id, &context));
  ASSIGN_OR_RETURN(auto key_pair, private_join_and_compute::elgamal::GenerateKeyPair(ec_group));
  ASSIGN_OR_RETURN(std::string g, key_pair.first->g.ToBytesCompressed());
  ASSIGN_OR_RETURN(std::string y, key_pair.first->y.ToBytesCompressed());

  ElGamalKeys result;
  result.public_key.set_g(g);
  result.public_key.set_y(y);
  result.private_key.set_x(key_pair.second->x.ToBytes());
  return result;
}

StatusOr<std::string> GenerateSecret(int curve_id) {
  Context context;
  ASSIGN_OR_RETURN(auto ec_group, ECGroup::Create(curve_id, &context));
  BigNum secret_exponent = ec_group.GeneratePrivateKey();
  return secret_exponent.ToBytes();
}

StatusOr<ElGamalCiphertext> Encrypt(absl::string_view message,
                                    const ElGamalPublicKey& public_key,
                                    int curve_id) {
  Context context;
  ASSIGN_OR_RETURN(auto ec_group, ECGroup::Create(curve_id, &context));

  ASSIGN_OR_RETURN(auto g, ec_group.CreateECPoint(public_key.g()));
  ASSIGN_OR_RETURN(auto y, ec_group.CreateECPoint(public_key.y()));
  auto public_key_ptr =
      absl::make_unique<PublicKey>(PublicKey({std::move(g), std::move(y)}));

  // Encrypt a message string using ElGamal encryption.
  ElGamalEncrypter elgamal_encrypter(&ec_group, std::move(public_key_ptr));
  ASSIGN_OR_RETURN(auto message_hashed_to_curve,
                   ec_group.GetPointByHashingToCurveSha256(message));

  ASSIGN_OR_RETURN(auto ciphertext,
                   elgamal_encrypter.Encrypt(message_hashed_to_curve));
  ASSIGN_OR_RETURN(std::string u, ciphertext.u.ToBytesCompressed());
  ASSIGN_OR_RETURN(std::string e, ciphertext.e.ToBytesCompressed());

  ElGamalCiphertext result;
  result.set_u(u);
  result.set_e(e);
  return result;
}

StatusOr<std::string> Decrypt(const ElGamalCiphertext& ciphertext,
                              const ElGamalPrivateKey& private_key,
                              int curve_id) {
  Context context;
  ASSIGN_OR_RETURN(auto ec_group, ECGroup::Create(curve_id, &context));

  BigNum x = context.CreateBigNum(private_key.x());
  std::unique_ptr<PrivateKey> private_key_ptr(new PrivateKey({x}));

  ASSIGN_OR_RETURN(auto u, ec_group.CreateECPoint(ciphertext.u()));
  ASSIGN_OR_RETURN(auto e, ec_group.CreateECPoint(ciphertext.e()));

  ElGamalDecrypter elgamal_decrypter(std::move(private_key_ptr));
  ASSIGN_OR_RETURN(auto decrypted, elgamal_decrypter.Decrypt(
                                       Ciphertext{std::move(u), std::move(e)}));

  ASSIGN_OR_RETURN(std::string decrypted_bytes, decrypted.ToBytesCompressed());
  return decrypted_bytes;
}

StatusOr<ElGamalCiphertext> ExponentiateOnCiphertext(
    const ElGamalCiphertext& ciphertext, const ElGamalPublicKey& public_key,
    absl::string_view secret_exponent_str, int curve_id) {
  Context context;
  ASSIGN_OR_RETURN(auto ec_group, ECGroup::Create(curve_id, &context));

  BigNum secret_exponent = context.CreateBigNum(secret_exponent_str);
  // Re-encrypt the ciphertext with the secret EC exponent.
  // This corresponds to homomorphically exponentiating the underlying message.
  ASSIGN_OR_RETURN(auto u, ec_group.CreateECPoint(ciphertext.u()));
  ASSIGN_OR_RETURN(auto e, ec_group.CreateECPoint(ciphertext.e()));
  ASSIGN_OR_RETURN(
      auto reencrypted_ciphertext,
      Exp(Ciphertext{std::move(u), std::move(e)}, secret_exponent));

  ASSIGN_OR_RETURN(auto g, ec_group.CreateECPoint(public_key.g()));
  ASSIGN_OR_RETURN(auto y, ec_group.CreateECPoint(public_key.y()));
  auto public_key_ptr =
      absl::make_unique<PublicKey>(PublicKey({std::move(g), std::move(y)}));
  ElGamalEncrypter elgamal_encrypter(&ec_group, std::move(public_key_ptr));

  // Rerandomize the re-encrypted ciphertext after exponentiating, in order to
  // make the ciphertext indistinguishable from a fresh encryption.
  ASSIGN_OR_RETURN(auto randomized_ciphertext,
                   elgamal_encrypter.ReRandomize(reencrypted_ciphertext));

  ASSIGN_OR_RETURN(std::string ru, randomized_ciphertext.u.ToBytesCompressed());
  ASSIGN_OR_RETURN(std::string re, randomized_ciphertext.e.ToBytesCompressed());

  ElGamalCiphertext result;
  result.set_u(ru);
  result.set_e(re);
  return result;
}

StatusOr<std::string> ExponentiateOnECPointStr(
    absl::string_view ec_point_str, absl::string_view secret_exponent_str,
    int curve_id) {
  Context context;
  ASSIGN_OR_RETURN(auto ec_group, ECGroup::Create(curve_id, &context));

  ASSIGN_OR_RETURN(auto ec_point, ec_group.CreateECPoint(ec_point_str));
  BigNum secret_exponent = context.CreateBigNum(secret_exponent_str);

  ASSIGN_OR_RETURN(auto exponentiated, ec_point.Mul(secret_exponent));
  ASSIGN_OR_RETURN(std::string exponentiated_bytes,
                   exponentiated.ToBytesCompressed());
  return exponentiated_bytes;
}

}  // namespace crypto
}  // namespace convagg
