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

package incrementaldpf

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func TestDpfGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 20, ElementBitsize: 64},
	}
	k1, k2, err := GenerateKeys(params, 0, []uint64{1})
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext(params, k1)
	if err != nil {
		t.Fatal(err)
	}
	expanded1, err := EvaluateNext64([]uint64{}, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext(params, k2)
	if err != nil {
		t.Fatal(err)
	}
	expanded2, err := EvaluateNext64([]uint64{}, evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]uint64, len(expanded1))
	for i := 0; i < len(expanded1); i++ {
		got[i] = expanded1[i] + expanded2[i]
	}
	want := make([]uint64, 1<<params[0].LogDomainSize)
	want[0] = 1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestDpfHierarchicalGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 2, ElementBitsize: 64},
		{LogDomainSize: 4, ElementBitsize: 64},
	}
	alpha, beta := uint64(8), uint64(1)
	k1, k2, err := GenerateKeys(params, alpha, []uint64{beta, beta})
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext(params, k1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext(params, k2)
	if err != nil {
		t.Fatal(err)
	}

	prefixes := &pb.HierarchicalPrefixes{Prefixes: []*pb.DomainPrefixes{{}, {Prefix: []uint64{0, 2}}}}
	// First level of expansion for the first two bits.
	expanded1, err := EvaluateNext64(prefixes.Prefixes[0].Prefix, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err := EvaluateNext64(prefixes.Prefixes[0].Prefix, evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]uint64, len(expanded1))
	for i := range expanded1 {
		got[i] = expanded1[i] + expanded2[i]
	}
	want := make([]uint64, 1<<params[0].GetLogDomainSize())
	want[2] = 1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}

	// Second level of expansion for all four bits of two prefixes: 0 (00**) and 2 (10**).
	expanded1, err = EvaluateNext64(prefixes.Prefixes[1].Prefix, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err = EvaluateNext64(prefixes.Prefixes[1].Prefix, evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	gotMap := make(map[uint64]uint64)
	ids, err := CalculateBucketID(&pb.IncrementalDpfParameters{Params: params}, prefixes, 0 /*prefixLevel*/, 1 /*expandLevel*/)
	if err != nil {
		t.Fatal(err)
	}
	if ids == nil {
		t.Fatalf("bucket IDs should not be empty")
	}
	for i := range expanded1 {
		result := expanded1[i] + expanded2[i]
		if result != 0 {
			gotMap[ids[i]] = result
		}
	}
	wantMap := make(map[uint64]uint64)
	wantMap[alpha] = beta
	if diff := cmp.Diff(wantMap, gotMap); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestDpfMultiLevelHierarchicalGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := &pb.IncrementalDpfParameters{
		Params: []*dpfpb.DpfParameters{
			{LogDomainSize: 2, ElementBitsize: 64},
			{LogDomainSize: 4, ElementBitsize: 64},
			{LogDomainSize: 5, ElementBitsize: 64},
		},
	}
	alpha, beta := uint64(16), uint64(1)
	k1, k2, err := GenerateKeys(params.Params, alpha, []uint64{beta, beta, beta})
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext(params.Params, k1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext(params.Params, k2)
	if err != nil {
		t.Fatal(err)
	}

	prefixes := &pb.HierarchicalPrefixes{Prefixes: []*pb.DomainPrefixes{
		{},
		{Prefix: []uint64{0, 2}},
		{Prefix: []uint64{1}},
	}}
	// First level of expansion for the first two bits.
	expanded1, err := EvaluateUntil64(0, prefixes.Prefixes[0].Prefix, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err := EvaluateUntil64(0, prefixes.Prefixes[0].Prefix, evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]uint64, len(expanded1))
	for i := range expanded1 {
		got[i] = expanded1[i] + expanded2[i]
	}
	want := make([]uint64, 1<<params.Params[0].GetLogDomainSize())
	want[2] = 1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}

	// The next level of expansion for all five bits of two prefixes: 0 (00***) and 2 (10***).
	expanded1, err = EvaluateUntil64(2, prefixes.Prefixes[1].Prefix, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err = EvaluateUntil64(2, prefixes.Prefixes[1].Prefix, evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	gotMap := make(map[uint64]uint64)
	ids, err := CalculateBucketID(params, prefixes, 0 /*prefixLevel*/, 2 /*expandLevel*/)
	if err != nil {
		t.Fatal(err)
	}
	if ids == nil {
		t.Fatalf("bucket IDs should not be empty")
	}
	for i := range expanded1 {
		result := expanded1[i] + expanded2[i]
		if result != 0 {
			gotMap[ids[i]] = result
		}
	}
	wantMap := make(map[uint64]uint64)
	wantMap[alpha] = beta
	if diff := cmp.Diff(wantMap, gotMap); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestCalculateBucketID(t *testing.T) {
	params := &pb.IncrementalDpfParameters{
		Params: []*dpfpb.DpfParameters{
			{LogDomainSize: 2, ElementBitsize: 64},
			{LogDomainSize: 3, ElementBitsize: 64},
			{LogDomainSize: 4, ElementBitsize: 64},
		},
	}
	prefixes := &pb.HierarchicalPrefixes{Prefixes: []*pb.DomainPrefixes{
		{},
		{Prefix: []uint64{1, 3}},
		{Prefix: []uint64{2, 3, 6}},
	}}

	got, err := CalculateBucketID(params, prefixes, 0 /*prefixLevel*/, 1 /*expandLevel*/)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]uint64{2, 3, 6, 7}, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}

	got, err = CalculateBucketID(params, prefixes, 1 /*prefixLevel*/, 2 /*expandLevel*/)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]uint64{4, 5, 6, 7, 12, 13}, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestReachUint64TupleDpfGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := CreateReachUint64TupleDpfParameters(17)

	wantTuple := &ReachTuple{C: 1, Rf: 2, R: 3, Qf: 4, Q: 5}
	alpha := uint64(123)
	k1, k2, err := GenerateReachTupleKeys(params, alpha, wantTuple)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext([]*dpfpb.DpfParameters{params}, k1)
	if err != nil {
		t.Fatal(err)
	}
	expanded1, err := EvaluateReachTuple(evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext([]*dpfpb.DpfParameters{params}, k2)
	if err != nil {
		t.Fatal(err)
	}
	expanded2, err := EvaluateReachTuple(evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]*ReachTuple, len(expanded1))
	for i := 0; i < len(expanded1); i++ {
		got[i] = &ReachTuple{
			C:  expanded1[i].C + expanded2[i].C,
			Rf: expanded1[i].Rf + expanded2[i].Rf,
			R:  expanded1[i].R + expanded2[i].R,
			Qf: expanded1[i].Qf + expanded2[i].Qf,
			Q:  expanded1[i].Q + expanded2[i].Q,
		}
	}
	want := make([]*ReachTuple, 1<<params.LogDomainSize)
	for i := range want {
		want[i] = &ReachTuple{}
	}
	want[alpha] = wantTuple
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestReachIntModNTupleDpfGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := CreateReachIntModNTupleDpfParameters(17)

	wantTuple := &ReachTuple{C: 1, Rf: 2, R: 3, Qf: 4, Q: 5}
	CreateReachIntModNTuple(wantTuple)
	alpha := uint64(123)
	k1, k2, err := GenerateReachTupleKeys(params, alpha, wantTuple)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext([]*dpfpb.DpfParameters{params}, k1)
	if err != nil {
		t.Fatal(err)
	}
	expanded1, err := EvaluateReachTuple(evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext([]*dpfpb.DpfParameters{params}, k2)
	if err != nil {
		t.Fatal(err)
	}
	expanded2, err := EvaluateReachTuple(evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]*ReachTuple, len(expanded1))
	for i := 0; i < len(expanded1); i++ {
		AddReachIntModNTuple(expanded1[i], expanded2[i])
		got[i] = expanded1[i]
	}
	want := make([]*ReachTuple, 1<<params.LogDomainSize)
	for i := range want {
		want[i] = &ReachTuple{}
	}
	want[alpha] = wantTuple
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}
