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

// Package dpfaggregator contains functions that aggregates the reports with the DPF protocol.
//
// Each encrypted PartialReportDpf contains a DPF key for SUM aggregation, which is
// stored as a single line in the input file. Function AggregatePartialReport() parses the lines
// and decrypts the DPF keys. Then function ExpandAndCombineHistogram() expands the two DPF keys
// into vectors as the contribution of one conversion record to the final histogram.
//
// The vectors need to be combined by summing the values for each bucket together to get the
// aggregation result. When the input size or histogram domain size is small, the combination
// can be done with directCombine(). Otherwise due to the Apache Beam issue
// (https://issues.apache.org/jira/browse/BEAM-11916), we need to split the vectors and then
// combine the peices (segmentCombine()) as a workaround.
//
// Finally, the SUM result is stored in a PartialAggregationDpf for each bucket ID. The
// buket ID and it's corresponding PartialAggregationDpf in wire-format is written as a line in the
// output file.
//
// Function MergePartialHistogram() reads the partial aggregation results from different helpers,
// and gets the complete histograms by adding the SUM results.
package dpfaggregator

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"cloud.google.com/go/storage"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.PartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialAggregationDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*alignVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*alignVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*expandDpfKeyFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*decryptPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatCompleteHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*mergeHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseEncryptedPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parsePartialHistogramFn)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*expandedVec)(nil)))
}

// parseEncryptedPartialReportFn parses each line of the input partial report and gets a StandardCiphertext, which represents a encrypted PartialReportDpf.
type parseEncryptedPartialReportFn struct {
	partialReportCounter beam.Counter
}

func (fn *parseEncryptedPartialReportFn) Setup() {
	fn.partialReportCounter = beam.NewCounter("aggregation-prototype", "partial-report-count")
}

func (fn *parseEncryptedPartialReportFn) ProcessElement(ctx context.Context, line string, emit func(*pb.StandardCiphertext)) error {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return err
	}

	ciphertext := &pb.StandardCiphertext{}
	if err := proto.Unmarshal(bsc, ciphertext); err != nil {
		return err
	}
	emit(ciphertext)
	return nil
}

