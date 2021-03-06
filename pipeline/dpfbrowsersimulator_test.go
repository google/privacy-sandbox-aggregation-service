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
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	
	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"

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
	WantResults [][]dpfaggregator.CompleteHistogram
	SumParams   *pb.IncrementalDpfParameters
	Prefixes    *pb.HierarchicalPrefixes
}

// filterResults filters the expected aggregation results with given prefixes to get the expected results at certain prefix length.
// The results will be used to verify the aggregation output at different hierarchy.
func filterResults(prefixes []uint64, prefixBitSize, keyBitSize, totalBitSize uint64, allResults map[uint64]uint64) []dpfaggregator.CompleteHistogram {
	prefixesSet := make(map[uint64]bool)
	for _, p := range prefixes {
		prefixesSet[p] = true
	}
	results := make(map[uint64]uint64)
	for k, v := range allResults {
		prefix := k >> (totalBitSize - prefixBitSize)
		key := k >> (totalBitSize - keyBitSize)
		if _, ok := prefixesSet[prefix]; ok || len(prefixesSet) == 0 {
			results[key] += v
		}
	}

	var histogram []dpfaggregator.CompleteHistogram
	for k, v := range results {
		histogram = append(histogram, dpfaggregator.CompleteHistogram{Index: k, Sum: v})
	}
	return histogram
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
		aggResult[index]++
	}

	var wantResults [][]dpfaggregator.CompleteHistogram
	for i := 0; i < len(prefixes.Prefixes); i++ {
		prefixBits := int32(logN)
		if i > 0 {
			prefixBits = sumParams.Params[i-1].LogDomainSize
		}
		keyBits := sumParams.Params[i].LogDomainSize
		wantResults = append(wantResults, filterResults(prefixes.Prefixes[i].Prefix, uint64(prefixBits), uint64(keyBits), logN, aggResult))
	}

	return &dpfTestData{Conversions: conversions, WantResults: wantResults, SumParams: sumParams, Prefixes: prefixes}, nil
}

func TestAggregationPipelineDPF(t *testing.T) {
	testAggregationPipelineDPF(t)
}

func testAggregationPipelineDPF(t testing.TB) {
	ctx := context.Background()
	privKeys1, pubKeysInfo1, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	privKeys2, pubKeysInfo2, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}

	const keyBitSize = 20
	testData, err := createConversionsDpf(keyBitSize, 6, 6)
	if err != nil {
		t.Fatal(err)
	}

	params := &pb.IncrementalDpfParameters{}
	prefixes := &pb.HierarchicalPrefixes{}
	for i := range testData.Prefixes.Prefixes {
		params.Params = append(params.Params, testData.SumParams.Params[i])
		prefixes.Prefixes = append(prefixes.Prefixes, testData.Prefixes.Prefixes[i])

		pipeline, scope := beam.NewPipelineWithRoot()
		conversions := beam.CreateList(scope, testData.Conversions)
		want := beam.CreateList(scope, testData.WantResults[i])
		ePr1, ePr2 := splitRawConversion(scope, conversions, &GeneratePartialReportParams{
			PublicKeys1: pubKeysInfo1,
			PublicKeys2: pubKeysInfo2,
			KeyBitSize:  keyBitSize,
		})

		pr1 := dpfaggregator.DecryptPartialReport(scope, ePr1, privKeys1)
		pr2 := dpfaggregator.DecryptPartialReport(scope, ePr2, privKeys2)

		aggregateParams := &dpfaggregator.AggregatePartialReportParams{
			SumParameters: params,
			Prefixes:      prefixes,
			DirectCombine: true,
			KeyBitSize:    keyBitSize,
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
}

func BenchmarkPipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testAggregationPipelineDPF(b)
	}
}

func TestWriteReadPartialReports(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	want := []*pb.EncryptedPartialReportDpf{
		{
			EncryptedReport: &pb.StandardCiphertext{Data: []byte("encrypted1")},
			ContextInfo:     []byte("context1"),
		},
		{
			EncryptedReport: &pb.StandardCiphertext{Data: []byte("encrypted2")},
			ContextInfo:     []byte("context2"),
		},
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	wantList := beam.CreateList(scope, want)
	filename := path.Join(tmpDir, "partial.txt")
	writePartialReport(scope, wantList, filename, 1)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}

	gotList := dpfaggregator.ReadPartialReport(scope, filename)
	passert.Equals(scope, gotList, wantList)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}
