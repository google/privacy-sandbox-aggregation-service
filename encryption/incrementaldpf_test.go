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
	"lukechampine.com/uint128"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

var defaultValueType = &dpfpb.ValueType{
	Type: &dpfpb.ValueType_Integer_{
		Integer: &dpfpb.ValueType_Integer{
			Bitsize: DefaultElementBitSize,
		},
	},
}

func TestDpfGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 20, ValueType: defaultValueType},
	}
	k1, k2, err := GenerateKeys(params, uint128.Uint128{}, []uint64{1})
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext(params, k1)
	if err != nil {
		t.Fatal(err)
	}
	expanded1, err := EvaluateNext64([]uint128.Uint128{}, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext(params, k2)
	if err != nil {
		t.Fatal(err)
	}
	expanded2, err := EvaluateNext64([]uint128.Uint128{}, evalCtx2)
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
		{LogDomainSize: 2, ValueType: defaultValueType},
		{LogDomainSize: 4, ValueType: defaultValueType},
	}
	alpha, beta := uint128.From64(8), uint64(1)
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

	prefixes := [][]uint128.Uint128{
		{},
		{uint128.From64(0), uint128.From64(2)},
	}
	// First level of expansion for the first two bits.
	expanded1, err := EvaluateNext64(prefixes[0], evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err := EvaluateNext64(prefixes[0], evalCtx2)
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
	expanded1, err = EvaluateNext64(prefixes[1], evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err = EvaluateNext64(prefixes[1], evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	gotMap := make(map[uint128.Uint128]uint64)
	ids, err := CalculateBucketID(params, prefixes[1], 1, 0)
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
	wantMap := make(map[uint128.Uint128]uint64)
	wantMap[alpha] = beta
	if diff := cmp.Diff(wantMap, gotMap); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestDpfMultiLevelHierarchicalGenEvalFunctions(t *testing.T) {
	testHierarchicalGenEvalFunctions(t, false /*useSafe*/, false /*useDefault*/)
	testHierarchicalGenEvalFunctions(t, true /*useSafe*/, true /*useDefault*/)
	testHierarchicalGenEvalFunctions(t, true /*useSafe*/, false /*useDefault*/)
}

func testHierarchicalGenEvalFunctions(t *testing.T, useSafe, useDefault bool) {
	os.Setenv("GODEBUG", "cgocheck=2")

	const keyBitSize = 5

	alpha, beta := uint128.From64(16), uint64(1)
	params := make([]*dpfpb.DpfParameters, keyBitSize)
	betas := make([]uint64, keyBitSize)
	for i := 0; i < keyBitSize; i++ {
		params[i] = &dpfpb.DpfParameters{LogDomainSize: int32(i + 1), ValueType: defaultValueType}
		betas[i] = beta
	}

	k1, k2, err := GenerateKeys(params, alpha, betas)
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

	prefixes := [][]uint128.Uint128{
		{},
		{uint128.From64(0), uint128.From64(2)},
		{uint128.From64(1)},
	}

	// First level of expansion for the first two bits.
	h0 := 1
	var expanded1, expanded2 []uint64
	if useSafe {
		expanded1, err = EvaluateUntil64(h0, prefixes[0], evalCtx1)
		if err != nil {
			t.Fatal(err)
		}

		expanded2, err = EvaluateUntil64(h0, prefixes[0], evalCtx2)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		prefixes0, prefixesLength0 := CreateCUint128ArrayUnsafe(prefixes[0])
		if useDefault {
			evalCtx1.Parameters = nil
			expanded1, err = EvaluateUntil64UnsafeDefault(keyBitSize, h0, prefixes0, prefixesLength0, evalCtx1)
			if err != nil {
				t.Fatal(err)
			}

			evalCtx2.Parameters = nil
			expanded2, err = EvaluateUntil64UnsafeDefault(keyBitSize, h0, prefixes0, prefixesLength0, evalCtx2)
			if err != nil {
				t.Fatal(err)
			}

		} else {
			expanded1, err = EvaluateUntil64Unsafe(h0, prefixes0, prefixesLength0, evalCtx1)
			if err != nil {
				t.Fatal(err)
			}

			expanded2, err = EvaluateUntil64Unsafe(h0, prefixes0, prefixesLength0, evalCtx2)
			if err != nil {
				t.Fatal(err)
			}

		}
		FreeUnsafePointer(prefixes0)
	}

	got := make([]uint64, len(expanded1))
	for i := range expanded1 {
		got[i] = expanded1[i] + expanded2[i]
	}
	want := make([]uint64, 1<<params[1].GetLogDomainSize())
	want[2] = 1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}

	// The next level of expansion for all five bits of two prefixes: 0 (00***) and 2 (10***).
	h1 := 4
	if useSafe {
		expanded1, err = EvaluateUntil64(h1, prefixes[1], evalCtx1)
		if err != nil {
			t.Fatal(err)
		}

		expanded2, err = EvaluateUntil64(h1, prefixes[1], evalCtx2)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		prefixes1, prefixesLength1 := CreateCUint128ArrayUnsafe(prefixes[1])
		if useDefault {
			evalCtx1.Parameters = nil
			expanded1, err = EvaluateUntil64UnsafeDefault(keyBitSize, h1, prefixes1, prefixesLength1, evalCtx1)
			if err != nil {
				t.Fatal(err)
			}

			evalCtx2.Parameters = nil
			expanded2, err = EvaluateUntil64UnsafeDefault(keyBitSize, h1, prefixes1, prefixesLength1, evalCtx2)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			expanded1, err = EvaluateUntil64Unsafe(h1, prefixes1, prefixesLength1, evalCtx1)
			if err != nil {
				t.Fatal(err)
			}

			expanded2, err = EvaluateUntil64Unsafe(h1, prefixes1, prefixesLength1, evalCtx2)
			if err != nil {
				t.Fatal(err)
			}

		}
		FreeUnsafePointer(prefixes1)
	}

	gotMap := make(map[uint128.Uint128]uint64)
	ids, err := CalculateBucketID(params, prefixes[1], int32(h1), 1)
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
	wantMap := make(map[uint128.Uint128]uint64)
	wantMap[alpha] = beta
	if diff := cmp.Diff(wantMap, gotMap); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestEvaluateAt64(t *testing.T) {
	testEvaluateAt64(t, true /*useSafe*/, false /*useDefault*/)
	testEvaluateAt64(t, false /*useSafe*/, false /*useDefault*/)
	testEvaluateAt64(t, false /*useSafe*/, true /*useDefault*/)
}

func testEvaluateAt64(t *testing.T, useSafe, useDefault bool) {
	os.Setenv("GODEBUG", "cgocheck=2")

	const keyBitSize = 128
	params := make([]*dpfpb.DpfParameters, keyBitSize)
	alpha := uint128.From64(16)
	betas := make([]uint64, keyBitSize)
	for i := 0; i < keyBitSize; i++ {
		params[i] = &dpfpb.DpfParameters{LogDomainSize: int32(i + 1), ValueType: &dpfpb.ValueType{Type: &dpfpb.ValueType_Integer_{Integer: &dpfpb.ValueType_Integer{Bitsize: 64}}}}
		betas[i] = 1
	}
	k1, k2, err := GenerateKeys(params, alpha, betas)
	if err != nil {
		t.Fatal(err)
	}

	evaluationPoints := []uint128.Uint128{
		alpha, uint128.From64(0), uint128.From64(1), uint128.Max}

	var evaluated1, evaluated2 []uint64
	if useSafe {
		evaluated1, err = EvaluateAt64(params, keyBitSize-1, evaluationPoints, k1)
		if err != nil {
			t.Fatal(err)
		}
		evaluated2, err = EvaluateAt64(params, keyBitSize-1, evaluationPoints, k2)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		bucketIDs, bucketIDlength := CreateCUint128ArrayUnsafe(evaluationPoints)
		defer FreeUnsafePointer(bucketIDs)
		if useDefault {
			evaluated1, err = EvaluateAt64UnsafeDefault(keyBitSize, keyBitSize-1, bucketIDs, bucketIDlength, k1)
			if err != nil {
				t.Fatal(err)
			}
			evaluated2, err = EvaluateAt64UnsafeDefault(keyBitSize, keyBitSize-1, bucketIDs, bucketIDlength, k2)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			evaluated1, err = EvaluateAt64Unsafe(params, keyBitSize-1, bucketIDs, bucketIDlength, k1)
			if err != nil {
				t.Fatal(err)
			}
			evaluated2, err = EvaluateAt64Unsafe(params, keyBitSize-1, bucketIDs, bucketIDlength, k2)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	if len(evaluated1) != len(evaluated2) {
		t.Fatalf("Evaluated arrays have different lengths")
	}
	if len(evaluated1) != len(evaluationPoints) {
		t.Fatalf("Size of evaluated array differs from the number of evaluation points")
	}

	for i := range evaluated1 {
		var expected uint64
		if evaluationPoints[i] == alpha {
			expected = 1
		} else {
			expected = 0
		}
		if evaluated1[i]+evaluated2[i] != expected {
			t.Fatalf("Expected %d, got %d", expected, evaluated1[i]+evaluated2[i])
		}
	}

}

func TestCalculateBucketID(t *testing.T) {
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 2, ValueType: defaultValueType},
		{LogDomainSize: 3, ValueType: defaultValueType},
		{LogDomainSize: 4, ValueType: defaultValueType},
	}

	got, err := CalculateBucketID(params, []uint128.Uint128{uint128.From64(1), uint128.From64(3)}, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]uint128.Uint128{
		uint128.From64(2),
		uint128.From64(3),
		uint128.From64(6),
		uint128.From64(7),
	}, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}

	got, err = CalculateBucketID(params, []uint128.Uint128{uint128.From64(2), uint128.From64(3), uint128.From64(6)}, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]uint128.Uint128{
		uint128.From64(4),
		uint128.From64(5),
		uint128.From64(6),
		uint128.From64(7),
		uint128.From64(12),
		uint128.From64(13),
	}, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestGetVectorLength(t *testing.T) {
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 2, ValueType: defaultValueType},
		{LogDomainSize: 3, ValueType: defaultValueType},
		{LogDomainSize: 4, ValueType: defaultValueType},
	}

	got, err := GetVectorLength(params, []uint128.Uint128{uint128.From64(1), uint128.From64(3)}, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint64(4); got != want {
		t.Fatalf("expect vector length %d, got %d\n", want, got)
	}

	got, err = GetVectorLength(params, []uint128.Uint128{uint128.From64(2), uint128.From64(3), uint128.From64(6)}, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint64(6); got != want {
		t.Fatalf("expect vector length %d, got %d\n", want, got)
	}
}

func TestReachUint64TupleDpfGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := CreateReachUint64TupleDpfParameters(17)

	wantTuple := &ReachTuple{C: 1, Rf: 2, R: 3, Qf: 4, Q: 5}
	alpha := uint128.From64(123)
	k1, k2, err := GenerateReachTupleKeys([]*dpfpb.DpfParameters{params}, alpha, []*ReachTuple{wantTuple})
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
	want[alpha.Lo] = wantTuple
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestReachIntModNTupleDpfGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := CreateReachIntModNTupleDpfParameters(17)

	wantTuple := &ReachTuple{C: 1, Rf: 2, R: 3, Qf: 4, Q: 5}
	CreateReachIntModNTuple(wantTuple)
	alpha := uint128.From64(123)
	k1, k2, err := GenerateReachTupleKeys([]*dpfpb.DpfParameters{params}, alpha, []*ReachTuple{wantTuple})
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
	want[alpha.Lo] = wantTuple
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestReachUint64TupleDpfGenEvalFullHierarchy(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	keyBitSize := 128
	prefixLevel, evalLevel := 100, 102

	params, err := getTupleDPFParametersFullHierarchy(keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	wantTuple := &ReachTuple{C: 1, Rf: 2, R: 3, Qf: 4, Q: 5}
	betas := make([]*ReachTuple, keyBitSize)
	for i := range betas {
		betas[i] = wantTuple
	}

	nonZeroIndex := uint64(100663296)
	alpha := uint128.From64(nonZeroIndex)
	k1, k2, err := GenerateReachTupleKeys(params, alpha, betas)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext(params, k1)
	if err != nil {
		t.Fatal(err)
	}
	expanded1, err := EvaluateReachTupleBetweenLevels(evalCtx1, prefixLevel, evalLevel)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext(params, k2)
	if err != nil {
		t.Fatal(err)
	}
	expanded2, err := EvaluateReachTupleBetweenLevels(evalCtx2, prefixLevel, evalLevel)
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

	wantNonZeroIndex := nonZeroIndex >> (keyBitSize - evalLevel - 1)
	wantSize := 1 << (params[evalLevel].LogDomainSize - params[prefixLevel].LogDomainSize)
	want := make([]*ReachTuple, wantSize)
	for i := range want {
		want[i] = &ReachTuple{}
	}
	want[wantNonZeroIndex] = wantTuple
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}

func TestReachUint64TupleDpfGenEvalSpecifiedHierarchy(t *testing.T) {
	for _, s := range []struct {
		KeyBitSize, PrefixBitSize, EvalBitSize int
		Alpha                                  uint128.Uint128
	}{
		{
			KeyBitSize:    128,
			PrefixBitSize: 101,
			EvalBitSize:   103,
			Alpha:         uint128.From64(100663296),
		},
		{
			KeyBitSize:    128,
			PrefixBitSize: 0,
			EvalBitSize:   2,
			Alpha:         uint128.New(0, 13835058055282163712),
		},
		{
			KeyBitSize:    128,
			PrefixBitSize: 126,
			EvalBitSize:   128,
			Alpha:         uint128.New(3, 0),
		},
	} {
		testReachUint64TupleDpfGenEvalSpecifiedHierarchy(t, s.PrefixBitSize, s.EvalBitSize, s.KeyBitSize, s.Alpha)
	}
}

func testReachUint64TupleDpfGenEvalSpecifiedHierarchy(t *testing.T, prefixBitSize, evalBitSize, keyBitSize int, alpha uint128.Uint128) {
	os.Setenv("GODEBUG", "cgocheck=2")

	params, err := getTupleDPFParametersSelectedHierarchy(prefixBitSize, evalBitSize, keyBitSize)
	if err != nil {
		t.Fatal(err)
	}

	wantTuple := &ReachTuple{C: 1, Rf: 2, R: 3, Qf: 4, Q: 5}
	betas := make([]*ReachTuple, len(params))
	for i := range betas {
		betas[i] = wantTuple
	}

	k1, k2, err := GenerateReachTupleKeys(params, alpha, betas)
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

	level1, level2 := -1, 0
	if prefixBitSize > 0 {
		level1, level2 = 0, 1
	}

	expanded1, err := EvaluateReachTupleBetweenLevels(evalCtx1, level1, level2)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err := EvaluateReachTupleBetweenLevels(evalCtx2, level1, level2)
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

	// wantNonZeroIndex := nonZeroIndex >> (keyBitSize - evalBitSize)
	wantNonZeroIndex := alpha.Rsh(uint(keyBitSize - evalBitSize)).Lo
	wantSize := 1 << (evalBitSize - prefixBitSize)
	want := make([]*ReachTuple, wantSize)
	for i := range want {
		want[i] = &ReachTuple{}
	}
	want[wantNonZeroIndex] = wantTuple
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}
