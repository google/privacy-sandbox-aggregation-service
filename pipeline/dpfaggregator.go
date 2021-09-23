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
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/distributednoise"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

const numberOfHelpers = 2

func init() {
	beam.RegisterType(reflect.TypeOf((*dpfpb.EvaluationContext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.EncryptedPartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialAggregationDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*alignVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*alignVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*createEvalCtxFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*expandDpfKeyFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*decryptPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatCompleteHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatEvaluationContextFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*mergeHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseEncryptedPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseEvaluationContextFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parsePartialHistogramFn)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*expandedVec)(nil)))
}

// Payload defines the payload sent to one server
type Payload struct {
	Operation string `json:"operation"`
	// DPFKey is a serialized proto of:
	// https://github.com/google/distributed_point_functions/blob/199696c7cde95d9f9e07a4dddbcaaa36d120ca12/dpf/distributed_point_function.proto#L110
	DPFKey  []byte `json:"dpf_key"`
	Padding string `json:"padding"`
}

// parseEncryptedPartialReportFn parses each line of the input partial report and gets a StandardCiphertext, which represents a encrypted PartialReportDpf.
type parseEncryptedPartialReportFn struct {
	partialReportCounter beam.Counter
}

func (fn *parseEncryptedPartialReportFn) Setup() {
	fn.partialReportCounter = beam.NewCounter("aggregation-prototype", "partial-report-count")
}

func (fn *parseEncryptedPartialReportFn) ProcessElement(ctx context.Context, line string, emit func(*pb.EncryptedPartialReportDpf)) error {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return err
	}

	encrypted := &pb.EncryptedPartialReportDpf{}
	if err := proto.Unmarshal(bsc, encrypted); err != nil {
		return err
	}
	emit(encrypted)
	return nil
}

// ReadPartialReport reads each line from a file, and parses it as a partial report that contains a encrypted DPF key and the context info.
func ReadPartialReport(scope beam.Scope, partialReportFile string) beam.PCollection {
	scope = scope.Scope("ReadEncryptedPartialReport")
	allFiles := ioutils.AddStrInPath(partialReportFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	return beam.ParDo(scope, &parseEncryptedPartialReportFn{}, lines)
}

// decryptPartialReportFn decrypts the StandardCiphertext and gets a PartialReportDpf with the private key from the helper server.
type decryptPartialReportFn struct {
	StandardPrivateKeys map[string]*pb.StandardPrivateKey
}

func (fn *decryptPartialReportFn) ProcessElement(encrypted *pb.EncryptedPartialReportDpf, emit func(*pb.PartialReportDpf)) error {
	privateKey, ok := fn.StandardPrivateKeys[encrypted.KeyId]
	if !ok {
		return fmt.Errorf("no private key found for keyID = %q", encrypted.KeyId)
	}

	b, err := standardencrypt.Decrypt(encrypted.EncryptedReport, encrypted.ContextInfo, privateKey)
	if err != nil {
		return fmt.Errorf("decrypt failed for cipherText: %s", encrypted.String())
	}

	payload := &Payload{}
	if err := ioutils.UnmarshalCBOR(b, payload); err != nil {
		return err
	}

	dpfKey := &dpfpb.DpfKey{}
	if err := proto.Unmarshal(payload.DPFKey, dpfKey); err != nil {
		return err
	}
	emit(&pb.PartialReportDpf{SumKey: dpfKey})
	return nil
}

// DecryptPartialReport decrypts every line in the input file with the helper private key, and gets the partial report.
func DecryptPartialReport(s beam.Scope, encryptedReport beam.PCollection, standardPrivateKeys map[string]*pb.StandardPrivateKey) beam.PCollection {
	s = s.Scope("DecryptPartialReport")
	return beam.ParDo(s, &decryptPartialReportFn{StandardPrivateKeys: standardPrivateKeys}, encryptedReport)
}

// GetDefaultDPFParameters generates the DPF parameters for creating DPF keys or evaluation context for all possible prefix lengths.
func GetDefaultDPFParameters(keyBitSize int32) ([]*dpfpb.DpfParameters, error) {
	if keyBitSize <= 0 {
		return nil, fmt.Errorf("keyBitSize should be positive, got %d", keyBitSize)
	}
	allParams := make([]*dpfpb.DpfParameters, keyBitSize)
	for i := int32(1); i <= keyBitSize; i++ {
		allParams[i-1] = &dpfpb.DpfParameters{
			LogDomainSize:  i,
			ElementBitsize: incrementaldpf.DefaultElementBitSize,
		}
	}
	return allParams, nil
}

type createEvalCtxFn struct {
	DPFParams  *pb.IncrementalDpfParameters
	ctxCounter beam.Counter
}

func (fn *createEvalCtxFn) Setup() {
	fn.ctxCounter = beam.NewCounter("aggregation", "createEvalCtxFn-ctx-count")
}

func (fn *createEvalCtxFn) ProcessElement(ctx context.Context, partialReport *pb.PartialReportDpf, emit func(*dpfpb.EvaluationContext)) error {
	sumCtx, err := incrementaldpf.CreateEvaluationContext(fn.DPFParams.Params, partialReport.GetSumKey())
	if err != nil {
		return err
	}
	emit(sumCtx)
	fn.ctxCounter.Inc(ctx, 1)

	return nil
}

// CreateEvaluationContext creates the DPF evaluation context from the decrypted keys.
func CreateEvaluationContext(s beam.Scope, decryptedReport beam.PCollection, dpfParams *pb.IncrementalDpfParameters) beam.PCollection {
	s = s.Scope("CreateEvaluationContext")
	return beam.ParDo(s, &createEvalCtxFn{
		DPFParams: dpfParams,
	}, decryptedReport)
}

// parseEvaluationContextFn parses each line of the input file and gets a DPF evaluation context.
type parseEvaluationContextFn struct {
	evalCtxCounter beam.Counter
}

func (fn *parseEvaluationContextFn) Setup() {
	fn.evalCtxCounter = beam.NewCounter("aggregation", "read-evaluation-context-count")
}

func (fn *parseEvaluationContextFn) ProcessElement(ctx context.Context, line string, emit func(*dpfpb.EvaluationContext)) error {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return err
	}

	evalCtx := &dpfpb.EvaluationContext{}
	if err := proto.Unmarshal(bsc, evalCtx); err != nil {
		return err
	}
	emit(evalCtx)

	fn.evalCtxCounter.Inc(ctx, 1)
	return nil
}

