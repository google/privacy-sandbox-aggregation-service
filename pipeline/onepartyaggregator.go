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

// Package onepartyaggregator contains functions for the one-party design of aggregation service.
package onepartyaggregator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	"google.golang.org/protobuf/proto"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.EncryptedReport)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*decryptReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*filterBucketFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseEncryptedReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseTargetBucketFn)(nil)).Elem())
}

// parseTargetBucketFn parses each line of the input and gets a uint128 bucket ID and a boolean value.
//
// The boolean value will be used for the join operation.
type parseTargetBucketFn struct {
	bucketCounter beam.Counter
}

func (fn *parseTargetBucketFn) Setup() {
	fn.bucketCounter = beam.NewCounter("one-party", "parse-target-bucket-count")
}

func (fn *parseTargetBucketFn) ProcessElement(ctx context.Context, line string, emit func(uint128.Uint128, bool)) error {
	bucket, err := utils.StringToUint128(line)
	if err != nil {
		return err
	}
	emit(bucket, true)
	fn.bucketCounter.Inc(ctx, 1)
	return nil
}

// ReadTargetBucket reads the input file and gets the bucket IDs.
func ReadTargetBucket(scope beam.Scope, bucketURI string) beam.PCollection {
	scope = scope.Scope("ReadTargetBucket")
	allFiles := utils.AddStrInPath(bucketURI, "*")
	lines := textio.Read(scope, allFiles)
	return beam.ParDo(scope, &parseTargetBucketFn{}, lines)
}

// parseEncryptedReportFn parses each line of the input report and gets a StandardCiphertext, which represents a encrypted raw report.
type parseEncryptedReportFn struct {
	reportCounter beam.Counter
}

func (fn *parseEncryptedReportFn) Setup() {
	fn.reportCounter = beam.NewCounter("one-party", "parse-encrypted-report-count")
}

// ParseEncryptedReport parses the input line into the EncryptedReport type.
func ParseEncryptedReport(line string) (*pb.EncryptedReport, error) {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return nil, err
	}

	encrypted := &pb.EncryptedReport{}
	if err := proto.Unmarshal(bsc, encrypted); err != nil {
		return nil, err
	}
	return encrypted, nil
}

func (fn *parseEncryptedReportFn) ProcessElement(ctx context.Context, line string, emit func(*pb.EncryptedReport)) error {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return err
	}

	encrypted := &pb.EncryptedReport{}
	if err := proto.Unmarshal(bsc, encrypted); err != nil {
		return err
	}
	emit(encrypted)
	fn.reportCounter.Inc(ctx, 1)
	return nil
}

// ReadEncryptedReport reads each line from a file, and parses it as a encrypted report.
func ReadEncryptedReport(scope beam.Scope, reportFile string) beam.PCollection {
	scope = scope.Scope("ReadEncryptedReport")
	allFiles := utils.AddStrInPath(reportFile, "*")
	lines := textio.Read(scope, allFiles)
	return beam.ParDo(scope, &parseEncryptedReportFn{}, lines)
}

// decryptReportFn decrypts the StandardCiphertext and gets a raw report with the private key from the helper server.
type decryptReportFn struct {
	StandardPrivateKeys                map[string]*pb.StandardPrivateKey
	reportCounter, nonencryptedCounter beam.Counter
	isEncryptedBundle                  bool
}

func (fn *decryptReportFn) Setup() {
	fn.isEncryptedBundle = true
	fn.reportCounter = beam.NewCounter("one-party", "decrypt-report-count")
	fn.nonencryptedCounter = beam.NewCounter("one-party", "decrypt-nonencrypted-count")
}