func readPartialReport(scope beam.Scope, partialReportFile string) beam.PCollection {
	allFiles := ioutils.AddStrInPath(partialReportFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	return beam.ParDo(scope, &parseEncryptedPartialReportFn{}, lines)
}

// decryptPartialReportFn decrypts the StandardCiphertext and gets a PartialReportDpf with the private key from the helper server.
type decryptPartialReportFn struct {
	StandardPrivateKey *pb.StandardPrivateKey
}

func (fn *decryptPartialReportFn) ProcessElement(encrypted *pb.StandardCiphertext, emit func(*pb.PartialReportDpf)) error {
	b, err := standardencrypt.Decrypt(encrypted, nil, fn.StandardPrivateKey)
	if err != nil {
		return fmt.Errorf("decrypt failed for cipherText: %s", encrypted.String())
	}

	partialReport := &pb.PartialReportDpf{}
	if err := proto.Unmarshal(b, partialReport); err != nil {
		return err
	}
	emit(partialReport)
	return nil
}

// DecryptPartialReport decrypts every line in the input file with the helper private key, and gets the partial report.
func DecryptPartialReport(s beam.Scope, encryptedReport beam.PCollection, standardPrivateKey *pb.StandardPrivateKey) beam.PCollection {
	s = s.Scope("DecryptPartialReport")
	return beam.ParDo(s, &decryptPartialReportFn{StandardPrivateKey: standardPrivateKey}, encryptedReport)
}

type expandedVec struct {
	SumVec []uint64
}

// expandDpfKeyFn expands the DPF keys in a PartialReportDpf into two vectors that represent the contribution to the SUM histogram.
type expandDpfKeyFn struct {
	SumParameters *pb.IncrementalDpfParameters
	Prefixes      *pb.HierarchicalPrefixes
	vecCounter    beam.Counter
}

func (fn *expandDpfKeyFn) Setup() {
	fn.vecCounter = beam.NewCounter("aggregation", "expandDpfFn-vec-count")
}

func (fn *expandDpfKeyFn) ProcessElement(ctx context.Context, partialReport *pb.PartialReportDpf, emit func(*expandedVec)) error {
	fn.vecCounter.Inc(ctx, 1)

	sumCtx, err := incrementaldpf.CreateEvaluationContext(fn.SumParameters.Params, partialReport.GetSumKey())
	if err != nil {
		return err
	}

	var vecSum []uint64
	for i := range fn.SumParameters.Params {
		vecSum, err = incrementaldpf.EvaluateNext64(fn.Prefixes.Prefixes[i].Prefix, sumCtx)
		if err != nil {
			return err
		}
	}

	emit(&expandedVec{SumVec: vecSum})
	return nil
}

// combineVectorFn combines the expandedVecs by adding the values for each index together for each
// vector. The combination result is a single expandedVec.
type combineVectorFn struct {
	VectorLength uint64

	inputCounter  beam.Counter
	createCounter beam.Counter
	mergeCounter  beam.Counter
}

func (fn *combineVectorFn) Setup() {
	fn.inputCounter = beam.NewCounter("aggregation", "combineVectorFn-input-count")
	fn.createCounter = beam.NewCounter("aggregation", "combineVectorFn-create-count")
	fn.mergeCounter = beam.NewCounter("aggregation", "combineVectorFn-merge-count")
}

func (fn *combineVectorFn) CreateAccumulator(ctx context.Context) *expandedVec {
	fn.createCounter.Inc(ctx, 1)
	return &expandedVec{SumVec: make([]uint64, fn.VectorLength)}
}

func (fn *combineVectorFn) AddInput(ctx context.Context, e *expandedVec, p *expandedVec) *expandedVec {
	fn.inputCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.VectorLength; i++ {
		e.SumVec[i] += p.SumVec[i]
	}
	return e
}

func (fn *combineVectorFn) MergeAccumulators(ctx context.Context, a, b *expandedVec) *expandedVec {
	fn.mergeCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.VectorLength; i++ {
		a.SumVec[i] += b.SumVec[i]
	}
	return a
}

// alignVectorFn turns the single expandedVec into a collection of <index, PartialAggregationDpf> pairs, which is easier for writing operation.
type alignVectorFn struct {
	BucketIDs []uint64

	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("combinetest", "flattenVecFn_input_count")
	fn.outputCounter = beam.NewCounter("combinetest", "flattenVecFn_output_count")
}

func (fn *alignVectorFn) ProcessElement(ctx context.Context, vec *expandedVec, emit func(uint64, *pb.PartialAggregationDpf)) {
	fn.inputCounter.Inc(ctx, 1)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		if fn.BucketIDs != nil {
			emit(fn.BucketIDs[i], &pb.PartialAggregationDpf{PartialSum: sum})
		} else {
			emit(uint64(i), &pb.PartialAggregationDpf{PartialSum: sum})
		}
	}
}

// combineVectorSegmentFn gets a segment of vectors from the input expandedVec with specific start index and length. And then does the same combination with combineVectorFn.
type combineVectorSegmentFn struct {
	StartIndex uint64
	Length     uint64

	inputCounter  beam.Counter
	createCounter beam.Counter
	mergeCounter  beam.Counter
}

func (fn *combineVectorSegmentFn) Setup() {
	fn.inputCounter = beam.NewCounter("aggregation", "combineVectorSegmentFn-input-count")
	fn.createCounter = beam.NewCounter("aggregation", "combineVectorSegmentFn-create-count")
	fn.mergeCounter = beam.NewCounter("aggregation", "combineVectorSegmentFn-merge-count")
}

func (fn *combineVectorSegmentFn) CreateAccumulator(ctx context.Context) *expandedVec {
	fn.createCounter.Inc(ctx, 1)

	return &expandedVec{SumVec: make([]uint64, fn.Length)}
}

func (fn *combineVectorSegmentFn) AddInput(ctx context.Context, e *expandedVec, p *expandedVec) *expandedVec {
	fn.inputCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.Length; i++ {
		e.SumVec[i] += p.SumVec[i+fn.StartIndex]
	}
	return e
}

