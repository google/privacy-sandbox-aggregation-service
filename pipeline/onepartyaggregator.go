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
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/distributednoise"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

const numberOfHelpers = 1

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.AggregatablePayload)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*addNoiseFn)(nil)).Elem())
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
	allFiles := pipelineutils.AddStrInPath(bucketURI, "*")
	lines := textio.ReadSdf(scope, allFiles)
	return beam.ParDo(scope, &parseTargetBucketFn{}, lines)
}

// parseEncryptedReportFn parses each line of the input report and gets a StandardCiphertext, which represents a encrypted raw report.
type parseEncryptedReportFn struct {
	reportCounter beam.Counter
}

func (fn *parseEncryptedReportFn) Setup() {
	fn.reportCounter = beam.NewCounter("one-party", "parse-encrypted-report-count")
}

func (fn *parseEncryptedReportFn) ProcessElement(ctx context.Context, line string, emit func(*pb.AggregatablePayload)) error {
	encrypted, err := reporttypes.DeserializeAggregatablePayload(line)
	if err != nil {
		return err
	}
	emit(encrypted)
	fn.reportCounter.Inc(ctx, 1)
	return nil
}

// ReadEncryptedReport reads each line from a file, and parses it as a encrypted report.
func ReadEncryptedReport(scope beam.Scope, reportFile string) beam.PCollection {
	scope = scope.Scope("ReadEncryptedReport")
	allFiles := pipelineutils.AddStrInPath(reportFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
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

func (fn *decryptReportFn) ProcessElement(ctx context.Context, encrypted *pb.AggregatablePayload, emit func(uint128.Uint128, uint64)) error {
	privateKey, ok := fn.StandardPrivateKeys[encrypted.KeyId]
	if !ok {
		return fmt.Errorf("no private key found for keyID = %q", encrypted.KeyId)
	}

	payload := &reporttypes.Payload{}
	if fn.isEncryptedBundle {
		var (
			isEncrypted bool
			err         error
		)
		payload, isEncrypted, err = cryptoio.DecryptOrUnmarshal(encrypted, privateKey)
		if err != nil {
			return err
		}
		if !isEncrypted {
			fn.nonencryptedCounter.Inc(ctx, 1)
			fn.isEncryptedBundle = false
		}
	} else {
		if err := utils.UnmarshalCBOR(encrypted.Payload.Data, payload); err != nil {
			return fmt.Errorf("failed in deserializing non-encrypted data: %s", encrypted.String())
		}
		fn.nonencryptedCounter.Inc(ctx, 1)
	}

	for _, contribution := range payload.Data {
		bucket, err := utils.BigEndianBytesToUint128(contribution.Bucket)
		if err != nil {
			return err
		}
		value, err := utils.BigEndianBytesToUint32(contribution.Value)
		if err != nil {
			return err
		}
		emit(bucket, uint64(value))
	}

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

type addNoiseFn struct {
	Epsilon       float64
	L1Sensitivity uint64
}

func (fn *addNoiseFn) ProcessElement(bucket uint128.Uint128, value uint64, emitResult func(uint128.Uint128, uint64)) error {
	noise, err := distributednoise.DistributedGeometricMechanismRand(fn.Epsilon, fn.L1Sensitivity, numberOfHelpers)
	if err != nil {
		return err
	}
	// Overflow of the noise is expected, and there's 50% probability that the noise is negative.
	value += uint64(noise)
	emitResult(bucket, value)
	return nil
}

func addNoise(scope beam.Scope, rawResult beam.PCollection, epsilon float64, l1Sensitivity uint64) beam.PCollection {
	scope = scope.Scope("AddNoise")
	return beam.ParDo(scope, &addNoiseFn{Epsilon: epsilon, L1Sensitivity: l1Sensitivity}, rawResult)
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
	// Privacy budget for adding noise to the aggregation.
	Epsilon       float64
	L1Sensitivity uint64
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

	if params.Epsilon > 0 {
		filteredResult = addNoise(scope, filteredResult, params.Epsilon, params.L1Sensitivity)
	}

	// TODO: Add noise before writing the result.
	writeHistogram(scope, filteredResult, params.HistogramURI)
}

// ValidateTargetBuckets checks if the targeted bucket IDs are empty.
func ValidateTargetBuckets(ctx context.Context, bucketURI string) error {
	lines, err := utils.ReadLines(ctx, bucketURI)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return errors.New("expect nonempty bucket IDs")
	}
	return nil
}

func parseHistogram(line string) (uint128.Uint128, uint64, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return uint128.Zero, 0, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key128, err := utils.StringToUint128(cols[0])
	if err != nil {
		return uint128.Zero, 0, err
	}

	value64, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return uint128.Zero, 0, err
	}
	return key128, value64, nil
}

// ReadHistogram reads the aggregation result.
func ReadHistogram(ctx context.Context, filename string) (map[uint128.Uint128]uint64, error) {
	lines, err := utils.ReadLines(ctx, filename)
	if err != nil {
		return nil, err
	}
	result := make(map[uint128.Uint128]uint64)
	for _, line := range lines {
		index, aggregation, err := parseHistogram(line)
		if err != nil {
			return nil, err
		}
		result[index] = aggregation
	}
	return result, nil
}
