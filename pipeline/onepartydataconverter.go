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

// Package onepartydataconverter contains functions for simulating browser behavior in the one-party aggregation service design.
package onepartydataconverter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

//ParseRawReport parses a raw conversion into RawReport.
func ParseRawReport(line string) (*reporttypes.RawReport, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return nil, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key128, err := ioutils.StringToUint128(cols[0])
	if err != nil {
		return nil, err
	}

	value64, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return nil, err
	}
	return &reporttypes.RawReport{Bucket: key128, Value: value64}, nil
}

// EncryptReport encrypts an input report with given public keys.
func EncryptReport(report *reporttypes.RawReport, keys []cryptoio.PublicKeyInfo, contextInfo []byte, encryptOutput bool) (*pb.EncryptedReport, error) {
	b, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}

	payload := reporttypes.Payload{
		Operation: "one-party",
		DPFKey:    b,
	}
	bPayload, err := ioutils.MarshalCBOR(payload)
	if err != nil {
		return nil, err
	}

	keyID, key, err := cryptoio.GetRandomPublicKey(keys)
	if err != nil {
		return nil, err
	}

	if !encryptOutput {
		return &pb.EncryptedReport{
			EncryptedReport: &pb.StandardCiphertext{Data: bPayload},
			ContextInfo:     contextInfo,
			KeyId:           keyID,
		}, nil
	}

	encrypted, err := standardencrypt.Encrypt(bPayload, contextInfo, key)
	if err != nil {
		return nil, err
	}
	return &pb.EncryptedReport{EncryptedReport: encrypted, ContextInfo: contextInfo, KeyId: keyID}, nil
}

// FormatEncryptedReport serializes the EncryptedReport into a string.
func FormatEncryptedReport(encrypted *pb.EncryptedReport) (string, error) {
	bEncrypted, err := proto.Marshal(encrypted)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bEncrypted), nil
}

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.EncryptedReport)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*encryptReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*reporttypes.RawReport)(nil)))
	beam.RegisterFunction(formatEncryptedReportFn)
}

// parseRawReportFn parses each line in the raw conversion file in the format: bucket ID, value.
type parseRawReportFn struct {
	countReport beam.Counter
}

func (fn *parseRawReportFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "parserawReportFn_report_count")
}

func (fn *parseRawReportFn) ProcessElement(ctx context.Context, line string, emit func(*reporttypes.RawReport)) error {
	conversion, err := ParseRawReport(line)
	if err != nil {
		return err
	}

	fn.countReport.Inc(ctx, 1)
	emit(conversion)
	return nil
}

type encryptReportFn struct {
	PublicKeys    []cryptoio.PublicKeyInfo
	EncryptOutput bool

	countReport beam.Counter
}

func (fn *encryptReportFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "encryptReportFn_report_count")
}

func (fn *encryptReportFn) ProcessElement(ctx context.Context, c *reporttypes.RawReport, emit func(*pb.EncryptedReport)) error {
	fn.countReport.Inc(ctx, 1)

	encrypted, err := EncryptReport(c, fn.PublicKeys, nil, fn.EncryptOutput)
	if err != nil {
		return err
	}

	emit(encrypted)
	return nil
}

// Since we store and read the data line by line through plain text files, the output is base64-encoded to avoid writing symbols in the proto wire-format that are interpreted as line breaks.
func formatEncryptedReportFn(encrypted *pb.EncryptedReport, emit func(string)) error {
	encryptedStr, err := FormatEncryptedReport(encrypted)
	if err != nil {
		return err
	}
	emit(encryptedStr)
	return nil
}

func writeEncryptedReport(s beam.Scope, output beam.PCollection, outputTextName string, shards int64) {
	s = s.Scope("WriteEncryptedReport")
	formatted := beam.ParDo(s, formatEncryptedReportFn, output)
	ioutils.WriteNShardedFiles(s, outputTextName, shards, formatted)
}

// GenerateEncryptedReportParams contains required parameters for generating partial reports.
type GenerateEncryptedReportParams struct {
	RawReportURI       string
	EncryptedReportURI string
	PublicKeys         []cryptoio.PublicKeyInfo
	Shards             int64

	// EncryptOutput should only be used for integration test before HPKE is ready in Go Tink.
	EncryptOutput bool
}

// GenerateEncryptedReport encrypts the reports with public keys from corresponding helpers.
func GenerateEncryptedReport(scope beam.Scope, params *GenerateEncryptedReportParams) {
	scope = scope.Scope("GenerateEncryptedReport")

	allFiles := ioutils.AddStrInPath(params.RawReportURI, "*")
	lines := textio.Read(scope, allFiles)

	rawReports := beam.ParDo(scope, &parseRawReportFn{}, lines)
	resharded := beam.Reshuffle(scope, rawReports)

	encrypted := beam.ParDo(scope, &encryptReportFn{PublicKeys: params.PublicKeys}, resharded)

	writeEncryptedReport(scope, encrypted, params.EncryptedReportURI, params.Shards)
}
