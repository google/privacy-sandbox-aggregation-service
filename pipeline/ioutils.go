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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/storage"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	"github.com/ugorji/go/codec"
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
//
// The file can be stored locally or in the GCS.
func ReadLines(ctx context.Context, filename string) ([]string, error) {
	var scanner *bufio.Scanner
	if strings.HasPrefix(filename, "gs://") {
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
		scanner = bufio.NewScanner(reader)
	} else {
		fs, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer fs.Close()
		scanner = bufio.NewScanner(fs)
	}

	var result []string
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	return result, scanner.Err()
}

// WriteLines writes the input string slice to the output file, one string per line.
//
// The file can be stored locally or in the GCS.
func WriteLines(ctx context.Context, lines []string, filename string) error {
	var buf *bufio.Writer
	if strings.HasPrefix(filename, "gs://") {
		client, err := storage.NewClient(ctx)
		if err != nil {
			return err
		}

		bucket, object, err := ParseGCSPath(filename)
		if err != nil {
			return err
		}
		cw := client.Bucket(bucket).Object(object).NewWriter(ctx)
		defer cw.Close()
		buf = bufio.NewWriter(cw)
	} else {
		fs, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer fs.Close()
		buf = bufio.NewWriter(fs)
	}

	for _, line := range lines {
		if _, err := buf.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	return buf.Flush()
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

func readBytesFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	content := make([]byte, resp.ContentLength)
	if _, err := resp.Body.Read(content); err != nil {
		return nil, err
	}
	return content, nil
}

// ReadBytes reads bytes from a file stored locally, in GCS or served at an URL.
func ReadBytes(ctx context.Context, filename string) ([]byte, error) {
	u, err := url.Parse(filename)
	if err == nil {
		if u.Scheme == "gs" {
			return readGCSObject(ctx, filename)
		} else if u.Scheme == "http" || u.Scheme == "https" {
			return readBytesFromURL(filename)
		}
	}
	return ioutil.ReadFile(filename)
}

// MarshalCBOR serializes the input data in CBOR format.
func MarshalCBOR(v interface{}) ([]byte, error) {
	encBuf := new(bytes.Buffer)
	enc := codec.NewEncoder(encBuf, &codec.CborHandle{})
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return encBuf.Bytes(), nil
}

// UnmarshalCBOR parses the bytes in CBOR format.
func UnmarshalCBOR(b []byte, v interface{}) error {
	decBuf := bytes.NewBuffer(b)
	dec := codec.NewDecoder(decBuf, &codec.CborHandle{})
	return dec.Decode(v)
}

// JoinPath joins the directory and the filename to get the full path of a file.
func JoinPath(directory, filename string) string {
	// Function path.Join does not work for GCS files, for example:
	// path.Join("gs://foo", "bar") returns "gs:/foo/bar"
	if strings.HasPrefix(directory, "gs://") {
		if strings.HasSuffix(directory, "/") {
			return fmt.Sprintf("%s%s", directory, filename)
		}
		return fmt.Sprintf("%s/%s", directory, filename)
	}
	return path.Join(directory, filename)
}

// SaveSecret saves the input payload with Google Cloud Secret Manager.
func SaveSecret(ctx context.Context, payload []byte, projectID, secretID string) (string, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	createSecretReq := &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", projectID),
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}

	secret, err := client.CreateSecret(ctx, createSecretReq)
	if err != nil {
		return "", err
	}

	addSecretVersionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secret.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}

	version, err := client.AddSecretVersion(ctx, addSecretVersionReq)
	if err != nil {
		return "", err
	}
	return version.Name, nil
}

// ReadSecret reads a secret payload from Google Cloud Secret Manager.
func ReadSecret(ctx context.Context, name string) ([]byte, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := client.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return nil, err
	}
	return result.Payload.Data, nil
}