// DecryptPayload decrypts the payload with a private key.
func DecryptPayload(encrypted *pb.EncryptedReport, privateKey *pb.StandardPrivateKey) (*reporttypes.Payload, error) {
	b, err := standardencrypt.Decrypt(encrypted.EncryptedReport, encrypted.ContextInfo, privateKey)
	if err != nil {
		return nil, err
	}

	payload := &reporttypes.Payload{}
	if err := utils.UnmarshalCBOR(b, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// ConvertPayload deserializes the payload if it is not encrypted.
func ConvertPayload(encrypted *pb.EncryptedReport) (*reporttypes.Payload, error) {
	payload := &reporttypes.Payload{}
	if err := utils.UnmarshalCBOR(encrypted.EncryptedReport.Data, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (fn *decryptReportFn) ProcessElement(ctx context.Context, encrypted *pb.EncryptedReport, emit func(uint128.Uint128, uint64)) error {
	privateKey, ok := fn.StandardPrivateKeys[encrypted.KeyId]
	if !ok {
		return fmt.Errorf("no private key found for keyID = %q", encrypted.KeyId)
	}

	var (
		payload *reporttypes.Payload
		err     error
	)
	if fn.isEncryptedBundle {
		payload, err = DecryptPayload(encrypted, privateKey)
		if err != nil {
			payload, err = ConvertPayload(encrypted)
			if err != nil {
				return fmt.Errorf("failed in decrypting and converting payload from data: %s", encrypted.String())
			}
		}
	} else {
		payload, err = ConvertPayload(encrypted)
		if err != nil {
			return fmt.Errorf("failed in converting payload from nonencrypted data: %s", encrypted.String())
		}
	}

	report := &reporttypes.RawReport{}
	if err := json.Unmarshal(payload.DPFKey, report); err != nil {
		return err
	}
	emit(report.Bucket, report.Value)
	fn.reportCounter.Inc(ctx, 1)
	return nil
}

// DecryptReport decrypts every line in the input file with the helper private key.
func DecryptReport(s beam.Scope, encryptedReport beam.PCollection, standardPrivateKeys map[string]*pb.StandardPrivateKey) beam.PCollection {
	s = s.Scope("DecryptReport")
	return beam.ParDo(s, &decryptReportFn{
		StandardPrivateKeys: standardPrivateKeys,
	}, encryptedReport)
}

// SumRawReport aggregates the raw report and get the sum for each index.
func SumRawReport(s beam.Scope, col beam.PCollection) beam.PCollection {
	return stats.SumPerKey(s, col)
}

// formatHistogramFn converts the aggregation results into a string.
type formatHistogramFn struct {
	countBucket beam.Counter
}

func (fn *formatHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("one-party", "format-histogram-bucket-count")
}

func (fn *formatHistogramFn) ProcessElement(ctx context.Context, index uint128.Uint128, result uint64, emit func(string)) error {
	emit(fmt.Sprintf("%s,%d", index.String(), result))
	return nil
}

func writeHistogram(s beam.Scope, col beam.PCollection, outputName string) {
	s = s.Scope("WriteHistogram")
	formatted := beam.ParDo(s, &formatHistogramFn{}, col)
	textio.Write(s, outputName, formatted)
}

type filterBucketFn struct {
	filterBucketCounter beam.Counter
}

func (fn *filterBucketFn) Setup() {
	fn.filterBucketCounter = beam.NewCounter("one-party", "filter-bucket-count")
}

func (fn *filterBucketFn) ProcessElement(ctx context.Context, bucket uint128.Uint128, targetIter func(*bool) bool, resultIter func(*uint64) bool, emitResult func(uint128.Uint128, uint64)) error {
	var target bool
	if !targetIter(&target) {
		// If the bucket is not a target, simply return.
		return nil
	}

	var result uint64
	if !resultIter(&result) {
		result = 0
	}

	emitResult(bucket, result)
	fn.filterBucketCounter.Inc(ctx, 1)
	return nil
}

// AggregateReportParams contains necessary parameters for function AggregateReport().
type AggregateReportParams struct {
	// Input report file URI, each line contains an encrypted payload from the browser.
	EncryptedReportURI string
	// Input target bucket URI, each line contains an bucket ID.
	TargetBucketURI string
	// Output aggregation file URI, each line contains a bucket index and the summation of values.
	HistogramURI string
	// The private keys for the standard encryption from the helper server.
	HelperPrivateKeys map[string]*pb.StandardPrivateKey
}

// AggregateReport reads the encrypted reports, decrypts and aggregates them.
func AggregateReport(scope beam.Scope, params *AggregateReportParams) {
	scope = scope.Scope("AggregateReport")

	buckets := ReadTargetBucket(scope, params.TargetBucketURI)

	encrypted := ReadEncryptedReport(scope, params.EncryptedReportURI)
	reshuffled := beam.Reshuffle(scope, encrypted)
	decrypted := DecryptReport(scope, reshuffled, params.HelperPrivateKeys)
	result := SumRawReport(scope, decrypted)

	joined := beam.CoGroupByKey(scope, buckets, result)
	filteredResult := beam.ParDo(scope, &filterBucketFn{}, joined)

	// TODO: Add noise before writing the result.
	writeHistogram(scope, filteredResult, params.HistogramURI)
}
