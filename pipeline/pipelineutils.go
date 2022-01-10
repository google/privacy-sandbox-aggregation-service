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

// Package pipelineutils contains utilities used by the beam pipelines.
package pipelineutils

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*addShardKeyFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*getShardFn)(nil)).Elem())
}

type addShardKeyFn struct {
	TotalShards int64
}

func (fn *addShardKeyFn) ProcessElement(line string, emit func(int64, string)) {
	emit(rand.Int63n(fn.TotalShards), line)
}

type getShardFn struct {
	Shard int64
}

func (fn *getShardFn) ProcessElement(key int64, line string, emit func(string)) {
	if fn.Shard == key {
		emit(line)
	}
}

// WriteNShardedFiles writes the text files in shards.
func WriteNShardedFiles(s beam.Scope, outputName string, n int64, lines beam.PCollection) {
	s = s.Scope("WriteNShardedFiles")

	if n == 1 {
		textio.Write(s, outputName, lines)
		return
	}
	keyed := beam.ParDo(s, &addShardKeyFn{TotalShards: n}, lines)
	for i := int64(0); i < n; i++ {
		shard := beam.ParDo(s, &getShardFn{Shard: i}, keyed)
		textio.Write(s, AddStrInPath(outputName, fmt.Sprintf("-%d-%d", i+1, n)), shard)
	}
}

// AddStrInPath adds a string in the file name before the file extension.
//
// For example: addStringInPath("/foo/x.bar", "_baz") = "/foo/x_baz.bar"
func AddStrInPath(path, str string) string {
	ext := filepath.Ext(path)
	return path[:len(path)-len(ext)] + str + ext
}
