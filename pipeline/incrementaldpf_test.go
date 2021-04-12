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
	"testing"

	"github.com/google/go-cmp/cmp"
	pb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

func TestDpfGenEvalFunctions(t *testing.T) {
	params := &pb.DpfParameters{
		LogDomainSize:  20,
		ElementBitsize: 64,
	}
	k1, k2, err := GenerateKeys(params, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx1, err := CreateEvaluationContext(params, k1)
	if err != nil {
		t.Fatal(err)
	}
	expanded1, err := EvaluateNext64(params, []uint64{}, evalCtx1)
	if err != nil {
		t.Fatal(err)
	}

	evalCtx2, err := CreateEvaluationContext(params, k2)
	if err != nil {
		t.Fatal(err)
	}
	expanded2, err := EvaluateNext64(params, []uint64{}, evalCtx2)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]uint64, len(expanded1))
	for i := 0; i < len(expanded1); i++ {
		got[i] = expanded1[i] + expanded2[i]
	}
	want := make([]uint64, 1<<params.GetLogDomainSize())
	want[0] = 1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}
}
