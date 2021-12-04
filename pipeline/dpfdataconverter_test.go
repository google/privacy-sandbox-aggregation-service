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
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reporttypes"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func TestReadInputConversions(t *testing.T) {
	var conversions []reporttypes.RawReport
	for i := 5; i <= 20; i++ {
		for j := 0; j < i; j++ {
			conversions = append(conversions, reporttypes.RawReport{Bucket: uint128.From64(uint64(i)).Lsh(27), Value: uint64(i)})
		}
	}

	testFile, err := ioutils.RunfilesPath("pipeline/dpf_test_conversion_data.csv", false /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	pipeline, scope := beam.NewPipelineWithRoot()
	lines := textio.Read(scope, testFile)
	got := beam.ParDo(scope, &parseRawConversionFn{KeyBitSize: 32}, lines)
	want := beam.CreateList(scope, conversions)

	passert.Equals(scope, got, want)
	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type dpfTestData struct {
	Conversions []reporttypes.RawReport
	WantResults [][]dpfaggregator.CompleteHistogram
	SumParams   *pb.IncrementalDpfParameters
	Prefixes    [][]uint128.Uint128
}

// filterResults filters the expected aggregation results with given prefixes to get the expected results at certain prefix length.
// The results will be used to verify the aggregation output at different hierarchy.
func filterResults(prefixes []uint128.Uint128, prefixBitSize, keyBitSize, totalBitSize uint64, allResults map[uint128.Uint128]uint64) []dpfaggregator.CompleteHistogram {
	prefixesSet := make(map[uint128.Uint128]bool)
	for _, p := range prefixes {
		prefixesSet[p] = true
	}
	results := make(map[uint128.Uint128]uint64)
	for k, v := range allResults {
		prefix := k.Rsh(uint(totalBitSize - prefixBitSize))
		key := k.Rsh(uint(totalBitSize - keyBitSize))
		if _, ok := prefixesSet[prefix]; ok || len(prefixesSet) == 0 {
			results[key] += v
		}
	}

	var histogram []dpfaggregator.CompleteHistogram
	for k, v := range results {
		histogram = append(histogram, dpfaggregator.CompleteHistogram{Bucket: k, Sum: v})
	}
	return histogram
}

func createConversionsDpf(logN, logElementSizeSum, totalCount uint64) (*dpfTestData, error) {
	root := &PrefixNode{Class: "root"}
	for i := 0; i < 1<<5; i++ {
		root.AddChildNode("campaignid", 12 /*bitSize*/, uint128.From64(uint64(i)) /*value*/)
	}

	prefixes, prefixDomainBits := CalculatePrefixes(root)
	sumParams := CalculateParameters(prefixDomainBits, int32(logN), 1<<logElementSizeSum)

	aggResult := make(map[uint128.Uint128]uint64)
	var conversions []reporttypes.RawReport
	// Generate conversions with indices that contain one of the given prefixes. These conversions are counted in the aggregation.
	prefixesLen := len(prefixes)
	for i := uint64(0); i < totalCount-4; i++ {
		index, err := CreateConversionIndex(prefixes[prefixesLen-1], prefixDomainBits[len(prefixDomainBits)-1], logN, true /*hasPrefix*/)
		if err != nil {
			return nil, err
		}
		conversions = append(conversions, reporttypes.RawReport{Bucket: index, Value: 1})
		aggResult[index]++
	}
	// Generate conversions with indices that do not contain any of the given prefixes. These conversions will not be counted in the aggregation.
	for i := uint64(0); i < 4; i++ {
		index, err := CreateConversionIndex(prefixes[prefixesLen-1], prefixDomainBits[len(prefixDomainBits)-1], logN, false /*hasPrefix*/)
		if err != nil {
			return nil, err
		}
		conversions = append(conversions, reporttypes.RawReport{Bucket: index, Value: 1})
		aggResult[index]++
	}

	var wantResults [][]dpfaggregator.CompleteHistogram
	for i := 0; i < len(prefixes); i++ {
		prefixBits := int32(logN)
		if i > 0 {
			prefixBits = sumParams.Params[i-1].LogDomainSize
		}
		keyBits := sumParams.Params[i].LogDomainSize
		wantResults = append(wantResults, filterResults(prefixes[i], uint64(prefixBits), uint64(keyBits), logN, aggResult))
	}

	return &dpfTestData{Conversions: conversions, WantResults: wantResults, SumParams: sumParams, Prefixes: prefixes}, nil
}

func TestAggregationPipelineDPF(t *testing.T) {
	testAggregationPipelineDPF(t, true /*withEncryption*/)
	testAggregationPipelineDPF(t, false /*withEncryption*/)
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

	const keyBitSize = 20
	testData, err := createConversionsDpf(keyBitSize, 6, 6)
	if err != nil {
		t.Fatal(err)
	}

	ctxParams, err := dpfaggregator.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	combineParams := &dpfaggregator.CombineParams{
		DirectCombine: true,
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	conversions := beam.CreateList(scope, testData.Conversions)

	ePr1, ePr2 := splitRawConversion(scope, conversions, &GeneratePartialReportParams{
		PublicKeys1:   pubKeysInfo1,
		PublicKeys2:   pubKeysInfo2,
		KeyBitSize:    keyBitSize,
		EncryptOutput: withEncryption,
	})

	pr1 := dpfaggregator.DecryptPartialReport(scope, ePr1, privKeys1)
	pr2 := dpfaggregator.DecryptPartialReport(scope, ePr2, privKeys2)

	previousLevel := int32(-1)
	for i := range testData.Prefixes {
		want := beam.CreateList(scope, testData.WantResults[i])

		expandParams := &dpfaggregator.ExpandParameters{
			Prefixes:      [][]uint128.Uint128{testData.Prefixes[i]},
			Levels:        []int32{testData.SumParams.Params[i].LogDomainSize - 1},
			PreviousLevel: previousLevel,
		}
		ctx1 := dpfaggregator.CreateEvaluationContext(scope, pr1, expandParams, keyBitSize)
		ctx2 := dpfaggregator.CreateEvaluationContext(scope, pr2, expandParams, keyBitSize)

		ph1, err := dpfaggregator.ExpandAndCombineHistogram(scope, ctx1, expandParams, ctxParams, combineParams, keyBitSize)
		if err != nil {
			t.Fatal(err)
		}
		ph2, err := dpfaggregator.ExpandAndCombineHistogram(scope, ctx2, expandParams, ctxParams, combineParams, keyBitSize)
		if err != nil {
			t.Fatal(err)
		}
		got := dpfaggregator.MergeHistogram(scope, ph1, ph2)
		passert.Equals(scope, got, want)

		previousLevel = testData.SumParams.Params[i].LogDomainSize - 1
	}

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func BenchmarkPipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testAggregationPipelineDPF(b, true /*withEncryption*/)
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

	gotList := dpfaggregator.ReadEncryptedPartialReport(scope, filename)
	passert.Equals(scope, gotList, wantList)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func TestGetMaxKey(t *testing.T) {
	want := uint128.Max
	got := getMaxKey(128)
	if got.Cmp(want) != 0 {
		t.Fatalf("expect %s for key size %d, got %s", want.String(), 128, got.String())
	}

	var err error
	want, err = ioutils.StringToUint128("1180591620717411303423") // 2^70-1
	if err != nil {
		t.Fatal(err)
	}
	got = getMaxKey(70)
	if got.Cmp(want) != 0 {
		t.Fatalf("expect %s for key size %d, got %s", want.String(), 70, got.String())
	}
}
