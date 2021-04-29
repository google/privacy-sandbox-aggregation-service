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

package standardencrypt

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestRandomKeySecretGeneration(t *testing.T) {
	priv1, pub1, err := GenerateStandardKeyPair()
	if err != nil {
		t.Fatalf("GenerateStandardKeyPair() = %s", err)
	}
	priv2, pub2, err := GenerateStandardKeyPair()
	if err != nil {
		t.Fatalf("GenerateStandardKeyPair() = %s", err)
	}

	if cmp.Equal(priv1, priv2, protocmp.Transform()) {
		t.Fatalf("duplicated private keys")
	}
	if cmp.Equal(pub1, pub2, protocmp.Transform()) {
		t.Fatalf("duplicated public keys")
	}
}

func TestEncryptAndDecrypt(t *testing.T) {
	priv, pub, err := GenerateStandardKeyPair()
	if err != nil {
		t.Fatalf("GenerateStandardKeyPair() = %s", err)
	}
	message1 := "Message 1"
	message2 := "Message 2"

	encrypted1, err := Encrypt([]byte(message1), nil, pub)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message1, err)
	}
	encrypted2, err := Encrypt([]byte(message2), nil, pub)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message2, err)
	}

	if cmp.Equal(encrypted1, encrypted2, protocmp.Transform()) {
		t.Fatalf("same encrypted results for different messages %s and %s", message1, message2)
	}

	encryptedAgain, err := Encrypt([]byte(message1), nil, pub)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message1, err)
	}
	if cmp.Equal(encrypted1, encryptedAgain, protocmp.Transform()) {
		t.Fatalf("same encrypted results for the same messages %s", message1)
	}

	decrypted, err := Decrypt(encrypted1, nil, priv)
	if err != nil {
		t.Fatalf("Decrypt(%s, %s) = %s", encrypted1.String(), priv.String(), err)
	}
	if message1 != string(decrypted) {
		t.Fatalf("want decrypted message %s, got %s", message1, decrypted)
	}
}

func TestHybridEncryptAndDecrypt(t *testing.T) {
	priv, pub, err := GenerateStandardKeyPair()
	if err != nil {
		t.Fatalf("GenerateStandardKeyPair() = %s", err)
	}
	message := "Message"
	context := "context info"

	encrypted, err := Encrypt([]byte(message), []byte(context), pub)
	if err != nil {
		t.Fatalf("Encrypt(%s) = %s", message, err)
	}

	decrypted, err := Decrypt(encrypted, []byte(context), priv)
	if err != nil {
		t.Fatalf("Decrypt(%s, %s) = %s", encrypted.String(), priv.String(), err)
	}
	if message != string(decrypted) {
		t.Fatalf("want decrypted message %s, got %s", message, decrypted)
	}
}
