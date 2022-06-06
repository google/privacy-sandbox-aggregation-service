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

package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"lukechampine.com/uint128"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

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

func TestParsePubSubResourceName(t *testing.T) {
	type parseResult struct {
		Input, Project, Name string
		ErrStr               string
	}
	for _, want := range []parseResult{
		{
			Input:   "projects/myproject/topics/mytopic",
			Project: "myproject",
			Name:    "mytopic",
		},
		{
			Input:   "projects/myproject/topics",
			Project: "",
			Name:    "",
			ErrStr:  fmt.Sprintf("expect format %s, got %s", "projects/project-identifier/collection/relative-name", "projects/myproject/topics"),
		},
		{
			Input:   "projects/myproject/foo/mytopic",
			Project: "",
			Name:    "",
			ErrStr:  fmt.Sprintf("expect format %s, got %s", "projects/project-identifier/collection/relative-name", "projects/myproject/foo/mytopic"),
		},
	} {
		project, name, err := ParsePubSubResourceName(want.Input)
		var errStr string
		if err != nil {
			errStr = err.Error()
		}
		if want.ErrStr != errStr {
			t.Errorf("expect error message %s, got %s", want.ErrStr, errStr)
		}

		if project != want.Project || name != want.Name {
			t.Errorf("want project %q and name %q, got %q, and %q", want.Project, want.Name, project, name)
		}
	}
}

func TestIsFileGlobExist(t *testing.T) {
	fileDir, err := ioutil.TempDir("/tmp", "test-file")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	ctx := context.Background()

	if err := WriteBytes(ctx, []byte("somedata"), path.Join(fileDir, "input_1.txt")); err != nil {
		t.Fatal(err)
	}
	if err := WriteBytes(ctx, []byte("somedata"), path.Join(fileDir, "input_2.txt")); err != nil {
		t.Fatal(err)
	}

	if exist, err := IsFileGlobExist(ctx, path.Join(fileDir, "input_1.txt")); err != nil {
		t.Fatal(err)
	} else if want, got := true, exist; want != got {
		t.Fatalf("glob existence wrong, want %v got %v", want, got)
	}

	if exist, err := IsFileGlobExist(ctx, path.Join(fileDir, "input*.txt")); err != nil {
		t.Fatal(err)
	} else if want, got := true, exist; want != got {
		t.Fatalf("glob existence wrong, want %v got %v", want, got)
	}

	if exist, err := IsFileGlobExist(ctx, path.Join(fileDir, "no_such_file*.txt")); err != nil {
		t.Fatal(err)
	} else if want, got := false, exist; want != got {
		t.Fatalf("glob existence wrong, want %v got %v", want, got)
	}
}

func TestIntegerToByteString(t *testing.T) {
	want128 := uint128.New(123, 456)
	b128 := Uint128ToBigEndianBytes(want128)
	got128, err := BigEndianBytesToUint128(b128)
	if err != nil {
		t.Fatal(err)
	}
	if !want128.Equals(got128) {
		t.Errorf("uint128 conversion failed: want %s, got %s", want128.String(), got128.String())
	}

	want32 := uint32(123)
	b32 := Uint32ToBigEndianBytes(want32)
	got32, err := BigEndianBytesToUint32(b32)
	if err != nil {
		t.Fatal(err)
	}
	if want32 != got32 {
		t.Errorf("uint32 conversion failed: want %d, got %d", want32, got32)
	}
}
