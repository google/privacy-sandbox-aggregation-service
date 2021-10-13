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

package dpfconvert

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

const keyBitSize = 5

func TestGenerateEncryptedReports(t *testing.T) {
	testGenerateEncryptedReports(t, true /*withEncryption*/)
	testGenerateEncryptedReports(t, false /*withEncryption*/)
}

func testGenerateEncryptedReports(t testing.TB, withEncryption bool) {
	ctx := context.Background()
	privKeys, pubKeysInfo, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	report := reporttypes.RawReport{
		Bucket: uint128.From64(1),
		Value:  1,
	}
	want := map[int]uint64{1: 1}
	epr1, epr2, err := GenerateEncryptedReports(report, keyBitSize, pubKeysInfo, pubKeysInfo, nil, withEncryption)
	if err != nil {
		t.Fatal(err)
	}

	privKey1, ok := privKeys[epr1.KeyId]
	if !ok {
		t.Fatalf("invalid encryption key ID: %s", epr1.KeyId)
	}
	privKey2, ok := privKeys[epr2.KeyId]
	if !ok {
		t.Fatalf("invalid encryption key ID: %s", epr2.KeyId)
	}

	var (
		payload1, payload2 *reporttypes.Payload
	)
	if withEncryption {
		payload1, err = dpfaggregator.DecryptPayload(epr1, privKey1)
		if err != nil {
			t.Fatal(err)
		}
		payload2, err = dpfaggregator.DecryptPayload(epr2, privKey2)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		payload1, err = dpfaggregator.ConvertPayload(epr1)
		if err != nil {
			t.Fatal(err)
		}
		payload2, err = dpfaggregator.ConvertPayload(epr2)
		if err != nil {
			t.Fatal(err)
		}
	}
	dpfKey1 := &dpfpb.DpfKey{}
	if err := proto.Unmarshal(payload1.DPFKey, dpfKey1); err != nil {
		t.Fatal(err)
	}
	dpfKey2 := &dpfpb.DpfKey{}
	if err := proto.Unmarshal(payload2.DPFKey, dpfKey2); err != nil {
		t.Fatal(err)
	}

	dpfParams, err := cryptoio.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	evalCtx1, err := incrementaldpf.CreateEvaluationContext(dpfParams, dpfKey1)
	if err != nil {
		t.Fatal(err)
	}
	evalCtx2, err := incrementaldpf.CreateEvaluationContext(dpfParams, dpfKey2)
	if err != nil {
		t.Fatal(err)
	}

	result1, err := incrementaldpf.EvaluateUntil64(keyBitSize-1, nil, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}
	result2, err := incrementaldpf.EvaluateUntil64(keyBitSize-1, nil, evalCtx2)
	if err != nil {
		t.Fatal(err)
	}
	if len(result1) != len(result2) {
		t.Fatalf("expect two results with the same length, got %d and %d", len(result1), len(result2))
	}

	got := make(map[int]uint64)
	for i := range result1 {
		c := result1[i] + result2[i]
		if c != 0 {
			got[i] = c
		}
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("merged results mismatch with expected (-want +got): \n%s", diff)
	}
}

func TestFormatEncryptedPartialReport(t *testing.T) {
	want := &pb.EncryptedPartialReportDpf{
		EncryptedReport: &pb.StandardCiphertext{Data: []byte("encrypted_data")},
		ContextInfo:     []byte("context_info"),
		KeyId:           "key_id",
	}
	line, err := FormatEncryptedPartialReport(want)
	if err != nil {
		t.Fatal(err)
	}

	got, err := dpfaggregator.ParseEncryptedPartialReport(line)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Fatalf("encrypted report mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateConversionIndex(t *testing.T) {
	prefixes := []uint128.Uint128{
		uint128.From64(1),
		uint128.From64(2),
		uint128.From64(3),
	}
	prefixeBitSize := uint64(2)
	prefixesStr := make(map[string]bool)
	for _, v := range prefixes {
		prefixesStr[v.String()] = true
	}

	totalBitSize := uint64(5)
	got, err := CreateConversionIndex(prefixes, prefixeBitSize, totalBitSize, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := prefixesStr[got.Rsh(uint(totalBitSize-prefixeBitSize)).String()]; !ok {
		t.Fatalf("expect number with one of the prefixes in %+v, got %s", prefixes, got.String())
	}

	got, err = CreateConversionIndex(prefixes, prefixeBitSize, totalBitSize, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := prefixesStr[got.Rsh(uint(totalBitSize-prefixeBitSize)).String()]; ok {
		t.Fatalf("expect number without any of the prefixes in %+v, got %s", prefixes, got.String())
	}
}
