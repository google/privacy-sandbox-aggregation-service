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

package dpfdataconverter

import (
	"context"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reporttypes"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

func generateLargePrefixes(count int) []uint128.Uint128 {
	output := make([]uint128.Uint128, count)
	for i := 0; i < count; i++ {
		output[i] = uint128.From64(uint64(i))
	}
	return output
}

func generateConversions(count int) []reporttypes.RawReport {
	output := make([]reporttypes.RawReport, count)
	for i := 0; i < count; i++ {
		output[i] = reporttypes.RawReport{Bucket: uint128.From64(1), Value: 1}
	}
	return output
}

func TestAggregationPipelineDPF(t *testing.T) {
	testAggregationPipelineDPF(t, true /*withEncryption*/)
}

func testAggregationPipelineDPF(t testing.TB, withEncryption bool) {
	ctx := context.Background()
	privKeys1, pubKeysInfo1, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	privKeys2, pubKeysInfo2, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}

	const keyBitSize = 128
	ctxParams, err := dpfaggregator.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	combineParams := &dpfaggregator.CombineParams{
		DirectCombine: false,
		SegmentLength: 32768,
	}

	reports := generateConversions(10)
	prefixes := generateLargePrefixes(1048576)

	expandParams := &dpfaggregator.ExpandParameters{
		Prefixes:      [][]uint128.Uint128{prefixes},
		Levels:        []int32{127},
		PreviousLevel: -1,
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	conversions := beam.CreateList(scope, reports)
	buckets := beam.Create(scope, prefixes)

	ePr1, ePr2 := splitRawConversion(scope, conversions, &GeneratePartialReportParams{
		PublicKeys1:   pubKeysInfo1,
		PublicKeys2:   pubKeysInfo2,
		KeyBitSize:    keyBitSize,
		EncryptOutput: withEncryption,
	})

	pr1 := dpfaggregator.DecryptPartialReport(scope, ePr1, privKeys1)
	pr2 := dpfaggregator.DecryptPartialReport(scope, ePr2, privKeys2)

	ctx1 := dpfaggregator.CreateEvaluationContext(scope, pr1, expandParams, ctxParams)
	ctx2 := dpfaggregator.CreateEvaluationContext(scope, pr2, expandParams, ctxParams)

	dpfaggregator.ExpandAtAndCombineHistogram(scope, ctx1, buckets, expandParams, ctxParams, combineParams)
	dpfaggregator.ExpandAtAndCombineHistogram(scope, ctx2, buckets, expandParams, ctxParams, combineParams)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func BenchmarkPipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testAggregationPipelineDPF(b, true /*withEncryption*/)
	}
}
