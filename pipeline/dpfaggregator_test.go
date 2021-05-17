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

package dpfaggregator

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

type standardEncryptFn struct {
	PublicKey *pb.StandardPublicKey
}

func (fn *standardEncryptFn) ProcessElement(report *pb.PartialReportDpf, emit func(*pb.StandardCiphertext)) error {
	b, err := proto.Marshal(report)
	if err != nil {
		return err
	}
	result, err := standardencrypt.Encrypt(b, nil, fn.PublicKey)
	if err != nil {
		return err
	}
	emit(result)
	return nil
}

func TestDecryptPartialReport(t *testing.T) {
	priv, pub, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	reports := []*pb.PartialReportDpf{
		{
			SumKey: &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 2, Low: 1}},
		},
		{
			SumKey: &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 4, Low: 3}},
		},
	}

	pipeline, scope := beam.NewPipelineWithRoot()

	wantReports := beam.CreateList(scope, reports)
	encryptedReports := beam.ParDo(scope, &standardEncryptFn{PublicKey: pub}, wantReports)
	getReports := DecryptPartialReport(scope, encryptedReports, priv)

	passert.Equals(scope, getReports, wantReports)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type idPartialAggregation struct {
	ID                 uint64
	PartialAggregation *pb.PartialAggregationDpf
}

func convertIDPartialAggregationFn(id uint64, agg *pb.PartialAggregationDpf) idPartialAggregation {
	return idPartialAggregation{ID: id, PartialAggregation: agg}
}

// The overriden function MergeAccumulators() of combineFn is not called when running beam pipeline test locally. An ordinary unit test is created to cover this function.
func TestDirectAndSegmentCombineVectorMergeAccumulators(t *testing.T) {
	logN := uint64(8)

	vec1 := &expandedVec{
		SumVec: make([]uint64, 1<<logN),
	}
	vec1.SumVec[0] = 1
	vec1.SumVec[1<<logN-1] = 3

	vec2 := &expandedVec{
		SumVec: make([]uint64, 1<<logN),
	}
	vec2.SumVec[0] = 5
	vec2.SumVec[1<<logN-1] = 7

	wantDirect := &expandedVec{
		SumVec: make([]uint64, 1<<logN),
	}
	wantDirect.SumVec[0] = 6
	wantDirect.SumVec[1<<logN-1] = 10

	ctx := context.Background()
	directFn := &combineVectorFn{VectorLength: 1 << logN}
	gotDirect := directFn.MergeAccumulators(ctx, vec1, vec2)

	sortFn := func(a, b uint64) bool { return a < b }
	if diff := cmp.Diff(wantDirect.SumVec, gotDirect.SumVec, cmpopts.SortSlices(sortFn)); diff != "" {
		t.Fatalf("sum results of direct combine mismatch (-want +got):\n%s", diff)
	}

	wantSegment := &expandedVec{
		SumVec: make([]uint64, 2),
	}
	wantSegment.SumVec[1] = 17

	segmentFn := &combineVectorSegmentFn{StartIndex: 1<<logN - 2, Length: 2}
	acc1 := segmentFn.CreateAccumulator(ctx)
	acc2 := segmentFn.CreateAccumulator(ctx)
	acc1 = segmentFn.AddInput(ctx, acc1, vec1)
	acc2 = segmentFn.AddInput(ctx, acc2, vec2)
	gotSegment := segmentFn.MergeAccumulators(ctx, acc1, acc2)
	if diff := cmp.Diff(wantSegment.SumVec, gotSegment.SumVec, cmpopts.SortSlices(sortFn)); diff != "" {
		t.Fatalf("sum results of segment combine mismatch (-want +got):\n%s", diff)
	}
}

