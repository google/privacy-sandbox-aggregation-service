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

// Package dpfprocess processes data for the DPF protocol.
package dpfprocess

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/tools/dpfconvert"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.EncryptedPartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*encryptSecretSharesFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawConversionFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*reporttypes.RawReport)(nil)))
	beam.RegisterFunction(formatPartialReportFn)
}

// parseRawConversionFn parses each line in the raw conversion file in the format: bucket ID, value.
type parseRawConversionFn struct {
	KeyBitSize      int
	countConversion beam.Counter
}

func (fn *parseRawConversionFn) Setup(ctx context.Context) {
	fn.countConversion = beam.NewCounter("aggregation", "parserawConversionFn_conversion_count")
}

func getMaxKey(s int) uint128.Uint128 {
	maxKey := uint128.Max
	if s < 128 {
		maxKey = uint128.Uint128{1, 0}.Lsh(uint(s)).Sub64(1)
	}
	return maxKey
}

//ParseRawConversion parses a raw conversion.
func ParseRawConversion(line string, keyBitSize int) (reporttypes.RawReport, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return reporttypes.RawReport{}, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key128, err := utils.StringToUint128(cols[0])
	if err != nil {
		return reporttypes.RawReport{}, err
	}
	if key128.Cmp(getMaxKey(keyBitSize)) == 1 {
		return reporttypes.RawReport{}, fmt.Errorf("key %q overflows the integer with %d bits", key128.String(), keyBitSize)
	}

	value64, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return reporttypes.RawReport{}, err
	}
	return reporttypes.RawReport{Bucket: key128, Value: value64}, nil
}

func (fn *parseRawConversionFn) ProcessElement(ctx context.Context, line string, emit func(reporttypes.RawReport)) error {
	conversion, err := ParseRawConversion(line, fn.KeyBitSize)
	if err != nil {
		return err
	}

	fn.countConversion.Inc(ctx, 1)
	emit(conversion)
	return nil
}

type encryptSecretSharesFn struct {
	PublicKeys1, PublicKeys2 []cryptoio.PublicKeyInfo
	KeyBitSize               int
	EncryptOutput            bool

	countReport beam.Counter
}

// TODO: Check if the chosen public key is out of date.
func getRandomPublicKey(keys []cryptoio.PublicKeyInfo) (string, *pb.StandardPublicKey, error) {
	keyInfo := keys[rand.Intn(len(keys))]
	bKey, err := base64.StdEncoding.DecodeString(keyInfo.Key)
	if err != nil {
		return "", nil, err
	}
	return keyInfo.ID, &pb.StandardPublicKey{Key: bKey}, nil
}

func encryptPartialReport(partialReport *pb.PartialReportDpf, keys []cryptoio.PublicKeyInfo, contextInfo []byte, encryptOutput bool) (*pb.EncryptedPartialReportDpf, error) {
	bDpfKey, err := proto.Marshal(partialReport.SumKey)
	if err != nil {
		return nil, err
	}

	payload := reporttypes.Payload{
		Operation: "hierarchical-histogram",
		DPFKey:    bDpfKey,
	}
	bPayload, err := utils.MarshalCBOR(payload)
	if err != nil {
		return nil, err
	}

	keyID, key, err := getRandomPublicKey(keys)
	if err != nil {
		return nil, err
	}
	// TODO: Remove the option of aggregating reports without encryption when HPKE is ready in Go tink.
	if !encryptOutput {
		return &pb.EncryptedPartialReportDpf{
			EncryptedReport: &pb.StandardCiphertext{Data: bPayload},
			ContextInfo:     contextInfo,
			KeyId:           keyID,
		}, nil
	}

	encrypted, err := standardencrypt.Encrypt(bPayload, contextInfo, key)
	if err != nil {
		return nil, err
	}
	return &pb.EncryptedPartialReportDpf{EncryptedReport: encrypted, ContextInfo: contextInfo, KeyId: keyID}, nil
}

func (fn *encryptSecretSharesFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "encryptSecretSharesFn_report_count")
}

// putValueForHierarchies determines which value to use when the keys are expanded for different hierarchies.
//
// For now, the values are the same for all the levels.
func putValueForHierarchies(params []*dpfpb.DpfParameters, value uint64) []uint64 {
	values := make([]uint64, len(params))
	for i := range values {
		values[i] = value
	}
	return values
}

func (fn *encryptSecretSharesFn) ProcessElement(ctx context.Context, c reporttypes.RawReport, emit1 func(*pb.EncryptedPartialReportDpf), emit2 func(*pb.EncryptedPartialReportDpf)) error {
	fn.countReport.Inc(ctx, 1)

	encryptedReport1, encryptedReport2, err := dpfconvert.GenerateEncryptedReports(c, fn.KeyBitSize, fn.PublicKeys1, fn.PublicKeys2, nil, fn.EncryptOutput)
	if err != nil {
		return err
	}

	emit1(encryptedReport1)
	emit2(encryptedReport2)
	return nil
}

// Since we store and read the data line by line through plain text files, the output is base64-encoded to avoid writing symbols in the proto wire-format that are interpreted as line breaks.
func formatPartialReportFn(encrypted *pb.EncryptedPartialReportDpf, emit func(string)) error {
	encryptedStr, err := dpfconvert.FormatEncryptedPartialReport(encrypted)
	if err != nil {
		return err
	}
	emit(encryptedStr)
	return nil
}

func writePartialReport(s beam.Scope, output beam.PCollection, outputTextName string, shards int64) {
	s = s.Scope("WriteEncryptedPartialReportDpf")
	formatted := beam.ParDo(s, formatPartialReportFn, output)
	pipelineutils.WriteNShardedFiles(s, outputTextName, shards, formatted)
}

// splitRawConversion splits the raw report records into secret shares and encrypts them with public keys from helpers.
func splitRawConversion(s beam.Scope, reports beam.PCollection, params *GeneratePartialReportParams) (beam.PCollection, beam.PCollection) {
	s = s.Scope("SplitRawConversion")

	return beam.ParDo2(s,
		&encryptSecretSharesFn{
			PublicKeys1:   params.PublicKeys1,
			PublicKeys2:   params.PublicKeys2,
			KeyBitSize:    params.KeyBitSize,
			EncryptOutput: params.EncryptOutput,
		}, reports)
}

// GeneratePartialReportParams contains required parameters for generating partial reports.
type GeneratePartialReportParams struct {
	ConversionURI, PartialReportURI1, PartialReportURI2 string
	PublicKeys1, PublicKeys2                            []cryptoio.PublicKeyInfo
	KeyBitSize                                          int
	Shards                                              int64

	// EncryptOutput should only be used for integration test before HPKE is ready in Go Tink.
	EncryptOutput bool
}

// GeneratePartialReport splits the raw reports into two shares and encrypts them with public keys from corresponding helpers.
func GeneratePartialReport(scope beam.Scope, params *GeneratePartialReportParams) {
	scope = scope.Scope("GeneratePartialReports")

	allFiles := utils.AddStrInPath(params.ConversionURI, "*")
	lines := textio.Read(scope, allFiles)
	rawConversions := beam.ParDo(scope, &parseRawConversionFn{KeyBitSize: params.KeyBitSize}, lines)
	resharded := beam.Reshuffle(scope, rawConversions)

	partialReport1, partialReport2 := splitRawConversion(scope, resharded, params)
	writePartialReport(scope, partialReport1, params.PartialReportURI1, params.Shards)
	writePartialReport(scope, partialReport2, params.PartialReportURI2, params.Shards)
}