func (fn *combineVectorSegmentFn) MergeAccumulators(ctx context.Context, a, b *expandedVec) *expandedVec {
	fn.mergeCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.Length; i++ {
		a.SumVec[i] += b.SumVec[i]
	}
	return a
}

// alignVectorSegmentFn does the same thing with alignVectorFn, except for starting the bucket index with a given value.
type alignVectorSegmentFn struct {
	StartIndex uint64
	BucketIDs  []uint64

	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorSegmentFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_input_count")
	fn.outputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_output_count")
}

func (fn *alignVectorSegmentFn) ProcessElement(ctx context.Context, vec *expandedVec, emit func(uint64, *pb.PartialAggregationDpf)) {
	fn.inputCounter.Inc(ctx, 1)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		if fn.BucketIDs != nil {
			emit(fn.BucketIDs[uint64(i)+fn.StartIndex], &pb.PartialAggregationDpf{PartialSum: sum})
		} else {
			emit(uint64(i)+fn.StartIndex, &pb.PartialAggregationDpf{PartialSum: sum})
		}
	}
}

// directCombine aggregates the expanded vectors to a single vector, and then converts it to be a PCollection.
func directCombine(scope beam.Scope, expanded beam.PCollection, vectorLength uint64, bucketIDs []uint64) beam.PCollection {
	scope = scope.Scope("DirectCombine")
	histogram := beam.Combine(scope, &combineVectorFn{VectorLength: vectorLength}, expanded)
	return beam.ParDo(scope, &alignVectorFn{BucketIDs: bucketIDs}, histogram)
}

// There is an issue when combining large vectors (large domain size):  https://issues.apache.org/jira/browse/BEAM-11916
// As a workaround, we split the vectors into pieces and combine the collection of the smaller vectors instead.
func segmentCombine(scope beam.Scope, expanded beam.PCollection, vectorLength, segmentLength uint64, bucketIDs []uint64) beam.PCollection {
	scope = scope.Scope("SegmentCombine")
	results := make([]beam.PCollection, vectorLength/segmentLength)
	for i := range results {
		pHistogram := beam.Combine(scope, &combineVectorSegmentFn{StartIndex: uint64(i) * segmentLength, Length: segmentLength}, expanded)
		results[i] = beam.ParDo(scope, &alignVectorSegmentFn{StartIndex: uint64(i) * segmentLength, BucketIDs: bucketIDs}, pHistogram)
	}
	return beam.Flatten(scope, results...)
}

// AggregatePartialReportParams contains necessary parameters for function AggregatePartialReport().
type AggregatePartialReportParams struct {
	// Input partial report file path, each line contains an encrypted PartialReportDpf.
	PartialReportFile string
	// Output partial aggregation file path, each line contains a bucket index and a wire-formatted PartialAggregationDpf.
	PartialHistogramFile string
	// Parameters for the DPF key expansion, which need to be consistent with the ones that are used to generate the partial report on the browser.
	SumParameters *pb.IncrementalDpfParameters
	// Prefixes for the DPF key expansion in each of the hierarchical domains.
	Prefixes *pb.HierarchicalPrefixes
	// The private key for the standard encryption from the helper server.
	HelperPrivateKey *pb.StandardPrivateKey
	// Weather to use directCombine() or segmentCombine() when combining the expanded vectors.
	DirectCombine bool
	// The segment length when using segmentCombine().
	SegmentLength uint64
	// Number of shards when writing the output file.
	Shards int64
}

// ExpandAndCombineHistogram calculates histograms from the DPF keys and combines them.
func ExpandAndCombineHistogram(scope beam.Scope, partialReport beam.PCollection, params *AggregatePartialReportParams) (beam.PCollection, error) {
	expanded := beam.ParDo(scope, &expandDpfKeyFn{
		SumParameters: params.SumParameters,
		Prefixes:      params.Prefixes,
	}, partialReport)

	paramsLen := len(params.SumParameters.Params)
	bucketIDs, err := incrementaldpf.CalculateBucketID(params.SumParameters, params.Prefixes, paramsLen-2, paramsLen-1)
	if err != nil {
		return beam.PCollection{}, err
	}
	vectorLength := uint64(len(bucketIDs))
	if bucketIDs == nil {
		paramsLen := len(params.SumParameters.Params)
		vectorLength = uint64(1) << params.SumParameters.Params[paramsLen-1].LogDomainSize
	}

	if params.DirectCombine {
		return directCombine(scope, expanded, vectorLength, bucketIDs), nil
	}
	return segmentCombine(scope, expanded, vectorLength, params.SegmentLength, bucketIDs), nil
}

