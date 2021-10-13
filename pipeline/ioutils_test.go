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

package ioutils

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

func TestAddStrInPath(t *testing.T) {
	for _, a := range []struct {
		Path, Str, Want string
	}{
		{Path: "gs://foo/.bar/x.bar", Str: "_baz", Want: "gs://foo/.bar/x_baz.bar"},
		{Path: "/foo/.bar/x.bar", Str: "_baz", Want: "/foo/.bar/x_baz.bar"},
	} {
		got := AddStrInPath(a.Path, a.Str)
		if got != a.Want {
			t.Fatalf("want result %q, got %q", a.Want, got)
		}
	}
}

func TestWriteNShardedFiles(t *testing.T) {
	storageDir, err := ioutil.TempDir("/tmp", "test-shards")
	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err)
	}
	defer os.RemoveAll(storageDir)

	var wantStr []string
	for i := int64(0); i < 100; i++ {
		wantStr = append(wantStr, strconv.FormatInt(i, 10))
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	want := beam.CreateList(scope, wantStr)

	fileName := "/output.txt"
	outputName := path.Join(storageDir, fileName)

	shards := int64(10)
	wantFiles := []string{
		storageDir + "/output-1-10.txt",
		storageDir + "/output-2-10.txt",
		storageDir + "/output-3-10.txt",
		storageDir + "/output-4-10.txt",
		storageDir + "/output-5-10.txt",
		storageDir + "/output-6-10.txt",
		storageDir + "/output-7-10.txt",
		storageDir + "/output-8-10.txt",
		storageDir + "/output-9-10.txt",
		storageDir + "/output-10-10.txt",
	}
	WriteNShardedFiles(scope, outputName, shards, want)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}

	gotFiles, err := filepath.Glob(AddStrInPath(outputName, "*"))
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(wantFiles, gotFiles, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Fatalf("files mismatch (-want +got):\n%s", diff)
	}

	got := textio.ReadSdf(scope, AddStrInPath(outputName, "*"))
	passert.Equals(scope, got, want)
	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func TestWriteReadLines(t *testing.T) {
	fileDir, err := ioutil.TempDir("/tmp", "test-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	want := []string{"foo", "bar", "baz"}
	resultFile := path.Join(fileDir, "result.txt")
	ctx := context.Background()
	if err := WriteLines(ctx, want, resultFile); err != nil {
		t.Fatal(err)
	}

	got, err := ReadLines(ctx, resultFile)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("strings mismatch (-want +got):\n%s", diff)
	}
}

func TestCborMarshalUnmarshal(t *testing.T) {
	type testStruct struct {
		FieldStr   string `json:"field_str"`
		FieldInt   int64  `json:"field_int"`
		FieldBytes []byte `json:"field_bytes"`
	}

	want := &testStruct{
		FieldStr:   "test_string",
		FieldInt:   12345,
		FieldBytes: []byte("test_bytes"),
	}

	b, err := MarshalCBOR(want)
	if err != nil {
		t.Fatal(err)
	}

	got := &testStruct{}
	if err := UnmarshalCBOR(b, got); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unmarshaled message mismatch (-want +got):\n%s", diff)
	}
}

func TestJoinPath(t *testing.T) {
	filename := "bar"

	dirGCS := "gs://foo"
	want := dirGCS + "/" + filename
	got := JoinPath(dirGCS, filename)
	if want != got {
		t.Errorf("expect joint path %s, got %s", want, got)
	}

	dirGCS = "gs://foo/"
	want = dirGCS + filename
	got = JoinPath(dirGCS, filename)
	if want != got {
		t.Errorf("expect joint path %s, got %s", want, got)
	}

	dirLocal := "/foo"
	want = dirLocal + "/" + filename
	got = JoinPath(dirLocal, filename)
	if want != got {
		t.Errorf("expect joint path %s, got %s", want, got)
	}

	dirLocal = "/foo/"
	want = dirLocal + filename
	got = JoinPath(dirLocal, filename)
	if want != got {
		t.Errorf("expect joint path %s, got %s", want, got)
	}
}

func TestStringToUint128(t *testing.T) {
	want := "147573952589676412928" // 2^67
	n, err := StringToUint128(want)
	if err != nil {
		t.Fatal(err)
	}
	got := n.String()
	if want != got {
		t.Fatalf("expect %q, got %q", want, got)
	}

	want = "xyz"
	n, err = StringToUint128(want)
	if err == nil {
		t.Fatalf("failed to detect invalid input %q", want)
	}
}

func TestStringToUint128Overflow(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("failed to panic on a value overflows uint128")
		}
	}()
	StringToUint128("680564733841876926926749214863536422912") // 2^129
}

func TestStringToUint128Negative(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("failed to panic on a negative value")
		}
	}()
	StringToUint128("-147573952589676412928") // -2^67
}
