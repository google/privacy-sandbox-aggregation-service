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
	"strings"
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
	ids, err := CalculateBucketID(params, prefixes, []int32{0, 1}, -1)
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
	testHierarchicalGenEvalFunctions(t, false /*unSafe*/)
	testHierarchicalGenEvalFunctions(t, true /*unSafe*/)
}

func testHierarchicalGenEvalFunctions(t *testing.T, useSafe bool) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 2, ValueType: defaultValueType},
		{LogDomainSize: 4, ValueType: defaultValueType},
		{LogDomainSize: 5, ValueType: defaultValueType},
	}
	alpha, beta := uint128.From64(16), uint64(1)
	k1, k2, err := GenerateKeys(params, alpha, []uint64{beta, beta, beta})
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

	var expanded1, expanded2 []uint64
	if useSafe {
		expanded1, err = EvaluateUntil64(0, prefixes[0], evalCtx1)
		if err != nil {
			t.Fatal(err)
		}

		expanded2, err = EvaluateUntil64(0, prefixes[0], evalCtx2)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		prefixes0, prefixesLength0 := CreateCUint128ArrayUnsafe(prefixes[0])
		expanded1, err = EvaluateUntil64Unsafe(0, prefixes0, prefixesLength0, evalCtx1)
		if err != nil {
			t.Fatal(err)
		}

		expanded2, err = EvaluateUntil64Unsafe(0, prefixes0, prefixesLength0, evalCtx2)
		if err != nil {
			t.Fatal(err)
		}
		FreeUnsafePointer(prefixes0)
	}

	// First level of expansion for the first two bits.

	got := make([]uint64, len(expanded1))
	for i := range expanded1 {
		got[i] = expanded1[i] + expanded2[i]
	}
	want := make([]uint64, 1<<params[0].GetLogDomainSize())
	want[2] = 1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}

	// The next level of expansion for all five bits of two prefixes: 0 (00***) and 2 (10***).
	if useSafe {
		expanded1, err = EvaluateUntil64(2, prefixes[1], evalCtx1)
		if err != nil {
			t.Fatal(err)
		}

		expanded2, err = EvaluateUntil64(2, prefixes[1], evalCtx2)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		prefixes1, prefixesLength1 := CreateCUint128ArrayUnsafe(prefixes[1])
		expanded1, err = EvaluateUntil64Unsafe(2, prefixes1, prefixesLength1, evalCtx1)
		if err != nil {
			t.Fatal(err)
		}

		expanded2, err = EvaluateUntil64Unsafe(2, prefixes1, prefixesLength1, evalCtx2)
		if err != nil {
			t.Fatal(err)
		}
		FreeUnsafePointer(prefixes1)
	}

	gotMap := make(map[uint128.Uint128]uint64)
	ids, err := CalculateBucketID(params, [][]uint128.Uint128{
		prefixes[1],
	}, []int32{2}, 0)
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
	testEvaluateAt64(t, true /*useSafe*/)
	testEvaluateAt64(t, false /*useSafe*/)
}

func testEvaluateAt64(t *testing.T, useSafe bool) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 128, ValueType: &dpfpb.ValueType{Type: &dpfpb.ValueType_Integer_{Integer: &dpfpb.ValueType_Integer{Bitsize: 64}}}},
	}
	alpha, beta := uint128.From64(16), uint64(1)
	k1, k2, err := GenerateKeys(params, alpha, []uint64{beta})
	if err != nil {
		t.Fatal(err)
	}

	evaluationPoints := []uint128.Uint128{
		alpha, uint128.From64(0), uint128.From64(1), uint128.Max}

	var evaluated1, evaluated2 []uint64
	if useSafe {
		evaluated1, err = EvaluateAt64(params, 0, evaluationPoints, k1)
		if err != nil {
			t.Fatal(err)
		}
		evaluated2, err = EvaluateAt64(params, 0, evaluationPoints, k2)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		bucketIDs, bucketIDlength := CreateCUint128ArrayUnsafe(evaluationPoints)
		defer FreeUnsafePointer(bucketIDs)
		evaluated1, err = EvaluateAt64Unsafe(params, 0, bucketIDs, bucketIDlength, k1)
		if err != nil {
			t.Fatal(err)
		}
		evaluated2, err = EvaluateAt64Unsafe(params, 0, bucketIDs, bucketIDlength, k2)
		if err != nil {
			t.Fatal(err)
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
			expected = beta
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

	got, err := CalculateBucketID(params, [][]uint128.Uint128{
		{},
		{uint128.From64(1), uint128.From64(3)},
	}, []int32{0, 1}, -1)
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

	got, err = CalculateBucketID(params, [][]uint128.Uint128{
		{uint128.From64(1), uint128.From64(3)},
		{uint128.From64(2), uint128.From64(3), uint128.From64(6)},
	}, []int32{1, 2}, 0)
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

func TestValidateLevels(t *testing.T) {
	if err := validateLevels([]int32{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	levels := []int32{1, 3, 2}
	if err := validateLevels(levels); !strings.Contains(err.Error(), "expect levels in ascending order") {
		t.Fatalf("failure in checking ascending levels in %v", levels)
	}
}

func TestGetVectorLength(t *testing.T) {
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 2, ValueType: defaultValueType},
		{LogDomainSize: 3, ValueType: defaultValueType},
		{LogDomainSize: 4, ValueType: defaultValueType},
	}

	got, err := GetVectorLength(params, [][]uint128.Uint128{
		{},
		{uint128.From64(1), uint128.From64(3)},
	}, []int32{0, 1}, -1)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint64(4); got != want {
		t.Fatalf("expect vector length %d, got %d\n", want, got)
	}

	got, err = GetVectorLength(params, [][]uint128.Uint128{
		{uint128.From64(1), uint128.From64(3)},
		{uint128.From64(2), uint128.From64(3), uint128.From64(6)},
	}, []int32{1, 2}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint64(6); got != want {
		t.Fatalf("expect vector length %d, got %d\n", want, got)
	}
}
