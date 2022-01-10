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

package pipelineutils

import (
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

	got := textio.Read(scope, AddStrInPath(outputName, "*"))
	passert.Equals(scope, got, want)
	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}