func TestDirectAndSegmentCombineVector(t *testing.T) {
	logN := uint64(8)

	vec1 := &expandedVec{
		SumVec: make([]uint64, 1<<logN),
	}
	vec1.SumVec[0] = 1
	vec1.SumVec[1<<logN-1] = 3

	vec2 := &expandedVec{
		SumVec: make([]uint64, 1<<logN),
	}
	vec2.SumVec[0] = 5
	vec2.SumVec[1<<logN-1] = 7

	pipeline, scope := beam.NewPipelineWithRoot()
	inputVec := beam.CreateList(scope, []*expandedVec{vec1, vec2})

	want := make([]idPartialAggregation, 1<<logN)
	for i := 0; i < 1<<logN; i++ {
		want[i].ID = uint64(i)
		want[i].PartialAggregation = &pb.PartialAggregationDpf{}
	}
	want[0].PartialAggregation.PartialSum = 6
	want[1<<logN-1].PartialAggregation.PartialSum = 10
	wantResult := beam.CreateList(scope, want)

	getResultSegment := segmentCombine(scope, inputVec, 1<<logN, 1<<(logN-5), nil)
	passert.Equals(scope, beam.ParDo(scope, convertIDPartialAggregationFn, getResultSegment), wantResult)

	getResultDirect := directCombine(scope, inputVec, 1<<logN, nil)
	passert.Equals(scope, beam.ParDo(scope, convertIDPartialAggregationFn, getResultDirect), wantResult)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type rawConversion struct {
	Index uint64
	Value uint64
}

type splitConversionFn struct {
	Params *pb.IncrementalDpfParameters
}

func (fn *splitConversionFn) ProcessElement(ctx context.Context, c rawConversion, emit1 func(*pb.PartialReportDpf), emit2 func(*pb.PartialReportDpf)) error {
	valueSum := make([]uint64, len(fn.Params.Params))
	valueCount := make([]uint64, len(fn.Params.Params))
	for i := range valueSum {
		valueSum[i] = c.Value
		valueCount[i] = uint64(1)
	}

	keyDpfSum1, keyDpfSum2, err := incrementaldpf.GenerateKeys(fn.Params.Params, c.Index, valueSum)
	if err != nil {
		return err
	}

	emit1(&pb.PartialReportDpf{
		SumKey: keyDpfSum1,
	})
	emit2(&pb.PartialReportDpf{
		SumKey: keyDpfSum2,
	})
	return nil
}

func TestDirectAggregationAndMerge(t *testing.T) {
	want := []CompleteHistogram{
		{Index: 1, Sum: 10},
	}
	var reports []rawConversion
	for _, h := range want {
		for i := uint64(0); i < h.Sum; i++ {
			reports = append(reports, rawConversion{Index: h.Index, Value: 1})
		}
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	conversions := beam.CreateList(scope, reports)

	dpfParams := &pb.IncrementalDpfParameters{Params: []*dpfpb.DpfParameters{
		{LogDomainSize: 8, ElementBitsize: 1 << 6},
	}}
	partialReport1, partialReport2 := beam.ParDo2(scope, &splitConversionFn{Params: dpfParams}, conversions)
	params := &AggregatePartialReportParams{
		SumParameters: dpfParams,
		Prefixes:      &pb.HierarchicalPrefixes{Prefixes: []*pb.DomainPrefixes{{}}},
		DirectCombine: true,
	}
	partialResult1, err := ExpandAndCombineHistogram(scope, partialReport1, params)
	if err != nil {
		t.Fatal(err)
	}
	partialResult2, err := ExpandAndCombineHistogram(scope, partialReport2, params)
	if err != nil {
		t.Fatal(err)
	}

	joined := beam.CoGroupByKey(scope, partialResult1, partialResult2)
	got := beam.ParDo(scope, &mergeHistogramFn{}, joined)

	passert.Equals(scope, got, beam.CreateList(scope, want))

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func TestHierarchicalAggregationAndMerge(t *testing.T) {
	want := []CompleteHistogram{
		{Index: 16, Sum: 10},
	}
	var reports []rawConversion
	for _, h := range want {
		for i := uint64(0); i < h.Sum; i++ {
			reports = append(reports, rawConversion{Index: h.Index, Value: 1})
		}
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	conversions := beam.CreateList(scope, reports)

	dpfParams := &pb.IncrementalDpfParameters{Params: []*dpfpb.DpfParameters{
		{LogDomainSize: 4, ElementBitsize: 1 << 6},
		{LogDomainSize: 8, ElementBitsize: 1 << 6},
	}}
	partialReport1, partialReport2 := beam.ParDo2(scope, &splitConversionFn{Params: dpfParams}, conversions)
	params := &AggregatePartialReportParams{
		SumParameters: dpfParams,
		Prefixes:      &pb.HierarchicalPrefixes{Prefixes: []*pb.DomainPrefixes{{}, {Prefix: []uint64{1}}}},
		DirectCombine: true,
	}
	partialResult1, err := ExpandAndCombineHistogram(scope, partialReport1, params)
	if err != nil {
		t.Fatal(err)
	}
	partialResult2, err := ExpandAndCombineHistogram(scope, partialReport2, params)
	if err != nil {
		t.Fatal(err)
	}

	joined := beam.CoGroupByKey(scope, partialResult1, partialResult2)
	got := beam.ParDo(scope, &mergeHistogramFn{}, joined)

	passert.Equals(scope, got, beam.CreateList(scope, want))

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type idAgg struct {
	ID  uint64
	Agg *pb.PartialAggregationDpf
}

func TestReadPartialHistogram(t *testing.T) {
	fileDir, err := ioutil.TempDir("/tmp", "test-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	data := []*idAgg{
		{ID: 111, Agg: &pb.PartialAggregationDpf{PartialSum: 222}},
		{ID: 444, Agg: &pb.PartialAggregationDpf{PartialSum: 555}},
	}
	want := make(map[uint64]*pb.PartialAggregationDpf)
	for _, d := range data {
		want[d.ID] = d.Agg
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	records := beam.CreateList(scope, data)
	partial := beam.ParDo(scope, func(a *idAgg) (uint64, *pb.PartialAggregationDpf) {
		return a.ID, a.Agg
	}, records)
	partialFile := path.Join(fileDir, "partial_agg.txt")
	writeHistogram(scope, partial, partialFile, 1)
	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}

	ctx := context.Background()
	got, err := ReadPartialHistogram(ctx, partialFile)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Saved and read partial aggregation mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteCompleteHistogram(t *testing.T) {
	fileDir, err := ioutil.TempDir("/tmp", "test-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	want := map[uint64]CompleteHistogram{
		111: {Index: 111, Sum: 222},
		555: {Index: 555, Sum: 666},
	}
	resultFile := path.Join(fileDir, "result.txt")
	ctx := context.Background()
	if err := WriteCompleteHistogram(ctx, resultFile, want); err != nil {
		t.Fatal(err)
	}

	lines, err := ioutils.ReadLines(resultFile)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[uint64]CompleteHistogram)
	for _, l := range lines {
		cols := strings.Split(l, ",")
		if gotLen, wantLen := len(cols), 2; gotLen != wantLen {
			t.Fatalf("got %d columns in line %q, want %d", gotLen, l, wantLen)
		}
		idx, err := strconv.ParseUint(cols[0], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		sum, err := strconv.ParseUint(cols[1], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		got[idx] = CompleteHistogram{Index: idx, Sum: sum}
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Read and saved complete histogram mismatch (-want +got):\n%s", diff)
	}
}
