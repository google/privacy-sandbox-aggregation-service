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

package reachbrowsersimulator

import (
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reachaggregator"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

func createReachRecords(keyBitSize, count uint64) ([]RawRecord, map[uint64]*reachaggregator.ReachResult) {
	maxKey := int64(1) << int64(keyBitSize)

	result := make(map[uint64]*reachaggregator.ReachResult)
	for i := uint64(0); i < uint64(maxKey); i++ {
		result[i] = &reachaggregator.ReachResult{}
	}

	var record []RawRecord
	for i := uint64(0); i < count; i++ {
		id := uint64(rand.Int63n(maxKey))
		// Keep the fingerprint the same for the same register id.
		record = append(record, RawRecord{LLRegister: id, Person: id})
		result[id].Count++
	}
	return record, result
}

func TestAggregationPipelineDPF(t *testing.T) {
	testAggregationPipelineDPF(t)
}

func testAggregationPipelineDPF(t testing.TB) {
	fileDir, err := ioutil.TempDir("/tmp", "test-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	tupleFile1 := path.Join(fileDir, "tuple1")
	tupleFile2 := path.Join(fileDir, "tuple2")

	rqFile1 := path.Join(fileDir, "rq1")
	rqFile2 := path.Join(fileDir, "rq2")

	ctx := context.Background()
	privKeys1, pubKeysInfo1, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}
	privKeys2, pubKeysInfo2, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}

	const keyBitSize = 17
	records, wantResult := createReachRecords(keyBitSize, 100)

	pipeline, scope := beam.NewPipelineWithRoot()

	conversions := beam.CreateList(scope, records)
	ePr1, ePr2 := splitRawConversion(scope, conversions, &GeneratePartialReportParams{
		PublicKeys1: pubKeysInfo1,
		PublicKeys2: pubKeysInfo2,
		KeyBitSize:  keyBitSize,
	})

	pr1 := reachaggregator.DecryptPartialReport(scope, ePr1, privKeys1)
	pr2 := reachaggregator.DecryptPartialReport(scope, ePr2, privKeys2)

	aggregateParams := &reachaggregator.AggregatePartialReportParams{
		DirectCombine: true,
		KeyBitSize:    keyBitSize,
	}
	ph1, err := reachaggregator.ExpandAndCombineHistogram(scope, pr1, aggregateParams)
	if err != nil {
		t.Fatal(err)
	}
	reachaggregator.WriteReachRQ(scope, ph1, rqFile1, 1)
	reachaggregator.WriteHistogram(scope, ph1, tupleFile1, 1)

	ph2, err := reachaggregator.ExpandAndCombineHistogram(scope, pr2, aggregateParams)
	if err != nil {
		t.Fatal(err)
	}
	reachaggregator.WriteReachRQ(scope, ph2, rqFile2, 1)
	reachaggregator.WriteHistogram(scope, ph2, tupleFile2, 1)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}

	tuple1, err := reachaggregator.ReadPartialHistogram(ctx, tupleFile1)
	if err != nil {
		t.Fatal(err)
	}
	rq2, err := reachaggregator.ReadReachRQ(ctx, rqFile2)
	if err != nil {
		t.Fatal(err)
	}

	tuple2, err := reachaggregator.ReadPartialHistogram(ctx, tupleFile2)
	if err != nil {
		t.Fatal(err)
	}
	rq1, err := reachaggregator.ReadReachRQ(ctx, rqFile1)
	if err != nil {
		t.Fatal(err)
	}

	reachResult1, err := reachaggregator.CreatePartialResult(rq2, tuple1)
	if err != nil {
		t.Fatal(err)
	}

	reachResult2, err := reachaggregator.CreatePartialResult(rq1, tuple2)
	if err != nil {
		t.Fatal(err)
	}

	gotResult, err := reachaggregator.CheckAndmergeReachResults(reachResult1, reachResult2)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantResult, gotResult); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}

func BenchmarkPipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testAggregationPipelineDPF(b)
	}
}
