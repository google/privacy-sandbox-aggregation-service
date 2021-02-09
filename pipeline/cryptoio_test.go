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

package cryptoio

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"
)

func TestKeyGeneration(t *testing.T) {
	privDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(privDir)

	sPub, ePub, err := CreateKeysAndSecret(privDir)
	if err != nil {
		t.Fatalf("CreateKeysAndSecret() = %s", err)
	}

	pubDir, err := ioutil.TempDir("/tmp", "test-pub")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(pubDir)

	if err := SaveStandardPublicKey(pubDir, sPub); err != nil {
		t.Fatalf("SaveStandardPublicKey() = %s", err)
	}
	if err := SaveElGamalPublicKey(pubDir, ePub); err != nil {
		t.Fatalf("SaveElGamalPublicKey() = %s", err)
	}

	sPub, err = ReadStandardPublicKey(pubDir)
	if err != nil {
		t.Fatalf("ReadStandardPrivateKey() = %s", err)
	}
	ePub, err = ReadElGamalPublicKey(pubDir)
	if err != nil {
		t.Fatalf("ReadElGamalPublicKey() = %s", err)
	}

	message := "original message"
	sEncrypted, err := standardencrypt.Encrypt([]byte(message), sPub)
	if err != nil {
		t.Fatalf("standardencrypt.Encrypt() = %s", err)
	}
	eEncrypted, err := elgamalencrypt.Encrypt(message, ePub)
	if err != nil {
		t.Fatalf("elgamalencrypt.Encrypt() = %s", err)
	}

	sPriv, err := ReadStandardPrivateKey(privDir)
	if err != nil {
		t.Fatalf("ReadStandardPrivateKey() = %s", err)
	}
	ePriv, err := ReadElGamalPrivateKey(privDir)
	if err != nil {
		t.Fatalf("ReadElGamalPrivateKey() = %s", err)
	}

	sDecrypted, err := standardencrypt.Decrypt(sEncrypted, sPriv)
	if err != nil {
		t.Fatalf("standardencrypt.Decrypt() = %s", err)
	}
	if message != string(sDecrypted) {
		t.Fatalf("want standard decrypted message %s, got %s", message, string(sDecrypted))
	}
	messageHashed, err := elgamalencrypt.GetHashedECPointStrForTesting(message)
	if err != nil {
		t.Fatalf("elgamalencrypt.GetHashedECPointStrForTesting() = %s", err)
	}
	eDecrypted, err := elgamalencrypt.Decrypt(eEncrypted, ePriv)
	if err != nil {
		t.Fatalf("elgamalencrypt.Decrypt() = %s", err)
	}
	if messageHashed != eDecrypted {
		t.Fatalf("want ElGamal decrypted message %s, got %s", message, string(sDecrypted))
	}

	secret, err := ReadElGamalSecret(privDir)
	if err != nil {
		t.Fatalf("ReadElGamalSecret() = %s", err)
	}
	encryptedExp, err := elgamalencrypt.ExponentiateOnCiphertext(eEncrypted, ePub, secret)
	if err != nil {
		t.Fatalf("elgamalencrypt.ExponentiateOnCiphertext() = %s", err)
	}
	exp1, err := elgamalencrypt.Decrypt(encryptedExp, ePriv)
	if err != nil {
		t.Fatalf("elgamalencrypt.Decrypt() = %s", err)
	}
	exp2, err := elgamalencrypt.ExponentiateOnECPointStr(messageHashed, secret)
	if err != nil {
		t.Fatalf("elgamalencrypt.ExponentiateOnECPointStr() = %s", err)
	}
	if exp1 != exp2 {
		t.Fatalf("exponential results should be the same: want %s, got %s", exp1, exp2)
	}
}
