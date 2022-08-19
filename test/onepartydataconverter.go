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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelinetypes"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

// ParseRawReport parses a raw conversion into RawReport.
func ParseRawReport(line string) (*pipelinetypes.RawReport, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return nil, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key128, err := utils.StringToUint128(cols[0])
	if err != nil {
		return nil, err
	}

	value64, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return nil, err
	}
	return &pipelinetypes.RawReport{Bucket: key128, Value: value64}, nil
}

// EncryptReport encrypts an input report with given public keys.
func EncryptReport(report *pipelinetypes.RawReport, keys *reporttypes.PublicKeys, sharedInfo string, encryptOutput bool) (*pb.AggregatablePayload, error) {
	payload := reporttypes.Payload{
		Operation: "histogram",
		Data: []reporttypes.Contribution{
			{Bucket: utils.Uint128ToBigEndianBytes(report.Bucket), Value: utils.Uint32ToBigEndianBytes(uint32(report.Value))},
		},
	}
	bPayload, err := utils.MarshalCBOR(payload)
	if err != nil {
		return nil, err
	}

	keyID, key, err := cryptoio.GetRandomPublicKey(keys)
	if err != nil {
		return nil, err
	}

	if !encryptOutput {
		return &pb.AggregatablePayload{
			Payload:    &pb.StandardCiphertext{Data: bPayload},
			SharedInfo: sharedInfo,
			KeyId:      keyID,
		}, nil
	}

	encrypted, err := standardencrypt.Encrypt(bPayload, []byte(sharedInfo), key)
	if err != nil {
		return nil, err
	}
	return &pb.AggregatablePayload{Payload: encrypted, SharedInfo: sharedInfo, KeyId: keyID}, nil
}

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.AggregatablePayload)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*encryptReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pipelinetypes.RawReport)(nil)))
	beam.RegisterFunction(formatEncryptedReportFn)
}

// parseRawReportFn parses each line in the raw conversion file in the format: bucket ID, value.
type parseRawReportFn struct {
	countReport beam.Counter
}

func (fn *parseRawReportFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "parserawReportFn_report_count")
}

func (fn *parseRawReportFn) ProcessElement(ctx context.Context, line string, emit func(*pipelinetypes.RawReport)) error {
	conversion, err := ParseRawReport(line)
	if err != nil {
		return err
	}

	fn.countReport.Inc(ctx, 1)
	emit(conversion)
	return nil
}

type encryptReportFn struct {
	PublicKeys    *reporttypes.PublicKeys
	EncryptOutput bool

	countReport beam.Counter
}

func (fn *encryptReportFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "encryptReportFn_report_count")
}

func (fn *encryptReportFn) ProcessElement(ctx context.Context, c *pipelinetypes.RawReport, emit func(*pb.AggregatablePayload)) error {
	fn.countReport.Inc(ctx, 1)

	encrypted, err := EncryptReport(c, fn.PublicKeys, "", fn.EncryptOutput)
	if err != nil {
		return err
	}

	emit(encrypted)
	return nil
}

// Since we store and read the data line by line through plain text files, the output is base64-encoded to avoid writing symbols in the proto wire-format that are interpreted as line breaks.
func formatEncryptedReportFn(encrypted *pb.AggregatablePayload, emit func(string)) error {
	encryptedStr, err := reporttypes.SerializeAggregatablePayload(encrypted)
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
	PublicKeys         *reporttypes.PublicKeys
	Shards             int64

	// EncryptOutput should only be used for integration test before HPKE is ready in Go Tink.
	EncryptOutput bool
}

// GenerateEncryptedReport encrypts the reports with public keys from corresponding helpers.
func GenerateEncryptedReport(scope beam.Scope, params *GenerateEncryptedReportParams) {
	scope = scope.Scope("GenerateEncryptedReport")

	allFiles := pipelineutils.AddStrInPath(params.RawReportURI, "*")
	lines := textio.ReadSdf(scope, allFiles)

	rawReports := beam.ParDo(scope, &parseRawReportFn{}, lines)
	resharded := beam.Reshuffle(scope, rawReports)

	encrypted := beam.ParDo(scope, &encryptReportFn{PublicKeys: params.PublicKeys}, resharded)

	writeEncryptedReport(scope, encrypted, params.EncryptedReportURI, params.Shards)
}

// GenerateBrowserReportParams contains required parameters for function GenerateReport().
type GenerateBrowserReportParams struct {
	RawReport     pipelinetypes.RawReport
	PublicKeys    *reporttypes.PublicKeys
	SharedInfo    string
	EncryptOutput bool
}

// GenerateBrowserReport creates an aggregation report from the browser.
func GenerateBrowserReport(params *GenerateBrowserReportParams) (*reporttypes.AggregatableReport, error) {
	encrypted, err := EncryptReport(&params.RawReport, params.PublicKeys, params.SharedInfo, params.EncryptOutput)
	if err != nil {
		return nil, err
	}
	payload := &reporttypes.AggregationServicePayload{Payload: base64.StdEncoding.EncodeToString(encrypted.Payload.Data), KeyID: encrypted.KeyId}
	return &reporttypes.AggregatableReport{
		SharedInfo:                 params.SharedInfo,
		AggregationServicePayloads: []*reporttypes.AggregationServicePayload{payload},
	}, nil
}
