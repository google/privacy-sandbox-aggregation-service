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

// Package onepartyprocess processes data for the one-party service design.
package onepartyprocess

import (
	"context"
	"reflect"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/report/reportutils"
	"github.com/google/privacy-sandbox-aggregation-service/tools/onepartyconvert"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

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

func (fn *parseRawReportFn) ProcessElement(ctx context.Context, line string, emit func(reporttypes.RawReport)) error {
	conversion, err := reportutils.ParseRawReport(line, 128 /*keyBitSize*/)
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

func (fn *encryptReportFn) ProcessElement(ctx context.Context, c reporttypes.RawReport, emit func(*pb.EncryptedReport)) error {
	fn.countReport.Inc(ctx, 1)

	encrypted, err := onepartyconvert.EncryptReport(c, fn.PublicKeys, nil, fn.EncryptOutput)
	if err != nil {
		return err
	}

	emit(encrypted)
	return nil
}

// Since we store and read the data line by line through plain text files, the output is base64-encoded to avoid writing symbols in the proto wire-format that are interpreted as line breaks.
func formatEncryptedReportFn(encrypted *pb.EncryptedReport, emit func(string)) error {
	encryptedStr, err := onepartyconvert.FormatEncryptedReport(encrypted)
	if err != nil {
		return err
	}
	emit(encryptedStr)
	return nil
}

func writeEncryptedReport(s beam.Scope, output beam.PCollection, outputTextName string, shards int64) {
	s = s.Scope("WriteEncryptedReport")
	formatted := beam.ParDo(s, formatEncryptedReportFn, output)
	pipelineutils.WriteNShardedFiles(s, outputTextName, shards, formatted)
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

	allFiles := utils.AddStrInPath(params.RawReportURI, "*")
	lines := textio.Read(scope, allFiles)

	rawReports := beam.ParDo(scope, &parseRawReportFn{}, lines)
	resharded := beam.Reshuffle(scope, rawReports)

	encrypted := beam.ParDo(scope, &encryptReportFn{PublicKeys: params.PublicKeys}, resharded)

	writeEncryptedReport(scope, encrypted, params.EncryptedReportURI, params.Shards)
}
