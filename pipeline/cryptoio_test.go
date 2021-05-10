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
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypttesting"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func testSaveReadKMSEncryptedStandardPrivateKey(t *testing.T) {
	privDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(privDir)

	priv, _, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	keyURI := "gcp-kms://projects/tink-test-infrastructure/locations/global/keyRings/unit-and-integration-testing/cryptoKeys/aead-key"
	credFile := "../../tink_base/testdata/credential.json"
	

	privPath := path.Join(privDir, DefaultStandardPrivateKey)
	if err := SaveKMSEncryptedStandardPrivateKey(keyURI, credFile, privPath, priv); err != nil {
		t.Fatal(err)
	}
	gotEncryptedKey, err := ioutil.ReadFile(privPath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(priv.Key, gotEncryptedKey); diff == "" {
		t.Error("Key bytes should be different after KMS encryption.")
	}

	gotPriv, err := ReadKMSDecryptedStandardPrivateKey(keyURI, credFile, privPath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(priv, gotPriv, protocmp.Transform()); diff != "" {
		t.Errorf("Saved and read private key mismatch (-want +got):\n%s", diff)
	}
}

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

	if err := SaveStandardPublicKey(path.Join(pubDir, DefaultStandardPublicKey), sPub); err != nil {
		t.Fatalf("SaveStandardPublicKey() = %s", err)
	}
	if err := SaveElGamalPublicKey(path.Join(pubDir, DefaultElgamalPublicKey), ePub); err != nil {
		t.Fatalf("SaveElGamalPublicKey() = %s", err)
	}

	sPub, err = ReadStandardPublicKey(path.Join(pubDir, DefaultStandardPublicKey))
	if err != nil {
		t.Fatalf("ReadStandardPrivateKey() = %s", err)
	}
	ePub, err = ReadElGamalPublicKey(path.Join(pubDir, DefaultElgamalPublicKey))
	if err != nil {
		t.Fatalf("ReadElGamalPublicKey() = %s", err)
	}

	message := "original message"
	sEncrypted, err := standardencrypt.Encrypt([]byte(message), nil, sPub)
	if err != nil {
		t.Fatalf("standardencrypt.Encrypt() = %s", err)
	}
	eEncrypted, err := elgamalencrypt.Encrypt(message, ePub)
	if err != nil {
		t.Fatalf("elgamalencrypt.Encrypt() = %s", err)
	}

	sPriv, err := ReadStandardPrivateKey(path.Join(privDir, DefaultStandardPrivateKey))
	if err != nil {
		t.Fatalf("ReadStandardPrivateKey() = %s", err)
	}
	ePriv, err := ReadElGamalPrivateKey(path.Join(privDir, DefaultElgamalPrivateKey))
	if err != nil {
		t.Fatalf("ReadElGamalPrivateKey() = %s", err)
	}

	sDecrypted, err := standardencrypt.Decrypt(sEncrypted, nil, sPriv)
	if err != nil {
		t.Fatalf("standardencrypt.Decrypt() = %s", err)
	}
	if message != string(sDecrypted) {
		t.Fatalf("want standard decrypted message %s, got %s", message, string(sDecrypted))
	}
	messageHashed, err := elgamalencrypttesting.GetHashedECPointStrForTesting(message)
	if err != nil {
		t.Fatalf("elgamalencrypttesting.GetHashedECPointStrForTesting() = %s", err)
	}
	eDecrypted, err := elgamalencrypt.Decrypt(eEncrypted, ePriv)
	if err != nil {
		t.Fatalf("elgamalencrypt.Decrypt() = %s", err)
	}
	if messageHashed != eDecrypted {
		t.Fatalf("want ElGamal decrypted message %s, got %s", message, string(sDecrypted))
	}

	secret, err := ReadElGamalSecret(path.Join(privDir, DefaultElgamalSecret))
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

func TestReadWriteDPFparameters(t *testing.T) {
	baseDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// wantPrefix := &pb.HierarchicalPrefixes{nil, nil, {1, 8, 10}}
	wantPrefix := &pb.HierarchicalPrefixes{
		Prefixes: []*pb.DomainPrefixes{
			{},
			{},
			{Prefix: []uint64{1, 8, 10}},
		},
	}
	prefixPath := path.Join(baseDir, "prefix.txt")

	ctx := context.Background()
	if err := SavePrefixes(ctx, prefixPath, wantPrefix); err != nil {
		t.Fatal(err)
	}
	gotPrefix, err := ReadPrefixes(ctx, prefixPath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantPrefix, gotPrefix, protocmp.Transform()); diff != "" {
		t.Errorf("prefixes read/write mismatch (-want +got):\n%s", diff)
	}

	wantParams := &pb.IncrementalDpfParameters{
		Params: []*dpfpb.DpfParameters{
			{LogDomainSize: 111, ElementBitsize: 121},
			{LogDomainSize: 222, ElementBitsize: 212},
		},
	}
	paramsPath := path.Join(baseDir, "params.txt")

	if err := SaveDPFParameters(ctx, paramsPath, wantParams); err != nil {
		t.Fatal(err)
	}
	gotParams, err := ReadDPFParameters(ctx, paramsPath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantParams, gotParams, protocmp.Transform()); diff != "" {
		t.Errorf("DPF parameters read/write mismatch (-want +got):\n%s", diff)
	}
}