// AggregatePartialReport reads the partial report and calculates partial aggregation results from it.
func AggregatePartialReport(scope beam.Scope, params *AggregatePartialReportParams) error {
	if err := incrementaldpf.CheckExpansionParameters(params.SumParameters, params.Prefixes); err != nil {
		return err
	}

	scope = scope.Scope("AggregatePartialreportDpf")

	encrypted := readPartialReport(scope, params.PartialReportFile)
	resharded := beam.Reshuffle(scope, encrypted)

	partialReport := DecryptPartialReport(scope, resharded, params.HelperPrivateKey)
	partialHistogram, err := ExpandAndCombineHistogram(scope, partialReport, params)
	if err != nil {
		return err
	}

	writeHistogram(scope, partialHistogram, params.PartialHistogramFile, params.Shards)
	return nil
}

// formatHistogramFn converts the partial aggregation results into a string with bucket ID and wire-formatted PartialAggregationDpf.
type formatHistogramFn struct {
	countBucket beam.Counter
}

func (fn *formatHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "formatHistogramFn_bucket_count")
}

func (fn *formatHistogramFn) ProcessElement(ctx context.Context, index uint64, result *pb.PartialAggregationDpf, emit func(string)) error {
	b, err := proto.Marshal(result)
	if err != nil {
		return err
	}
	fn.countBucket.Inc(ctx, 1)
	emit(fmt.Sprintf("%d,%s", index, base64.StdEncoding.EncodeToString(b)))
	return nil
}

func writeHistogram(s beam.Scope, col beam.PCollection, outputName string, shards int64) {
	s = s.Scope("WriteHistogram")
	formatted := beam.ParDo(s, &formatHistogramFn{}, col)
	ioutils.WriteNShardedFiles(s, outputName, shards, formatted)
}

// parsePartialHistogramFn parses each line from the partial aggregation file, and gets a pair of bucket ID and PartialAggregationDpf.
type parsePartialHistogramFn struct {
	countBucket beam.Counter
}

func (fn *parsePartialHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "parsePartialHistogramFn_bucket_count")
}

func parseHistogram(line string) (uint64, *pb.PartialAggregationDpf, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return 0, nil, fmt.Errorf("got %d number of columns in line %q, expected %d", got, line, want)
	}

	index, err := strconv.ParseUint(cols[0], 10, 64)
	if err != nil {
		return 0, nil, err
	}

	bResult, err := base64.StdEncoding.DecodeString(cols[1])
	if err != nil {
		return 0, nil, err
	}

	aggregation := &pb.PartialAggregationDpf{}
	if err := proto.Unmarshal(bResult, aggregation); err != nil {
		return 0, nil, err
	}
	return index, aggregation, nil
}

func (fn *parsePartialHistogramFn) ProcessElement(ctx context.Context, line string, emit func(uint64, *pb.PartialAggregationDpf)) error {
	index, aggregation, err := parseHistogram(line)
	if err != nil {
		return err
	}

	fn.countBucket.Inc(ctx, 1)
	emit(index, aggregation)
	return nil
}

func readPartialHistogram(s beam.Scope, partialHistogramFile string) beam.PCollection {
	s = s.Scope("ReadPartialHistogram")
	allFiles := ioutils.AddStrInPath(partialHistogramFile, "*")
	lines := textio.ReadSdf(s, allFiles)
	return beam.ParDo(s, &parsePartialHistogramFn{}, lines)
}

// CompleteHistogram represents the final aggregation result in a histogram.
type CompleteHistogram struct {
	Index uint64
	Sum   uint64
}

// mergeHistogramFn merges the two PartialAggregationDpf messages for the same bucket ID by summing the results.
type mergeHistogramFn struct {
	countBucket beam.Counter
}

