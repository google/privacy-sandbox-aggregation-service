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

package secretshare

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCombineByteShares(t *testing.T) {
	a := "abcd"
	b := make([]byte, len(a))

	got, err := XorBytes([]byte(a), []byte(a))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(b, got); diff != "" {
		t.Errorf("combined result mismatch (-want +got):\n%s", diff)
	}

	got, err = XorBytes([]byte(a), b)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != a {
		t.Fatalf("combine with zero bits, want %s, got %s", a, string(got))
	}
}

func TestCombineIntShares(t *testing.T) {
	got := CombineIntShares(uint32(1), uint32(1))
	if got != uint32(2) {
		t.Fatalf("want %v, got %v", 2, got)
	}

	a := uint32(1)
	b := uint32(9)
	c := a - b
	got = CombineIntShares(b, c)
	if a != got {
		t.Fatalf("want %v, got %v", a, got)
	}
}