// ReadEvaluationContext reads each line from a file, and parses it as a DPF evaluation context.
func ReadEvaluationContext(scope beam.Scope, evalContextFile string) beam.PCollection {
	scope = scope.Scope("ReadEvaluationContext")
	allFiles := ioutils.AddStrInPath(evalContextFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	return beam.ParDo(scope, &parseEvaluationContextFn{}, lines)
}

type formatEvaluationContextFn struct {
	evalCtxCounter beam.Counter
}

func (fn *formatEvaluationContextFn) Setup() {
	fn.evalCtxCounter = beam.NewCounter("aggregation", "write-evaluation-context-count")
}

func (fn *formatEvaluationContextFn) ProcessElement(ctx context.Context, evalCtx *dpfpb.EvaluationContext, emit func(string)) error {
	b, err := proto.Marshal(evalCtx)
	if err != nil {
		return err
	}
	emit(base64.StdEncoding.EncodeToString(b))

	fn.evalCtxCounter.Inc(ctx, 1)
	return nil
}

// writeEvaluationContext writes the DPF evaluation contexts into a file.
func writeEvaluationContext(s beam.Scope, col beam.PCollection, outputName string, shards int64) {
	s = s.Scope("WriteEvaluationContext")
	formatted := beam.ParDo(s, &formatEvaluationContextFn{}, col)
	ioutils.WriteNShardedFiles(s, outputName, shards, formatted)
}

type expandedVec struct {
	SumVec []uint64
}

// expandDpfKeyFn expands the DPF keys in a PartialReportDpf into a vector that represent the contribution to the SUM histogram.
type expandDpfKeyFn struct {
	Levels     []int32
	Prefixes   *pb.HierarchicalPrefixes
	vecCounter beam.Counter
}

func (fn *expandDpfKeyFn) Setup() {
	fn.vecCounter = beam.NewCounter("aggregation", "expandDpfFn-vec-count")
}

func (fn *expandDpfKeyFn) ProcessElement(ctx context.Context, evalCtx *dpfpb.EvaluationContext, emitVec func(*expandedVec), emitCtx func(*dpfpb.EvaluationContext)) error {
	if int32(fn.Levels[0]) <= evalCtx.PreviousHierarchyLevel {
		return fmt.Errorf("expect current level higher than the previous level %d, got %d", evalCtx.PreviousHierarchyLevel, fn.Levels[0])
	}

	var (
		vecSum []uint64
		err    error
	)
	for i, level := range fn.Levels {
		vecSum, err = incrementaldpf.EvaluateUntil64(int(level), fn.Prefixes.Prefixes[i].Prefix, evalCtx)
		if err != nil {
			return err
		}
	}

	emitVec(&expandedVec{SumVec: vecSum})
	// The evaluation context needs to be saved as the expansion state except for the last-level hierarchy.
	if fn.Levels[len(fn.Levels)-1] < int32(len(evalCtx.Parameters)-1) {
		emitCtx(evalCtx)
	}

	fn.vecCounter.Inc(ctx, 1)
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

type addNoiseFn struct {
	Epsilon       float64
	L1Sensitivity uint64
}

func (fn *addNoiseFn) ProcessElement(ctx context.Context, id uint64, pa *pb.PartialAggregationDpf, emit func(uint64, *pb.PartialAggregationDpf)) error {
	noise, err := distributednoise.DistributedGeometricMechanismRand(fn.Epsilon, fn.L1Sensitivity, numberOfHelpers)
	if err != nil {
		return err
	}
	// Overflow of the noise is expected, and there's 50% probability that the noise is negative.
	pa.PartialSum += uint64(noise)
	emit(id, pa)
	return nil
}

func addNoise(scope beam.Scope, rawResult beam.PCollection, epsilon float64, l1Sensitivity uint64) beam.PCollection {
	scope = scope.Scope("AddNoise")
	return beam.ParDo(scope, &addNoiseFn{Epsilon: epsilon, L1Sensitivity: l1Sensitivity}, rawResult)
}

// CombineParams contains parameters for combining the expanded vectors.
type CombineParams struct {
	// Weather to use directCombine() or segmentCombine() when combining the expanded vectors.
	DirectCombine bool
	// The segment length when using segmentCombine().
	SegmentLength uint64
	// Privacy budget for adding noise to the aggregation.
	Epsilon       float64
	L1Sensitivity uint64
}

// ExpandAndCombineHistogram calculates histograms from the DPF keys and combines them.
func ExpandAndCombineHistogram(scope beam.Scope, evaluationContext beam.PCollection, expandParams *pb.ExpandParameters, combineParams *CombineParams) (beam.PCollection, beam.PCollection, error) {
	bucketIDs, err := incrementaldpf.CalculateBucketID(expandParams.SumParameters, expandParams.Prefixes, expandParams.Levels, expandParams.PreviousLevel)
	if err != nil {
		return beam.PCollection{}, beam.PCollection{}, err
	}

	expanded, evalctx := beam.ParDo2(scope, &expandDpfKeyFn{
		Levels:   expandParams.Levels,
		Prefixes: expandParams.Prefixes,
	}, evaluationContext)

	vectorLength := uint64(len(bucketIDs))
	if bucketIDs == nil {
		finalLevel := expandParams.Levels[len(expandParams.Levels)-1]
		vectorLength = uint64(1) << expandParams.SumParameters.Params[finalLevel].LogDomainSize
	}

	var rawResult beam.PCollection
	if combineParams.DirectCombine {
		rawResult = directCombine(scope, expanded, vectorLength, bucketIDs)
	} else {
		rawResult = segmentCombine(scope, expanded, vectorLength, combineParams.SegmentLength, bucketIDs)
	}

	if combineParams.Epsilon > 0 {
		return addNoise(scope, rawResult, combineParams.Epsilon, combineParams.L1Sensitivity), evalctx, nil
	}
	return rawResult, evalctx, nil
}

// AggregatePartialReportParams contains necessary parameters for function AggregatePartialReport().
type AggregatePartialReportParams struct {
	// Input partial report file path, each line contains an encrypted PartialReportDpf.
	PartialReportURI string
	// Output partial aggregation file path, each line contains a bucket index and a wire-formatted PartialAggregationDpf.
	PartialHistogramURI string
	// Output the evaluation context to save the current state of key expansion.
	EvaluationContextURI string
	// Number of shards when writing the output file.
	Shards int64
	// The private keys for the standard encryption from the helper server.
	HelperPrivateKeys map[string]*pb.StandardPrivateKey
	ExpandParams      *pb.ExpandParameters
	CombineParams     *CombineParams
	// If this is true, we use the evaluation context as the expansion state; otherwise the aggregation always starts with the original partial report.
	UseEvaluationContext bool
}

// AggregatePartialReport reads the partial report and calculates partial aggregation results from it.
func AggregatePartialReport(scope beam.Scope, params *AggregatePartialReportParams) error {
	if err := incrementaldpf.CheckExpansionParameters(
		params.ExpandParams.SumParameters,
		params.ExpandParams.Prefixes,
		params.ExpandParams.Levels,
		params.ExpandParams.PreviousLevel,
	); err != nil {
		return err
	}

	scope = scope.Scope("AggregatePartialreportDpf")

	var evalCtx beam.PCollection
	if params.ExpandParams.PreviousLevel < 0 || !params.UseEvaluationContext {
		encrypted := ReadPartialReport(scope, params.PartialReportURI)
		resharded := beam.Reshuffle(scope, encrypted)
		partialReport := DecryptPartialReport(scope, resharded, params.HelperPrivateKeys)
		evalCtx = CreateEvaluationContext(scope, partialReport, params.ExpandParams.SumParameters)
	} else {
		evalCtxOrig := ReadEvaluationContext(scope, params.PartialReportURI)
		evalCtx = beam.Reshuffle(scope, evalCtxOrig)
	}
	partialHistogram, evalctx, err := ExpandAndCombineHistogram(scope, evalCtx, params.ExpandParams, params.CombineParams)
	if err != nil {
		return err
	}

	isFinalLevel := params.ExpandParams.Levels[len(params.ExpandParams.Levels)-1] == int32(len(params.ExpandParams.SumParameters.Params)-1)
	if !isFinalLevel && params.UseEvaluationContext {
		writeEvaluationContext(scope, evalctx, params.EvaluationContextURI, params.Shards)
	}

	writeHistogram(scope, partialHistogram, params.PartialHistogramURI, params.Shards)
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
	lines, err := ioutils.ReadLines(ctx, filename)
	if err != nil {
		return nil, err
	}
	result := make(map[uint64]*pb.PartialAggregationDpf)
	for _, line := range lines {
		index, aggregation, err := parseHistogram(line)
		if err != nil {
			return nil, err
		}
		result[index] = aggregation
	}
	return result, nil
}

// WriteCompleteHistogram writes the final aggregation result without using a Beam pipeline.
func WriteCompleteHistogram(ctx context.Context, filename string, results map[uint64]CompleteHistogram) error {
	var lines []string
	for _, result := range results {
		lines = append(lines, fmt.Sprintf("%d,%d", result.Index, result.Sum))
	}
	return ioutils.WriteLines(ctx, lines, filename)
}

// ConvertOldParamsToExpandParameter converts the parameters used in the server code the one that is used in the new pipeline.
//
// This function makes the current pipeline compatible with the server implementation, which will be refactored soon.
func ConvertOldParamsToExpandParameter(params *pb.IncrementalDpfParameters, prefixes *pb.HierarchicalPrefixes) (*pb.ExpandParameters, error) {
	if err := incrementaldpf.CheckOldExpansionParameters(params, prefixes); err != nil {
		return nil, err
	}

	expandParams := &pb.ExpandParameters{}
	for _, param := range params.Params {
		expandParams.Levels = append(expandParams.Levels, param.LogDomainSize-1)
	}
	expandParams.Prefixes = prefixes
	expandParams.PreviousLevel = -1

	return expandParams, nil
}
