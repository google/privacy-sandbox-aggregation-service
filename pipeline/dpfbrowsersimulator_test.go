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

package dpfbrowsersimulator

import (
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"
)

func createConversionsDpf(n int) ([]rawConversion, []dpfaggregator.CompleteHistogram) {
	var conversions []rawConversion
	var results []dpfaggregator.CompleteHistogram
	for i := 0; i < (n+9)/10; i++ {
		sum := 0
		for j := 0; j < 10; j++ {
			if len(conversions) >= n {
				continue
			}
			conversions = append(conversions, rawConversion{Index: uint64(i), Value: uint64(1)})
			sum++
		}
		results = append(results, dpfaggregator.CompleteHistogram{Index: uint64(i), Sum: uint64(sum), Count: uint64(sum)})

		if len(conversions) >= n {
			break
		}
	}
	return conversions, results
}

func TestAggregationPipelineDPF(t *testing.T) {
	testAggregationPipelineDPF(t)
}

func testAggregationPipelineDPF(t testing.TB) {
	privKey1, pubKey1, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	privKey2, pubKey2, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	conversionArray, wantArray := createConversionsDpf(100)

	pipeline, scope := beam.NewPipelineWithRoot()

	logN := uint64(10)
	logElementSizeSum := uint64(6)
	logElementSizeCount := uint64(6)

	conversions := beam.CreateList(scope, conversionArray)
	want := beam.CreateList(scope, wantArray)
	ePr1, ePr2 := splitRawConversion(scope, conversions, &GeneratePartialReportParams{
		LogN:                logN,
		LogElementSizeSum:   logElementSizeSum,
		LogElementSizeCount: logElementSizeCount,
		PublicKey1:          pubKey1,
		PublicKey2:          pubKey2,
	})

	pr1 := dpfaggregator.DecryptPartialReport(scope, ePr1, privKey1)
	pr2 := dpfaggregator.DecryptPartialReport(scope, ePr2, privKey2)

	aggregateParams := &dpfaggregator.AggregatePartialReportParams{
		LogN:                logN,
		LogElementSizeSum:   logElementSizeSum,
		LogElementSizeCount: logElementSizeCount,
		LogSegmentLength:    7,
		DirectCombine:       true,
	}
	ph1 := dpfaggregator.ExpandAndCombineHistogram(scope, pr1, aggregateParams)
	ph2 := dpfaggregator.ExpandAndCombineHistogram(scope, pr2, aggregateParams)

	got := dpfaggregator.MergeHistogram(scope, ph1, ph2)

	passert.Equals(scope, got, want)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func BenchmarkPipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testAggregationPipelineDPF(b)
	}
}
