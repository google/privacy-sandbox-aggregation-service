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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

func getRandomPublicKey(keys []cryptoio.PublicKeyInfo) (string, *pb.StandardPublicKey, error) {
	keyInfo := keys[rand.Intn(len(keys))]
	bKey, err := base64.StdEncoding.DecodeString(keyInfo.Key)
	if err != nil {
		return "", nil, err
	}
	return keyInfo.ID, &pb.StandardPublicKey{Key: bKey}, nil
}

type standardEncryptFn struct {
	PublicKeys []cryptoio.PublicKeyInfo
}

func (fn *standardEncryptFn) ProcessElement(report *pb.PartialReportDpf, emit func(*pb.AggregatablePayload)) error {
	b, err := proto.Marshal(report.SumKey)
	if err != nil {
		return err
	}

	payload := reporttypes.Payload{DPFKey: b}
	bPayload, err := utils.MarshalCBOR(payload)
	if err != nil {
		return err
	}

	contextInfo := "context"
	keyID, publicKey, err := getRandomPublicKey(fn.PublicKeys)
	if err != nil {
		return err
	}
	result, err := standardencrypt.Encrypt(bPayload, []byte(contextInfo), publicKey)
	if err != nil {
		return err
	}
	emit(&pb.AggregatablePayload{Payload: result, SharedInfo: contextInfo, KeyId: keyID})
	return nil
}

