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

// Package utils contains basic utilities.
package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/golang/glog"
	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/apache/beam/sdks/go/pkg/beam/io/filesystem"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/idtoken"
	"github.com/ugorji/go/codec"
	"lukechampine.com/uint128"

	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"

	// The following packages are required to read files from GCS or local.
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/gcs"
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

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
		// create all dirs if not existing, ignore errors
		idx := strings.LastIndex(filename, "/")
		if idx != -1 {
			os.MkdirAll(filename[:idx], os.ModePerm)
		}

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
	if _, err := writer.Write(data); err != nil {
		return err
	}

	return writer.Close()
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

// StringToUint128 converts a string of decimal number to an uint128 integer.
func StringToUint128(str string) (uint128.Uint128, error) {
	n, ok := (&big.Int{}).SetString(str, 10)
	if !ok {
		return uint128.Uint128{}, fmt.Errorf("function SetString(%s) failed", str)
	}
	return uint128.FromBig(n), nil
}

// BigEndianBytesToUint128 converts a big-ending byte string to a 128-bit integer.
func BigEndianBytesToUint128(b []byte) (uint128.Uint128, error) {
	if want, got := 16, len(b); want != got {
		return uint128.Uint128{}, fmt.Errorf("expect %d bytes, got %d", want, got)
	}
	return uint128.New(binary.BigEndian.Uint64(b[8:16]), binary.BigEndian.Uint64(b[0:8])), nil
}

// Uint128ToBigEndianBytes encodes a 128-bit integer to a big-ending byte string.
func Uint128ToBigEndianBytes(i uint128.Uint128) []byte {
	lo := make([]byte, 8)
	binary.BigEndian.PutUint64(lo, i.Lo)
	hi := make([]byte, 8)
	binary.BigEndian.PutUint64(hi, i.Hi)
	hi = append(hi, lo...)
	return hi
}

// BigEndianBytesToUint32 converts a big-ending byte string to a 32-bit integer.
func BigEndianBytesToUint32(b []byte) (uint32, error) {
	if want, got := 4, len(b); want != got {
		return uint32(0), fmt.Errorf("expect %d bytes, got %d", want, got)
	}
	return binary.BigEndian.Uint32(b), nil
}

// Uint32ToBigEndianBytes encodes a 32-bit integer to a big-ending byte string.
func Uint32ToBigEndianBytes(i uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, i)
	return b
}

// RunfilesPath gets the paths of files based on a rooted file system for the tests.
func RunfilesPath(path string, isBinary bool) (string, error) {
	if isBinary {
		path = fmt.Sprintf("%s_/%s", path, filepath.Base(path))
	}
	return bazel.Runfile(path)
}

// GetAuthorizationToken gets GCP service auth token based env service account or impersonated service account through default credentials
func GetAuthorizationToken(ctx context.Context, audience, impersonatedSvcAccount string) (string, error) {
	// TODO Switch to this implementation once google api upgraded to v0.52.0+
	// tokenSource, err = impersonate.IDTokenSource(ctx, impersonate.IDTokenConfig{
	// 	Audience:        audience,
	// 	TargetPrincipal: impersonatedSvcAccount,
	// 	IncludeEmail:    true,
	// })
	// if err != nil {
	// 	return nil, err
	// }
	token := ""
	// First we try the idtoken package, which only works for service accounts
	tokenSource, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		if !strings.Contains(err.Error(), `idtoken: credential must be service_account, found`) {
			return token, err
		}
		if impersonatedSvcAccount == "" {
			return token, fmt.Errorf("Couldn't obtain Auth Token, no svc account for impersonation set (flag 'impersonated_svc_account'): %v", err)
		}

		log.Info("no service account found, using application default credentials to impersonate service account")
		svc, err := iamcredentials.NewService(ctx)
		if err != nil {
			return token, err
		}
		resp, err := svc.Projects.ServiceAccounts.GenerateIdToken("projects/-/serviceAccounts/"+impersonatedSvcAccount, &iamcredentials.GenerateIdTokenRequest{
			Audience: audience,
		}).Do()
		if err != nil {
			return token, err
		}
		token = resp.Token

	} else {
		t, err := tokenSource.Token()
		if err != nil {
			return token, fmt.Errorf("TokenSource.Token: %v", err)
		}
		token = t.AccessToken
	}
	return token, nil
}

// IsGCSObjectExist checks if a GCS object exists.
func IsGCSObjectExist(ctx context.Context, client *storage.Client, filename string) (bool, error) {
	bucket, object, err := ParseGCSPath(filename)
	if err != nil {
		return false, err
	}
	_, err = client.Bucket(bucket).Object(object).Attrs(ctx)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}
	return false, err
}

// PublishRequest publishes on a topic with the aggregation request as the content.
func PublishRequest(ctx context.Context, client *pubsub.Client, pubsubTopic string, content interface{}) error {
	topic := client.Topic(pubsubTopic)

	b, err := json.Marshal(content)
	if err != nil {
		return err
	}
	log.Infof("topic: %s; request: %s", pubsubTopic, string(b))

	_, err = topic.Publish(ctx, &pubsub.Message{Data: b}).Get(ctx)
	return err
}

// ParsePubSubResourceName parses the PubSub resource name and get the project ID and topic or subscription.
//
// Details about the resource names: https://cloud.google.com/pubsub/docs/admin#resource_names
func ParsePubSubResourceName(name string) (projectID, relativeName string, err error) {
	strs := strings.Split(name, "/")
	if len(strs) != 4 || strs[0] != "projects" || (strs[2] != "subscriptions" && strs[2] != "topics") {
		err = fmt.Errorf("expect format %s, got %s", "projects/project-identifier/collection/relative-name", name)
		return
	}
	projectID, relativeName = strs[1], strs[3]
	return
}

// IsFileGlobExist checks if there is any file that matches the input pattern.
func IsFileGlobExist(ctx context.Context, glob string) (bool, error) {
	if strings.TrimSpace(glob) == "" {
		return false, nil
	}
	fs, err := filesystem.New(ctx, glob)
	if err != nil {
		return false, err
	}
	defer fs.Close()

	files, err := fs.List(ctx, glob)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}
