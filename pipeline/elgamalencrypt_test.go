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

package elgamalencrypt

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypttesting"
)

func TestRandomKeySecretGeneration(t *testing.T) {
	priv1, pub1, err := GenerateElGamalKeyPair()
	if err != nil {
		t.Fatalf("GenerateElGamalKeyPair() = %s", err)
	}
	priv2, pub2, err := GenerateElGamalKeyPair()
	if err != nil {
		t.Fatalf("GenerateElGamalKeyPair() = %s", err)
	}

	if cmp.Equal(priv1, priv2, protocmp.Transform()) {
		t.Fatalf("duplicated private keys")
	}
	if cmp.Equal(pub1, pub2, protocmp.Transform()) {
		t.Fatalf("duplicated public keys")
	}

	secret1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() = %s", err)
	}
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() = %s", err)
	}

	if secret1 == secret2 {
		t.Fatalf("duplicated secrets")
	}
}

func TestEncryptAndDecrypt(t *testing.T) {
	priv, pub, err := GenerateElGamalKeyPair()
	if err != nil {
		t.Fatalf("GenerateElGamalKeyPair() = %s", err)
	}
	message1 := "Message 1"
	message2 := "Message 2"

	encrypted1, err := Encrypt(message1, pub)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message1, err)
	}
	encrypted2, err := Encrypt(message2, pub)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message2, err)
	}

	if cmp.Equal(encrypted1, encrypted2, protocmp.Transform()) {
		t.Fatalf("same encrypted results for different messages %s and %s", message1, message2)
	}

	encryptedAgain, err := Encrypt(message1, pub)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message1, err)
	}
	if cmp.Equal(encrypted1, encryptedAgain, protocmp.Transform()) {
		t.Fatalf("same encrypted results for the same messages %s", message1)
	}

	hashedMessage, err := elgamalencrypttesting.GetHashedECPointStrForTesting(message1)
	if err != nil {
		t.Fatalf("elgamalencrypttesting.GetHashedECPointStrForTesting(%s) = %s", message1, err)
	}
	decrypted, err := Decrypt(encrypted1, priv)
	if err != nil {
		t.Fatalf("Decrypt(%s, %s) = %s", encrypted1.String(), priv.String(), err)
	}
	if hashedMessage != decrypted {
		t.Fatalf("want decrypted message %s, got %s", hashedMessage, decrypted)
	}
}

func TestCrossEncryptAndDecrypt(t *testing.T) {
	priv1, pub1, err := GenerateElGamalKeyPair()
	if err != nil {
		t.Fatalf("GenerateElGamalKeyPair() = %s", err)
	}
	secret1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() = %s", err)
	}

	priv2, pub2, err := GenerateElGamalKeyPair()
	if err != nil {
		t.Fatalf("GenerateElGamalKeyPair() = %s", err)
	}
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() = %s", err)
	}

	message := "Message"
	encrypted1, err := Encrypt(message, pub1)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message, err)
	}
	encrypted2, err := Encrypt(message, pub2)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message, err)
	}

	exponentiated12, err := ExponentiateOnCiphertext(encrypted1, pub1, secret2)
	if err != nil {
		t.Fatalf("ExponentiateOnCiphertext() = %s", err)
	}

	exponentiated21, err := ExponentiateOnCiphertext(encrypted2, pub2, secret1)
	if err != nil {
		t.Fatalf("ExponentiateOnCiphertext() = %s", err)
	}

	decrypted12, err := Decrypt(exponentiated12, priv1)
	if err != nil {
		t.Fatalf("Decrypt() = %s", err)
	}

	decrypted21, err := Decrypt(exponentiated21, priv2)
	if err != nil {
		t.Fatalf("Decrypt() = %s", err)
	}

	reExponentiated121, err := ExponentiateOnECPointStr(decrypted12, secret1)
	if err != nil {
		t.Fatalf("ExponentiateOnECPointStr() = %s", err)
	}

	reExponentiated212, err := ExponentiateOnECPointStr(decrypted21, secret2)
	if err != nil {
		t.Fatalf("ExponentiateOnECPointStr() = %s", err)
	}

	if reExponentiated121 != reExponentiated212 {
		t.Fatalf("want same results %s, got %s", reExponentiated121, reExponentiated212)
	}
}
