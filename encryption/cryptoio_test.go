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
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

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
			{LogDomainSize: 111,
				ValueType: &dpfpb.ValueType{
					Type: &dpfpb.ValueType_Integer_{
						Integer: &dpfpb.ValueType_Integer{
							Bitsize: 121,
						},
					},
				},
			},
			{LogDomainSize: 222,
				ValueType: &dpfpb.ValueType{
					Type: &dpfpb.ValueType_Integer_{
						Integer: &dpfpb.ValueType_Integer{
							Bitsize: 212,
						},
					},
				},
			},
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

func TestDecryptOrUnmarshal(t *testing.T) {
	testDecryptOrUnmarshal(t, true /*encryptOutput*/)
	testDecryptOrUnmarshal(t, false /*encryptOutput*/)
}

func testDecryptOrUnmarshal(t *testing.T, encryptOutput bool) {
	priv, pub, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	want := &reporttypes.Payload{
		Operation: "some operation",
		DPFKey:    []byte("some key"),
	}
	data, err := utils.MarshalCBOR(want)
	if err != nil {
		t.Fatal(err)
	}

	contextInfo := "some context"
	if encryptOutput {
		encrypted, err := standardencrypt.Encrypt(data, []byte(contextInfo), pub)
		if err != nil {
			t.Fatal(err)
		}
		data = encrypted.Data
	}

	got, isEncrypted, err := DecryptOrUnmarshal(&pb.AggregatablePayload{
		Payload:    &pb.StandardCiphertext{Data: data},
		SharedInfo: contextInfo,
	}, priv)
	if err != nil {
		t.Fatal(err)
	}

	if isEncrypted != encryptOutput {
		t.Fatalf("expect isEncrypted = %t, got %t", encryptOutput, isEncrypted)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("decrypted payload mismatch (-want +got):\n%s", diff)
	}
}

func TestExtractPayloadsFromAggregatableReportOnepartyNoEncryption(t *testing.T) {
	// The message generated by the test binary with contribution to a single bucket: <1234, 5>.
	message := "{\"aggregation_service_payloads\":[{\"key_id\":\"id123\",\"payload\":\"omRkYXRhgaJldmFsdWVEAAAABWZidWNrZXRQAAAAAAAAAAAAAAAAAAAE0mlvcGVyYXRpb25paGlzdG9ncmFt\"}],\"shared_info\":\"{\\\"privacy_budget_key\\\":\\\"test_privacy_budget_key\\\",\\\"report_id\\\":\\\"c0eb0114-71ab-4811-9ae8-0c9eef442452\\\",\\\"reporting_origin\\\":\\\"https://example.com\\\",\\\"scheduled_report_time\\\":\\\"1648488303\\\",\\\"version\\\":\\\"\\\"}\"}"
	clientReport := &reporttypes.AggregatableReport{}
	if err := json.Unmarshal([]byte(message), clientReport); err != nil {
		t.Fatal(err)
	}

	helperReports, err := clientReport.ExtractPayloadsFromAggregatableReport(false /*useCleartext*/)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 1, len(helperReports); want != got {
		t.Errorf("want %d payloads, got %d", want, got)
	}

	gotPayload, encrypted, err := DecryptOrUnmarshal(helperReports[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	if want, got := false, encrypted; want != got {
		t.Errorf("want encrypted = %v, got %v", want, got)
	}

	wantPayload := &reporttypes.Payload{
		Operation: "histogram",
		Data: []reporttypes.Contribution{
			{
				Bucket: utils.Uint128ToBigEndianBytes(uint128.From64(1234)),
				Value:  utils.Uint32ToBigEndianBytes(uint32(5)),
			},
		},
	}

	if diff := cmp.Diff(wantPayload, gotPayload); diff != "" {
		t.Errorf("payload mismatch (-want +got):\n%s", diff)
	}
}

func expandDpfKeyInReport(report *pb.AggregatablePayload, privateKey *pb.StandardPrivateKey, buckets []uint128.Uint128, keyBitSize int) ([]uint64, error) {
	payload, _, err := DecryptOrUnmarshal(report, privateKey)
	if err != nil {
		return nil, err
	}

	dpfKey := &dpfpb.DpfKey{}
	if err := proto.Unmarshal(payload.DPFKey, dpfKey); err != nil {
		return nil, err
	}

	params, err := incrementaldpf.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		return nil, err
	}
	return incrementaldpf.EvaluateAt64(params, keyBitSize-1, buckets, dpfKey)
}

func TestExtractPayloadsFromAggregatableReportMPCNoEncryption(t *testing.T) {
	// The message generated by the test binary with contribution to a single bucket: <1234, 5>.
	message := "{\"aggregation_service_payloads\":[{\"key_id\":\"id123\",\"payload\":\"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\"},{\"key_id\":\"id123\",\"payload\":\"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\"}],\"shared_info\":\"{\\\"privacy_budget_key\\\":\\\"test_privacy_budget_key\\\",\\\"report_id\\\":\\\"90955186-a443-4545-8f1e-6a8e53fa5e96\\\",\\\"reporting_origin\\\":\\\"https://example.com\\\",\\\"scheduled_report_time\\\":\\\"1648577619\\\",\\\"version\\\":\\\"\\\"}\"}"
	clientReport := &reporttypes.AggregatableReport{}
	if err := json.Unmarshal([]byte(message), clientReport); err != nil {
		t.Fatal(err)
	}

	helperReports, err := clientReport.ExtractPayloadsFromAggregatableReport(false /*useCleartext*/)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 2, len(helperReports); want != got {
		t.Errorf("want %d payloads, got %d", want, got)
	}

	keyBitSize := 32
	buckets := []uint128.Uint128{uint128.From64(1234), uint128.From64(5678)}
	result0, err := expandDpfKeyInReport(helperReports[0], nil, buckets, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	result1, err := expandDpfKeyInReport(helperReports[1], nil, buckets, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	if len0, len1 := len(result0), len(result1); len0 != len1 {
		t.Fatalf("expect the resulted arrays with the same length, got %d and %d", len0, len1)
	}

	var got []uint64
	for i := range result0 {
		got = append(got, result0[i]+result1[i])
	}
	if diff := cmp.Diff([]uint64{5, 0}, got); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}