func (fn *mergeHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "mergeHistogramFn_bucket_count")
}

func (fn *mergeHistogramFn) ProcessElement(ctx context.Context, index uint64, pHisIter1 func(**pb.PartialAggregationDpf) bool, pHisIter2 func(**pb.PartialAggregationDpf) bool, emit func(CompleteHistogram)) error {
	var hist1 *pb.PartialAggregationDpf
	if !pHisIter1(&hist1) {
		return fmt.Errorf("expect two shares for bucket ID %d, missing from helper1", index)
	}
	var hist2 *pb.PartialAggregationDpf
	if !pHisIter2(&hist2) {
		return fmt.Errorf("expect two shares for bucket ID %d, missing from helper2", index)
	}

	if hist1.PartialSum+hist2.PartialSum == 0 {
		return nil
	}

	fn.countBucket.Inc(ctx, 1)
	emit(CompleteHistogram{
		Index: index,
		Sum:   hist1.PartialSum + hist2.PartialSum,
	})
	return nil
}

// MergeHistogram merges the partial aggregation results in histograms.
func MergeHistogram(s beam.Scope, pHist1, pHist2 beam.PCollection) beam.PCollection {
	s = s.Scope("MergePartialHistograms")
	joined := beam.CoGroupByKey(s, pHist1, pHist2)
	return beam.ParDo(s, &mergeHistogramFn{}, joined)
}

// formatCompleteHistogramFn converts the complete aggregation result into a string of format: bucket ID, SUM, COUNT.
type formatCompleteHistogramFn struct {
	countBucket beam.Counter
}

func (fn *formatCompleteHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "formatCompleteHistogramFn_bucket_count")
}

func (fn *formatCompleteHistogramFn) ProcessElement(ctx context.Context, result CompleteHistogram) string {
	return fmt.Sprintf("%d,%d", result.Index, result.Sum)
}

func writeCompleteHistogram(s beam.Scope, indexResult beam.PCollection, fileName string) {
	s = s.Scope("WriteCompleteHistogram")
	formatted := beam.ParDo(s, &formatCompleteHistogramFn{}, indexResult)
	textio.Write(s, fileName, formatted)
}

// MergePartialHistogram reads the partial aggregated histograms and merges them to get the complete histogram.
func MergePartialHistogram(scope beam.Scope, partialHistFile1, partialHistFile2, completeHistFile string) {
	scope = scope.Scope("MergePartialHistogram")

	partialHist1 := readPartialHistogram(scope, partialHistFile1)
	partialHist2 := readPartialHistogram(scope, partialHistFile2)
	completeHistogram := MergeHistogram(scope, partialHist1, partialHist2)
	writeCompleteHistogram(scope, completeHistogram, completeHistFile)
}

// ReadPartialHistogram reads the partial aggregation result without using a Beam pipeline.
func ReadPartialHistogram(ctx context.Context, filename string) (map[uint64]*pb.PartialAggregationDpf, error) {
	var scanner *bufio.Scanner
	if strings.HasPrefix(filename, "gs://") {
		client, err := storage.NewClient(ctx)
		if err != nil {
			return nil, err
		}

		bucket, object, err := ioutils.ParseGCSPath(filename)
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

	result := make(map[uint64]*pb.PartialAggregationDpf)
	for scanner.Scan() {
		index, aggregation, err := parseHistogram(scanner.Text())
		if err != nil {
			return nil, err
		}
		result[index] = aggregation
	}
	return result, scanner.Err()
}

// WriteCompleteHistogram writes the final aggregation result without using a Beam pipeline.
func WriteCompleteHistogram(ctx context.Context, filename string, results map[uint64]CompleteHistogram) error {
	var buf *bufio.Writer
	if strings.HasPrefix(filename, "gs://") {
		client, err := storage.NewClient(ctx)
		if err != nil {
			return err
		}

		bucket, object, err := ioutils.ParseGCSPath(filename)
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

	for _, result := range results {
		line := fmt.Sprintf("%d,%d\n", result.Index, result.Sum)
		if _, err := buf.WriteString(line); err != nil {
			return err
		}
	}
	return buf.Flush()
}
