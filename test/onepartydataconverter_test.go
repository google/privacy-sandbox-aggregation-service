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

package onepartydataconverter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/go-cmp/cmp"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartyaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelinetypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
)

func TestAggregationPipelineOneParty(t *testing.T) {
	testAggregationPipeline(t, true /*withEncryption*/)
	testAggregationPipeline(t, false /*withEncryption*/)
}

func testAggregationPipeline(t testing.TB, withEncryption bool) {
	ctx := context.Background()
	privKeys, pubKeysInfo, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}

	type keyValue struct {
		Key   uint128.Uint128
		Value uint64
	}

	var rawReports []*pipelinetypes.RawReport
	wantSum := make(map[uint128.Uint128]uint64)
	for i := 5; i <= 20; i++ {
		for j := 0; j < i; j++ {
			index := uint128.From64(uint64(i) << 27)
			value := uint64(i)
			wantSum[index] += value
			rawReports = append(rawReports, &pipelinetypes.RawReport{Bucket: index, Value: value})
		}
	}
	var wantResult []*keyValue
	for key := range wantSum {
		wantResult = append(wantResult, &keyValue{Key: key, Value: wantSum[key]})
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	report := beam.CreateList(scope, rawReports)
	encrypted := beam.ParDo(scope, &encryptReportFn{PublicKeys: pubKeysInfo, EncryptOutput: withEncryption}, report)

	decrypted := onepartyaggregator.DecryptReport(scope, encrypted, privKeys)
	result := onepartyaggregator.SumRawReport(scope, decrypted)
	got := beam.ParDo(scope, func(index uint128.Uint128, value uint64) *keyValue {
		return &keyValue{Key: index, Value: value}
	}, result)

	want := beam.CreateList(scope, wantResult)
	passert.Equals(scope, got, want)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func testGenerateBrowserReport(t *testing.T, encryptOutput bool) {
	ctx := context.Background()
	privKeys, publicKeys, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}

	sharedInfo := "context info"
	want := pipelinetypes.RawReport{Bucket: uint128.From64(123), Value: 789}
	report, err := GenerateBrowserReport(&GenerateBrowserReportParams{
		RawReport:     want,
		PublicKeys:    publicKeys,
		SharedInfo:    sharedInfo,
		EncryptOutput: encryptOutput,
	})
	if err != nil {
		t.Fatal(err)
	}

	lines, err := report.GetSerializedEncryptedRecords()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(lines), 1; got != want {
		t.Fatalf("want %d lines, got %d", want, got)
	}

	encrypted, err := reporttypes.DeserializeAggregatablePayload(lines["0"])
	if err != nil {
		t.Fatal(err)
	}

	payload, _, err := cryptoio.DecryptOrUnmarshal(encrypted, privKeys[encrypted.KeyId])
	if err != nil {
		t.Fatal(err)
	}

	got := &pipelinetypes.RawReport{}
	if err := json.Unmarshal(payload.DPFKey, report); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resulting report mismatch (-want +got):\n%s", diff)
	}
}
