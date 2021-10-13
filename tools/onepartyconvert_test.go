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

package onepartyconvert

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartyaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

func TestEncryptReport(t *testing.T) {
	testEncryptReport(t, true /*withEncryption*/)
	testEncryptReport(t, false /*withEncryption*/)
}

func testEncryptReport(t testing.TB, withEncryption bool) {
	ctx := context.Background()
	privKeys, pubKeysInfo, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := reporttypes.RawReport{
		Bucket: uint128.From64(1),
		Value:  1,
	}

	er, err := EncryptReport(want, pubKeysInfo, nil, withEncryption)
	if err != nil {
		t.Fatal(err)
	}

	privKey, ok := privKeys[er.KeyId]
	if !ok {
		t.Fatalf("invalid encryption key ID: %s", er.KeyId)
	}

	var (
		payload *reporttypes.Payload
	)
	if withEncryption {
		payload, err = onepartyaggregator.DecryptPayload(er, privKey)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		payload, err = onepartyaggregator.ConvertPayload(er)
		if err != nil {
			t.Fatal(err)
		}
	}

	got := reporttypes.RawReport{}
	if err := json.Unmarshal(payload.DPFKey, &got); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("raw report mismatch (-want +got): \n%s", diff)
	}
}

func TestFormatEncryptedReport(t *testing.T) {
	want := &pb.EncryptedReport{
		EncryptedReport: &pb.StandardCiphertext{Data: []byte("encrypted_data")},
		ContextInfo:     []byte("context_info"),
		KeyId:           "key_id",
	}
	line, err := FormatEncryptedReport(want)
	if err != nil {
		t.Fatal(err)
	}

	got, err := onepartyaggregator.ParseEncryptedReport(line)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Fatalf("encrypted report mismatch (-want +got):\n%s", diff)
	}
}
