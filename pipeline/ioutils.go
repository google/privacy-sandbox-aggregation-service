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

// Package ioutils contains utilities for reading/writing files with the beam pipelines.
package ioutils

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"cloud.google.com/go/storage"
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

// ParseGCSPath gets the bucket and object names from the input filename.
func ParseGCSPath(filename string) (bucket, object string, err error) {
	parsed, err := url.Parse(filename)
	if err != nil {
		return
	}
	if parsed.Scheme != "gs" {
		err = fmt.Errorf("object %q must have 'gs' scheme", filename)
		return
	}
	if parsed.Host == "" {
		err = fmt.Errorf("object %q must have bucket", filename)
		return
	}

	bucket = parsed.Host
	if parsed.Path != "" {
		object = parsed.Path[1:]
	}
	return
}

// ReadLines reads the input file line by line and returns the content as a slice of strings.
func ReadLines(filename string) ([]string, error) {
	fs, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	var lines []string
	scanner := bufio.NewScanner(fs)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// TODO: Add a unit test for writing and reading files in GCS buckets
func writeGCSObject(ctx context.Context, data []byte, filename string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	bucket, object, err := ParseGCSPath(filename)
	if err != nil {
		return err
	}
	writer := client.Bucket(bucket).Object(object).NewWriter(ctx)
	defer writer.Close()
	_, err = writer.Write(data)
	return err
}

func readGCSObject(ctx context.Context, filename string) ([]byte, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	bucket, object, err := ParseGCSPath(filename)
	if err != nil {
		return nil, err
	}
	reader, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return ioutil.ReadAll(reader)
}

// WriteBytes writes bytes into a local or GCS file.
func WriteBytes(ctx context.Context, data []byte, filename string) error {
	if strings.HasPrefix(filename, "gs://") {
		return writeGCSObject(ctx, data, filename)
	}
	return ioutil.WriteFile(filename, data, os.ModePerm)
}

// ReadBytes reads bytes from a local or GCS file.
func ReadBytes(ctx context.Context, filename string) ([]byte, error) {
	if strings.HasPrefix(filename, "gs://") {
		return readGCSObject(ctx, filename)
	}
	return ioutil.ReadFile(filename)
}