func TestDecryptPartialReport(t *testing.T) {
	ctx := context.Background()
	privKeys, pubKeysInfo, err := cryptoio.GenerateHybridKeyPairs(ctx, 1, "", "")
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
	encryptedReports := beam.ParDo(scope, &standardEncryptFn{PublicKeys: pubKeysInfo}, wantReports)
	getReports := DecryptPartialReport(scope, encryptedReports, privKeys)

	passert.Equals(scope, getReports, wantReports)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type idPartialAggregation struct {
	ID                 uint128.Uint128
	PartialAggregation *pb.PartialAggregationDpf
}

func convertIDPartialAggregationFn(id uint128.Uint128, agg *pb.PartialAggregationDpf) idPartialAggregation {
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

	for _, withBucketIDs := range []bool{
		true,
		false,
	} {
		want := make([]idPartialAggregation, 1<<logN)
		bucketIDs := make([]uint128.Uint128, 1<<logN)
		for i := 0; i < 1<<logN; i++ {
			bucketIDs[i] = uint128.From64(rand.Uint64())
			want[i].ID = uint128.From64(uint64(i))
			if withBucketIDs {
				want[i].ID = bucketIDs[i]
			}
			want[i].PartialAggregation = &pb.PartialAggregationDpf{}
		}
		want[0].PartialAggregation.PartialSum = 6
		want[1<<logN-1].PartialAggregation.PartialSum = 10

		pipeline, scope := beam.NewPipelineWithRoot()
		inputVec := beam.CreateList(scope, []*expandedVec{vec1, vec2})

		wantResult := beam.CreateList(scope, want)

		var intputBuckets beam.PCollection
		if withBucketIDs {
			intputBuckets = beam.CreateList(scope, [][]uint128.Uint128{bucketIDs})
		} else {
			intputBuckets = beam.CreateList(scope, [][]uint128.Uint128{{}})
		}
		getResultSegment := segmentCombine(scope, inputVec, intputBuckets, 1<<logN, 13)
		passert.Equals(scope, beam.ParDo(scope, convertIDPartialAggregationFn, getResultSegment), wantResult)

		getResultDirect := directCombine(scope, inputVec, intputBuckets, 1<<logN)
		passert.Equals(scope, beam.ParDo(scope, convertIDPartialAggregationFn, getResultDirect), wantResult)

		if err := ptest.Run(pipeline); err != nil {
			t.Fatalf("pipeline failed with input buckets %v: %s", withBucketIDs, err)
		}
	}
}

type rawConversion struct {
	Index uint128.Uint128
	Value uint64
}

type splitConversionFn struct {
	KeyBitSize int
}

func (fn *splitConversionFn) ProcessElement(ctx context.Context, c rawConversion, emit1 func(*pb.PartialReportDpf), emit2 func(*pb.PartialReportDpf)) error {
	valueSum := make([]uint64, fn.KeyBitSize)
	for i := range valueSum {
		valueSum[i] = c.Value
	}
	ctxParams, err := incrementaldpf.GetDefaultDPFParameters(fn.KeyBitSize)
	if err != nil {
		return err
	}

	keyDpfSum1, keyDpfSum2, err := incrementaldpf.GenerateKeys(ctxParams, c.Index, valueSum)
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

const keyBitSize = 8

func TestAggregationAndMerge(t *testing.T) {
	want := []CompleteHistogram{
		{Bucket: uint128.From64(1), Sum: 10},
	}
	var reports []rawConversion
	for _, h := range want {
		for i := uint64(0); i < h.Sum; i++ {
			reports = append(reports, rawConversion{Index: h.Bucket, Value: 1})
		}
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	conversions := beam.CreateList(scope, reports)

	ctxParams, err := incrementaldpf.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	expandParams := &ExpandParameters{
		Level:           7,
		PreviousLevel:   -1,
		DirectExpansion: false,
	}
	combineParams := &CombineParams{
		DirectCombine: true,
	}

	partialReport1, partialReport2 := beam.ParDo2(scope, &splitConversionFn{KeyBitSize: keyBitSize}, conversions)
	evalCtx1 := CreateEvaluationContext(scope, partialReport1, expandParams, keyBitSize)
	evalCtx2 := CreateEvaluationContext(scope, partialReport2, expandParams, keyBitSize)

	partialResult1, err := ExpandAndCombineHistogram(scope, evalCtx1, expandParams, ctxParams, combineParams, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	partialResult2, err := ExpandAndCombineHistogram(scope, evalCtx2, expandParams, ctxParams, combineParams, keyBitSize)
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
		{Bucket: uint128.From64(16), Sum: 10},
	}
	var reports []rawConversion
	for _, h := range want {
		for i := uint64(0); i < h.Sum; i++ {
			reports = append(reports, rawConversion{Index: h.Bucket, Value: 1})
		}
	}
	combineParams := &CombineParams{
		DirectCombine: true,
	}
	ctxParams, err := incrementaldpf.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	conversions := beam.CreateList(scope, reports)

	partialReport1, partialReport2 := beam.ParDo2(scope, &splitConversionFn{KeyBitSize: keyBitSize}, conversions)

	// For the first level.
	expandParams0 := &ExpandParameters{
		Level:           3,
		PreviousLevel:   -1,
		DirectExpansion: false,
	}
	evalCtx01 := CreateEvaluationContext(scope, partialReport1, expandParams0, keyBitSize)
	evalCtx02 := CreateEvaluationContext(scope, partialReport2, expandParams0, keyBitSize)
	partialResult01, err := ExpandAndCombineHistogram(scope, evalCtx01, expandParams0, ctxParams, combineParams, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	partialResult02, err := ExpandAndCombineHistogram(scope, evalCtx02, expandParams0, ctxParams, combineParams, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	joined0 := beam.CoGroupByKey(scope, partialResult01, partialResult02)
	got0 := beam.ParDo(scope, &mergeHistogramFn{}, joined0)
	passert.Equals(scope, got0, beam.CreateList(scope, []CompleteHistogram{
		{Bucket: uint128.From64(1), Sum: 10},
	}))

	// For the second level.
	expandParams1 := &ExpandParameters{
		Prefixes:        []uint128.Uint128{uint128.From64(1)},
		Level:           7,
		PreviousLevel:   3,
		DirectExpansion: false,
	}
	evalCtx11 := CreateEvaluationContext(scope, partialReport1, expandParams1, keyBitSize)
	evalCtx12 := CreateEvaluationContext(scope, partialReport2, expandParams1, keyBitSize)
	partialResult11, err := ExpandAndCombineHistogram(scope, evalCtx11, expandParams1, ctxParams, combineParams, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	partialResult12, err := ExpandAndCombineHistogram(scope, evalCtx12, expandParams1, ctxParams, combineParams, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	joined1 := beam.CoGroupByKey(scope, partialResult11, partialResult12)
	got1 := beam.ParDo(scope, &mergeHistogramFn{}, joined1)
	passert.Equals(scope, got1, beam.CreateList(scope, want))

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func TestDirectAggregationAndMerge(t *testing.T) {
	want := []CompleteHistogram{
		{Bucket: uint128.From64(16), Sum: 10},
	}
	var reports []rawConversion
	for _, h := range want {
		for i := uint64(0); i < h.Sum; i++ {
			reports = append(reports, rawConversion{Index: h.Bucket, Value: 1})
		}
	}
	combineParams := &CombineParams{
		DirectCombine: true,
	}
	ctxParams, err := incrementaldpf.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	conversions := beam.CreateList(scope, reports)

	partialReport1, partialReport2 := beam.ParDo2(scope, &splitConversionFn{KeyBitSize: keyBitSize}, conversions)
	expandParams := &ExpandParameters{
		Prefixes:        []uint128.Uint128{uint128.From64(16), uint128.From64(17)},
		Level:           7,
		PreviousLevel:   -1,
		DirectExpansion: true,
	}
	evalCtx1 := CreateEvaluationContext(scope, partialReport1, expandParams, keyBitSize)
	evalCtx2 := CreateEvaluationContext(scope, partialReport2, expandParams, keyBitSize)
	partialResult1, err := ExpandAndCombineHistogram(scope, evalCtx1, expandParams, ctxParams, combineParams, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}
	partialResult2, err := ExpandAndCombineHistogram(scope, evalCtx2, expandParams, ctxParams, combineParams, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	joined := beam.CoGroupByKey(scope, partialResult1, partialResult2)
	got := beam.ParDo(scope, &mergeHistogramFn{}, joined)
	passert.Equals(scope, got, beam.CreateList(scope, []CompleteHistogram{
		{Bucket: uint128.From64(16), Sum: 10},
	}))

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type idAgg struct {
	ID  uint128.Uint128
	Agg *pb.PartialAggregationDpf
}

func TestReadPartialHistogram(t *testing.T) {
	fileDir, err := ioutil.TempDir("/tmp", "test-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	data := []*idAgg{
		{ID: uint128.From64(111), Agg: &pb.PartialAggregationDpf{PartialSum: 222}},
		{ID: uint128.From64(444), Agg: &pb.PartialAggregationDpf{PartialSum: 555}},
	}
	want := make(map[uint128.Uint128]*pb.PartialAggregationDpf)
	for _, d := range data {
		want[d.ID] = d.Agg
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	records := beam.CreateList(scope, data)
	partial := beam.ParDo(scope, func(a *idAgg) (uint128.Uint128, *pb.PartialAggregationDpf) {
		return a.ID, a.Agg
	}, records)
	partialFile := path.Join(fileDir, "partial_agg.txt")
	writeHistogram(scope, partial, partialFile)
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

func parseCompleteHistogram(line string) (CompleteHistogram, error) {
	cols := strings.Split(line, ",")
	if gotLen, wantLen := len(cols), 2; gotLen != wantLen {
		return CompleteHistogram{}, fmt.Errorf("got %d columns in line %q, want %d", gotLen, line, wantLen)
	}
	idx, err := utils.StringToUint128(cols[0])
	if err != nil {
		return CompleteHistogram{}, err
	}
	sum, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return CompleteHistogram{}, err
	}
	return CompleteHistogram{Bucket: idx, Sum: sum}, nil
}

func TestWriteCompleteHistogramWithoutPipeline(t *testing.T) {
	fileDir, err := ioutil.TempDir("/tmp", "test-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	want := map[uint128.Uint128]CompleteHistogram{
		uint128.From64(111): {Bucket: uint128.From64(111), Sum: 222},
		uint128.From64(555): {Bucket: uint128.From64(555), Sum: 666},
	}
	resultFile := path.Join(fileDir, "result.txt")
	ctx := context.Background()
	if err := WriteCompleteHistogram(ctx, resultFile, want); err != nil {
		t.Fatal(err)
	}

	lines, err := utils.ReadLines(ctx, resultFile)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[uint128.Uint128]CompleteHistogram)
	for _, l := range lines {
		histogram, err := parseCompleteHistogram(l)
		if err != nil {
			t.Fatal(err)
		}
		got[histogram.Bucket] = histogram
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Read and saved complete histogram mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteReadPartialHistogram(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	want := []idPartialAggregation{
		{ID: uint128.From64(1), PartialAggregation: &pb.PartialAggregationDpf{PartialSum: 1}},
		{ID: uint128.From64(2), PartialAggregation: &pb.PartialAggregationDpf{PartialSum: 2}},
		{ID: uint128.From64(3), PartialAggregation: &pb.PartialAggregationDpf{PartialSum: 3}},
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	wantList := beam.CreateList(scope, want)
	table := beam.ParDo(scope, func(p idPartialAggregation) (uint128.Uint128, *pb.PartialAggregationDpf) {
		return p.ID, p.PartialAggregation
	}, wantList)
	filename := path.Join(tmpDir, "partial.txt")
	writeHistogram(scope, table, filename)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}

	gotList := beam.ParDo(scope, convertIDPartialAggregationFn, readPartialHistogram(scope, filename))
	passert.Equals(scope, gotList, wantList)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func TestWriteReadCompleteHistogramWithPipeline(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	want := []CompleteHistogram{
		{Bucket: uint128.From64(1), Sum: 1},
		{Bucket: uint128.From64(2), Sum: 2},
		{Bucket: uint128.From64(3), Sum: 3},
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	wantList := beam.CreateList(scope, want)
	filename := path.Join(tmpDir, "complete.txt")
	writeCompleteHistogram(scope, wantList, filename)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}

	gotList := beam.ParDo(scope, func(line string, emit func(CompleteHistogram)) error {
		histogram, err := parseCompleteHistogram(line)
		if err != nil {
			return err
		}
		emit(histogram)
		return nil
	}, textio.Read(scope, filename))
	passert.Equals(scope, gotList, wantList)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func TestMergePartialHistogram(t *testing.T) {
	partial1 := map[uint128.Uint128]*pb.PartialAggregationDpf{
		uint128.From64(0): &pb.PartialAggregationDpf{PartialSum: 1},
		uint128.From64(1): &pb.PartialAggregationDpf{PartialSum: 1},
		uint128.From64(2): &pb.PartialAggregationDpf{PartialSum: 1},
		uint128.From64(3): &pb.PartialAggregationDpf{PartialSum: 1},
	}
	partial2 := map[uint128.Uint128]*pb.PartialAggregationDpf{
		uint128.From64(0): &pb.PartialAggregationDpf{PartialSum: 0},
		uint128.From64(1): &pb.PartialAggregationDpf{PartialSum: 1},
		uint128.From64(2): &pb.PartialAggregationDpf{PartialSum: 2},
		uint128.From64(3): &pb.PartialAggregationDpf{PartialSum: 3},
	}

	got, err := MergePartialResult(partial1, partial2)
	if err != nil {
		t.Fatal(err)
	}

	want := []CompleteHistogram{
		{Bucket: uint128.From64(0), Sum: 1},
		{Bucket: uint128.From64(1), Sum: 2},
		{Bucket: uint128.From64(2), Sum: 3},
		{Bucket: uint128.From64(3), Sum: 4},
	}
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b CompleteHistogram) bool { return a.Bucket.Cmp(b.Bucket) == -1 })); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}

	errStr := "partial results have different lengths: 1 and 2"
	if _, err := MergePartialResult(map[uint128.Uint128]*pb.PartialAggregationDpf{
		uint128.From64(0): &pb.PartialAggregationDpf{PartialSum: 1},
	}, map[uint128.Uint128]*pb.PartialAggregationDpf{
		uint128.From64(0): &pb.PartialAggregationDpf{PartialSum: 1},
		uint128.From64(1): &pb.PartialAggregationDpf{PartialSum: 1},
	}); err == nil {
		t.Fatalf("expect error %q", errStr)
	} else if err.Error() != errStr {
		t.Fatalf("expect error message %q, got %q", errStr, err.Error())
	}

	errStr = "index 2 appears in partial1, missing in partial2"
	if _, err := MergePartialResult(map[uint128.Uint128]*pb.PartialAggregationDpf{
		uint128.From64(1): &pb.PartialAggregationDpf{PartialSum: 1},
		uint128.From64(2): &pb.PartialAggregationDpf{PartialSum: 1},
	}, map[uint128.Uint128]*pb.PartialAggregationDpf{
		uint128.From64(0): &pb.PartialAggregationDpf{PartialSum: 1},
		uint128.From64(1): &pb.PartialAggregationDpf{PartialSum: 1},
	}); err == nil {
		t.Fatalf("expect error %q", errStr)
	} else if err.Error() != errStr {
		t.Fatalf("expect error message %q, got %q", errStr, err.Error())
	}
}

func TestReadWriteDPFparameters(t *testing.T) {
	baseDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(baseDir)

	wantExpandParams := &ExpandParameters{
		Level:           3,
		Prefixes:        []uint128.Uint128{uint128.From64(1)},
		PreviousLevel:   -1,
		DirectExpansion: false,
	}
	expandPath := path.Join(baseDir, "expand.txt")

	ctx := context.Background()
	if err := SaveExpandParameters(ctx, wantExpandParams, expandPath); err != nil {
		t.Fatal(err)
	}
	gotExpandParams, err := ReadExpandParameters(ctx, expandPath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantExpandParams, gotExpandParams, protocmp.Transform()); diff != "" {
		t.Errorf("Expand parameters read/write mismatch (-want +got):\n%s", diff)
	}
}
