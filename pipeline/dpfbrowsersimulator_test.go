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
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func TestReadInputConversions(t *testing.T) {
	var conversions []RawConversion
	for i := 5; i <= 20; i++ {
		for j := 0; j < i; j++ {
			conversions = append(conversions, RawConversion{Index: uint64(i), Value: uint64(i)})
		}
	}

	testFile := "dpf_test_conversion_data.csv"
	
	pipeline, scope := beam.NewPipelineWithRoot()
	lines := textio.ReadSdf(scope, testFile)
	got := beam.ParDo(scope, &parseRawConversionFn{}, lines)
	want := beam.CreateList(scope, conversions)

	passert.Equals(scope, got, want)
	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type dpfTestData struct {
	Conversions []RawConversion
	WantResults []dpfaggregator.CompleteHistogram
	SumParams   *pb.IncrementalDpfParameters
	Prefixes    *pb.HierarchicalPrefixes
}

func createConversionsDpf(logN, logElementSizeSum, totalCount uint64) (*dpfTestData, error) {
	root := &PrefixNode{Class: "root"}
	for i := 0; i < 1<<5; i++ {
		root.AddChildNode("campaignid", 12 /*bitSize*/, uint64(i) /*value*/)
	}

	prefixes, prefixDomainBits := CalculatePrefixes(root)
	sumParams := CalculateParameters(prefixDomainBits, int32(logN), 1<<logElementSizeSum)

	aggResult := make(map[uint64]uint64)
	var conversions []RawConversion
	// Generate conversions with indices that contain one of the given prefixes. These conversions are counted in the aggregation.
	prefixesLen := len(prefixes.Prefixes)
	for i := uint64(0); i < totalCount-4; i++ {
		index, err := CreateConversionIndex(prefixes.Prefixes[prefixesLen-1].Prefix, prefixDomainBits[len(prefixDomainBits)-1], logN, true /*hasPrefix*/)
		if err != nil {
			return nil, err
		}
		conversions = append(conversions, RawConversion{Index: index, Value: 1})
		aggResult[index]++
	}
	// Generate conversions with indices that do not contain any of the given prefixes. These conversions will not be counted in the aggregation.
	for i := uint64(0); i < 4; i++ {
		index, err := CreateConversionIndex(prefixes.Prefixes[prefixesLen-1].Prefix, prefixDomainBits[len(prefixDomainBits)-1], logN, false /*hasPrefix*/)
		if err != nil {
			return nil, err
		}
		conversions = append(conversions, RawConversion{Index: index, Value: 1})
	}

	var wantResults []dpfaggregator.CompleteHistogram
	for k, v := range aggResult {
		wantResults = append(wantResults, dpfaggregator.CompleteHistogram{Index: k, Sum: v})
	}

	return &dpfTestData{Conversions: conversions, WantResults: wantResults, SumParams: sumParams, Prefixes: prefixes}, nil
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

	testData, err := createConversionsDpf(20, 6, 6)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, scope := beam.NewPipelineWithRoot()

	conversions := beam.CreateList(scope, testData.Conversions)
	want := beam.CreateList(scope, testData.WantResults)
	ePr1, ePr2 := splitRawConversion(scope, conversions, &GeneratePartialReportParams{
		SumParameters: testData.SumParams,
		PublicKey1:    pubKey1,
		PublicKey2:    pubKey2,
	})

	pr1 := dpfaggregator.DecryptPartialReport(scope, ePr1, privKey1)
	pr2 := dpfaggregator.DecryptPartialReport(scope, ePr2, privKey2)

	aggregateParams := &dpfaggregator.AggregatePartialReportParams{
		SumParameters: testData.SumParams,
		Prefixes:      testData.Prefixes,
		DirectCombine: true,
	}
	ph1, err := dpfaggregator.ExpandAndCombineHistogram(scope, pr1, aggregateParams)
	if err != nil {
		t.Fatal(err)
	}
	ph2, err := dpfaggregator.ExpandAndCombineHistogram(scope, pr2, aggregateParams)
	if err != nil {
		t.Fatal(err)
	}

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
