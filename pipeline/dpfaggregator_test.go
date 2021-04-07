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
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

type standardEncryptFn struct {
	PublicKey *pb.StandardPublicKey
}

func (fn *standardEncryptFn) ProcessElement(report *pb.PartialReportDpf, emit func(*pb.StandardCiphertext)) error {
	b, err := proto.Marshal(report)
	if err != nil {
		return err
	}
	result, err := standardencrypt.Encrypt(b, fn.PublicKey)
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
			SumKey:   &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 2, Low: 1}},
			CountKey: &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 3, Low: 2}},
		},
		{
			SumKey:   &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 4, Low: 3}},
			CountKey: &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 5, Low: 4}},
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
		SumVec:   make([]uint64, 1<<logN),
		CountVec: make([]uint64, 1<<logN),
	}
	vec1.SumVec[0] = 1
	vec1.CountVec[0] = 2
	vec1.SumVec[1<<logN-1] = 3
	vec1.CountVec[1<<logN-1] = 4

	vec2 := &expandedVec{
		SumVec:   make([]uint64, 1<<logN),
		CountVec: make([]uint64, 1<<logN),
	}
	vec2.SumVec[0] = 5
	vec2.CountVec[0] = 6
	vec2.SumVec[1<<logN-1] = 7
	vec2.CountVec[1<<logN-1] = 8

	wantDirect := &expandedVec{
		SumVec:   make([]uint64, 1<<logN),
		CountVec: make([]uint64, 1<<logN),
	}
	wantDirect.SumVec[0] = 6
	wantDirect.CountVec[0] = 8
	wantDirect.SumVec[1<<logN-1] = 10
	wantDirect.CountVec[1<<logN-1] = 12

	ctx := context.Background()
	directFn := &combineVectorFn{LogN: logN}
	gotDirect := directFn.MergeAccumulators(ctx, vec1, vec2)

	sortFn := func(a, b uint64) bool { return a < b }
	if diff := cmp.Diff(wantDirect.SumVec, gotDirect.SumVec, cmpopts.SortSlices(sortFn)); diff != "" {
		t.Fatalf("sum results of direct combine mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantDirect.CountVec, gotDirect.CountVec, cmpopts.SortSlices(sortFn)); diff != "" {
		t.Fatalf("count results of direct combine mismatch (-want +got):\n%s", diff)
	}

	wantSegment := &expandedVec{
		SumVec:   make([]uint64, 2),
		CountVec: make([]uint64, 2),
	}
	wantSegment.SumVec[1] = 17
	wantSegment.CountVec[1] = 20

	segmentFn := &combineVectorSegmentFn{StartIndex: 1<<logN - 2, Length: 2}
	acc1 := segmentFn.CreateAccumulator(ctx)
	acc2 := segmentFn.CreateAccumulator(ctx)
	acc1 = segmentFn.AddInput(ctx, acc1, vec1)
	acc2 = segmentFn.AddInput(ctx, acc2, vec2)
	gotSegment := segmentFn.MergeAccumulators(ctx, acc1, acc2)
	if diff := cmp.Diff(wantSegment.SumVec, gotSegment.SumVec, cmpopts.SortSlices(sortFn)); diff != "" {
		t.Fatalf("sum results of segment combine mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantSegment.CountVec, gotSegment.CountVec, cmpopts.SortSlices(sortFn)); diff != "" {
		t.Fatalf("count results of segment combine mismatch (-want +got):\n%s", diff)
	}
}

func TestDirectAndSegmentCombineVector(t *testing.T) {
	logN := uint64(8)

	vec1 := &expandedVec{
		SumVec:   make([]uint64, 1<<logN),
		CountVec: make([]uint64, 1<<logN),
	}
	vec1.SumVec[0] = 1
	vec1.CountVec[0] = 2
	vec1.SumVec[1<<logN-1] = 3
	vec1.CountVec[1<<logN-1] = 4

	vec2 := &expandedVec{
		SumVec:   make([]uint64, 1<<logN),
		CountVec: make([]uint64, 1<<logN),
	}
	vec2.SumVec[0] = 5
	vec2.CountVec[0] = 6
	vec2.SumVec[1<<logN-1] = 7
	vec2.CountVec[1<<logN-1] = 8

	pipeline, scope := beam.NewPipelineWithRoot()
	inputVec := beam.CreateList(scope, []*expandedVec{vec1, vec2})

	want := make([]idPartialAggregation, 1<<logN)
	for i := 0; i < 1<<logN; i++ {
		want[i].ID = uint64(i)
		want[i].PartialAggregation = &pb.PartialAggregationDpf{}
	}
	want[0].PartialAggregation.PartialSum = 6
	want[0].PartialAggregation.PartialCount = 8
	want[1<<logN-1].PartialAggregation.PartialSum = 10
	want[1<<logN-1].PartialAggregation.PartialCount = 12
	wantResult := beam.CreateList(scope, want)

	getResultSegment := segmentCombine(scope, inputVec, logN, 1<<(logN-5))
	passert.Equals(scope, beam.ParDo(scope, convertIDPartialAggregationFn, getResultSegment), wantResult)

	getResultDirect := directCombine(scope, inputVec, logN)
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
	LogN                uint64
	LogElementSizeSum   uint64
	LogElementSizeCount uint64
}

func (fn *splitConversionFn) ProcessElement(ctx context.Context, c rawConversion, emit1 func(*pb.PartialReportDpf), emit2 func(*pb.PartialReportDpf)) error {
	keyDpfSum1, keyDpfSum2, err := incrementaldpf.GenerateKeys(&dpfpb.DpfParameters{
		LogDomainSize:  int32(fn.LogN),
		ElementBitsize: 1 << fn.LogElementSizeSum,
	}, c.Index, c.Value)
	if err != nil {
		return err
	}
	keyDpfCount1, keyDpfCount2, err := incrementaldpf.GenerateKeys(&dpfpb.DpfParameters{
		LogDomainSize:  int32(fn.LogN),
		ElementBitsize: 1 << fn.LogElementSizeCount,
	}, c.Index, 1)
	if err != nil {
		return err
	}

	emit1(&pb.PartialReportDpf{
		SumKey:   keyDpfSum1,
		CountKey: keyDpfCount1,
	})
	emit2(&pb.PartialReportDpf{
		SumKey:   keyDpfSum2,
		CountKey: keyDpfCount2,
	})
	return nil
}
