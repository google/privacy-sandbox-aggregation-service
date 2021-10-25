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
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

// TestKMSEncryptDecryptStandardPrivateKey can be tested with 'blaze run', but fails with 'blaze test'.
func testKMSEncryptDecryptStandardPrivateKey(t *testing.T) {
	wantKey, _, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	keyURI := "gcp-kms://projects/tink-test-infrastructure/locations/global/keyRings/unit-and-integration-testing/cryptoKeys/aead-key"
	credFile := "../../tink_base/testdata/credential.json"

	ctx := context.Background()
	encrypted, err := KMSEncryptData(ctx, keyURI, credFile, wantKey.Key)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantKey.Key, encrypted); diff == "" {
		t.Error("Key bytes should be different after KMS encryption.")
	}

	decrypted, err := KMSDecryptData(ctx, keyURI, credFile, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantKey, &pb.StandardPrivateKey{Key: decrypted}, protocmp.Transform()); diff != "" {
		t.Errorf("Decrypted and original private key mismatch (-want +got):\n%s", diff)
	}
}

func TestReadWriteDPFparameters(t *testing.T) {
	baseDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	// wantPrefix := &pb.HierarchicalPrefixes{nil, nil, {1, 8, 10}}
	wantPrefix := [][]uint128.Uint128{
		{},
		{},
		{uint128.From64(1), uint128.From64(2)},
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

func TestSaveReadPublicKeyVersions(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "keys")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	want := map[string][]PublicKeyInfo{
		"version1": {
			{ID: "id11", Key: "key11", NotBefore: "not_before1", NotAfter: "not_after1"},
			{ID: "id12", Key: "key12", NotBefore: "not_before2", NotAfter: "not_after2"},
		},
		"version2": {{ID: "id21", Key: "key21"}},
	}

	ctx := context.Background()
	for _, tc := range []struct {
		desc, filePath string
	}{
		{"file-path", path.Join(tmpDir, "keys")},
		{"env-var", ""},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if err := SavePublicKeyVersions(ctx, want, tc.filePath); err != nil {
				t.Fatal(err)
			}
			got, err := ReadPublicKeyVersions(ctx, tc.filePath)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("read/write versioned public keys mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSaveReadPrivateKeyParamsCollection(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "key_params")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	want := map[string]*ReadStandardPrivateKeyParams{
		"key_id_1": {
			KMSKeyURI:         "kms_key_uri_1",
			KMSCredentialPath: "kms_credential_path",
			SecretName:        "secret_name_1",
			FilePath:          "file_path_1"},
		"key_id_2": {
			SecretName: "secret_name_2",
			FilePath:   "file_path_2"},
	}
	ctx := context.Background()
	filePath := path.Join(tmpDir, "key_params")
	if err := SavePrivateKeyParamsCollection(ctx, want, filePath); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPrivateKeyParamsCollection(ctx, filePath)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("read/write private key parameters mismatch (-want +got):\n%s", diff)
	}
}
