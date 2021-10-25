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

func TestDpfGenEvalFunctions(t *testing.T) {
	os.Setenv("GODEBUG", "cgocheck=2")
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 20, ElementBitsize: 64},
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
		{LogDomainSize: 2, ElementBitsize: 64},
		{LogDomainSize: 4, ElementBitsize: 64},
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
	os.Setenv("GODEBUG", "cgocheck=2")
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 2, ElementBitsize: 64},
		{LogDomainSize: 4, ElementBitsize: 64},
		{LogDomainSize: 5, ElementBitsize: 64},
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
	// First level of expansion for the first two bits.
	expanded1, err := EvaluateUntil64(0, prefixes[0], evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err := EvaluateUntil64(0, prefixes[0], evalCtx2)
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

	// The next level of expansion for all five bits of two prefixes: 0 (00***) and 2 (10***).
	expanded1, err = EvaluateUntil64(2, prefixes[1], evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	expanded2, err = EvaluateUntil64(2, prefixes[1], evalCtx2)
	if err != nil {
		t.Fatal(err)
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

func TestCalculateBucketID(t *testing.T) {
	params := []*dpfpb.DpfParameters{
		{LogDomainSize: 2, ElementBitsize: 64},
		{LogDomainSize: 3, ElementBitsize: 64},
		{LogDomainSize: 4, ElementBitsize: 64},
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
