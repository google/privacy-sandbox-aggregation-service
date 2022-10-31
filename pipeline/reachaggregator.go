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

// Package reachaggregator contains functions that aggregates the Reach partial reports with the DPF protocol.
package reachaggregator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/distributednoise"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"

	// The following packages are required to read files from GCS or local.
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/gcs"
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

const numberOfHelpers = 2

func init() {
	beam.RegisterType(reflect.TypeOf((*dpfpb.EvaluationContext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.AggregatablePayload)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialAggregationDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*alignVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*alignVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*expandDpfKeyFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatRQFn)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*expandedVec)(nil)))
}

type expandedVec struct {
	SumVec []*incrementaldpf.ReachTuple
}

// expandDpfKeyFn expands the DPF keys in a PartialReportDpf into two vectors that represent the contribution to the SUM histogram.
type expandDpfKeyFn struct {
	KeyBitSize int

	UseHierarchy               bool
	FullHierarchy              bool
	PrefixBitSize, EvalBitSize int

	vecCounter beam.Counter
}

func (fn *expandDpfKeyFn) Setup() {
	fn.vecCounter = beam.NewCounter("reach", "expanded-vector-count")
}

func getHierarchyLevels(prefixBitSize, evalBitSize int, fullHierarchy bool) (int, int) {
	if fullHierarchy {
		return prefixBitSize - 1, evalBitSize - 1
	}

	var prefixLevel, evalLevel int = -1, 0
	if prefixBitSize > 0 {
		prefixLevel = 0
		evalLevel = 1
	}
	return prefixLevel, evalLevel
}

func (fn *expandDpfKeyFn) ProcessElement(ctx context.Context, partialReport *pb.PartialReportDpf, emit func(*expandedVec)) error {
	fn.vecCounter.Inc(ctx, 1)

	var params []*dpfpb.DpfParameters
	if fn.UseHierarchy {
		var err error
		params, err = incrementaldpf.GetTupleDPFParameters(fn.PrefixBitSize, fn.EvalBitSize, fn.KeyBitSize, fn.FullHierarchy)
		if err != nil {
			return err
		}
	} else {
		params = []*dpfpb.DpfParameters{incrementaldpf.CreateReachUint64TupleDpfParameters(int32(fn.KeyBitSize))}
	}
	sumCtx, err := incrementaldpf.CreateEvaluationContext(params, partialReport.GetSumKey())
	if err != nil {
		return err
	}

	var vecSum []*incrementaldpf.ReachTuple
	if fn.UseHierarchy {
		prefixLevel, evalLevel := getHierarchyLevels(fn.PrefixBitSize, fn.EvalBitSize, fn.FullHierarchy)
		vecSum, err = incrementaldpf.EvaluateReachTupleBetweenLevels(sumCtx, prefixLevel, evalLevel)
	} else {
		vecSum, err = incrementaldpf.EvaluateReachTuple(sumCtx)
	}
	if err != nil {
		return err
	}

	emit(&expandedVec{SumVec: vecSum})
	return nil
}

func addTupleTo(a, b *incrementaldpf.ReachTuple) {
	a.C += b.C
	a.Rf += b.Rf
	a.R += b.R
	a.Qf += b.Qf
	a.Q += b.Q
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
	initialTuples := make([]*incrementaldpf.ReachTuple, fn.VectorLength)
	for i := range initialTuples {
		initialTuples[i] = &incrementaldpf.ReachTuple{}
	}
	return &expandedVec{SumVec: initialTuples}
}

func (fn *combineVectorFn) AddInput(ctx context.Context, e *expandedVec, p *expandedVec) *expandedVec {
	fn.inputCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.VectorLength; i++ {
		addTupleTo(e.SumVec[i], p.SumVec[i])
	}
	return e
}

func (fn *combineVectorFn) MergeAccumulators(ctx context.Context, a, b *expandedVec) *expandedVec {
	fn.mergeCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.VectorLength; i++ {
		addTupleTo(a.SumVec[i], b.SumVec[i])
	}
	return a
}

type alignVectorFn struct {
	SuffixBitSize int

	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("combinetest", "flattenVecFn_input_count")
	fn.outputCounter = beam.NewCounter("combinetest", "flattenVecFn_output_count")
}

func (fn *alignVectorFn) ProcessElement(ctx context.Context, vec *expandedVec, emit func(uint128.Uint128, *incrementaldpf.ReachTuple)) {
	fn.inputCounter.Inc(ctx, 1)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		index := uint128.From64(uint64(i)).Lsh(uint(fn.SuffixBitSize))
		emit(index, sum)
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

	initialTuples := make([]*incrementaldpf.ReachTuple, fn.Length)
	for i := range initialTuples {
		initialTuples[i] = &incrementaldpf.ReachTuple{}
	}
	return &expandedVec{SumVec: initialTuples}
}

func (fn *combineVectorSegmentFn) AddInput(ctx context.Context, e *expandedVec, p *expandedVec) *expandedVec {
	fn.inputCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.Length; i++ {
		addTupleTo(e.SumVec[i], p.SumVec[i+fn.StartIndex])
	}
	return e
}

func (fn *combineVectorSegmentFn) MergeAccumulators(ctx context.Context, a, b *expandedVec) *expandedVec {
	fn.mergeCounter.Inc(ctx, 1)

	for i := uint64(0); i < fn.Length; i++ {
		addTupleTo(a.SumVec[i], b.SumVec[i])
	}
	return a
}

// alignVectorSegmentFn does the same thing with alignVectorFn, except for starting the bucket index with a given value.
type alignVectorSegmentFn struct {
	StartIndex    uint64
	SuffixBitSize int

	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorSegmentFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_input_count")
	fn.outputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_output_count")
}

func (fn *alignVectorSegmentFn) ProcessElement(ctx context.Context, vec *expandedVec, emit func(uint128.Uint128, *incrementaldpf.ReachTuple)) {
	fn.inputCounter.Inc(ctx, 1)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		index := uint128.From64(uint64(i) + fn.StartIndex).Lsh(uint(fn.SuffixBitSize))
		emit(index, sum)
	}
}

// directCombine aggregates the expanded vectors to a single vector, and then converts it to be a PCollection.
func directCombine(scope beam.Scope, expanded beam.PCollection, vectorLength uint64, suffixBitSize int) beam.PCollection {
	scope = scope.Scope("DirectCombine")
	histogram := beam.Combine(scope, &combineVectorFn{VectorLength: vectorLength}, expanded)
	return beam.ParDo(scope, &alignVectorFn{SuffixBitSize: suffixBitSize}, histogram)
}

// There is an issue when combining large vectors (large domain size):  https://issues.apache.org/jira/browse/BEAM-11916
// As a workaround, we split the vectors into pieces and combine the collection of the smaller vectors instead.
func segmentCombine(scope beam.Scope, expanded beam.PCollection, vectorLength, segmentLength uint64, suffixBitSize int) beam.PCollection {
	scope = scope.Scope("SegmentCombine")
	segmentCount := vectorLength / segmentLength
	var segmentLengths []uint64
	for i := uint64(0); i < segmentCount; i++ {
		segmentLengths = append(segmentLengths, segmentLength)
	}
	remainder := vectorLength % segmentLength
	if remainder != 0 {
		segmentLengths = append(segmentLengths, remainder)
		segmentCount++
	}

	results := make([]beam.PCollection, segmentCount)
	for i := range results {
		pHistogram := beam.Combine(scope, &combineVectorSegmentFn{StartIndex: uint64(i) * segmentLength, Length: segmentLengths[i]}, expanded)
		results[i] = beam.ParDo(scope, &alignVectorSegmentFn{StartIndex: uint64(i) * segmentLength, SuffixBitSize: suffixBitSize}, pHistogram)
	}
	return beam.Flatten(scope, results...)
}

type addNoiseFn struct {
	Epsilon       float64
	L1Sensitivity uint64
}

func (fn *addNoiseFn) ProcessElement(ctx context.Context, id uint64, tuple *incrementaldpf.ReachTuple, emit func(uint64, *incrementaldpf.ReachTuple)) error {
	noise, err := distributednoise.DistributedGeometricMechanismRand(fn.Epsilon, fn.L1Sensitivity, numberOfHelpers)
	if err != nil {
		return err
	}
	// Overflow of the noise is expected, and there's 50% probability that the noise is negative.
	tuple.C += uint64(noise)
	emit(id, tuple)
	return nil
}

func addNoise(scope beam.Scope, rawResult beam.PCollection, epsilon float64, l1Sensitivity uint64) beam.PCollection {
	scope = scope.Scope("AddNoise")
	return beam.ParDo(scope, &addNoiseFn{Epsilon: epsilon, L1Sensitivity: l1Sensitivity}, rawResult)
}

// ExpandAndCombineHistogram calculates histograms from the DPF keys and combines them.
func ExpandAndCombineHistogram(scope beam.Scope, partialReport beam.PCollection, params *AggregatePartialReportParams) (beam.PCollection, error) {
	expanded := beam.ParDo(scope, &expandDpfKeyFn{
		KeyBitSize:    params.KeyBitSize,
		UseHierarchy:  params.UseHierarchy,
		FullHierarchy: params.FullHierarchy,
		PrefixBitSize: params.PrefixBitSize,
		EvalBitSize:   params.EvalBitSize,
	}, partialReport)

	var vectorLength uint64
	var suffixBitSize int
	if params.UseHierarchy {
		vectorLength = uint64(1) << uint64(params.EvalBitSize-params.PrefixBitSize)
		suffixBitSize = params.KeyBitSize - params.EvalBitSize
	} else {
		vectorLength = uint64(1) << uint64(params.KeyBitSize)
	}

	var rawResult beam.PCollection
	if params.CombineParams.DirectCombine {
		rawResult = directCombine(scope, expanded, vectorLength, suffixBitSize)
	} else {
		rawResult = segmentCombine(scope, expanded, vectorLength, params.CombineParams.SegmentLength, suffixBitSize)
	}

	if params.CombineParams.Epsilon > 0 {
		rawResult = addNoise(scope, rawResult, params.CombineParams.Epsilon, params.CombineParams.L1Sensitivity)
	}

	return rawResult, nil
}

// AggregatePartialReportParams contains necessary parameters for function AggregatePartialReport().
type AggregatePartialReportParams struct {
	// Input partial report file path, each line contains an encrypted PartialReportDpf.
	PartialReportURI string
	// Output partial aggregation file path, each line contains a bucket index and a wire-formatted PartialAggregationDpf.
	PartialHistogramURI string
	// Output the partial validity bits.
	PartialValidityURI string
	// Number of shards when writing the output file.
	Shards int64
	// The private keys for the standard encryption from the helper server.
	HelperPrivateKeys map[string]*pb.StandardPrivateKey
	KeyBitSize        int

	UseHierarchy               bool
	FullHierarchy              bool
	PrefixBitSize, EvalBitSize int

	CombineParams *dpfaggregator.CombineParams
}

// AggregatePartialReport reads the partial report and calculates partial aggregation results from it.
func AggregatePartialReport(scope beam.Scope, params *AggregatePartialReportParams) error {
	scope = scope.Scope("AggregatePartialreportDpf")

	encrypted := dpfaggregator.ReadEncryptedPartialReport(scope, params.PartialReportURI)
	decrypted := dpfaggregator.DecryptPartialReport(scope, encrypted, params.HelperPrivateKeys)
	partialHistogram, err := ExpandAndCombineHistogram(scope, decrypted, params)
	if err != nil {
		return err
	}

	WriteReachRQ(scope, partialHistogram, params.PartialValidityURI, params.Shards)
	WriteHistogram(scope, partialHistogram, params.PartialHistogramURI, params.Shards)
	return nil
}

// ReachRQ contains R and Q values from one helper for verification.
type ReachRQ struct {
	R, Q uint64
}

type formatRQFn struct {
	countRQ beam.Counter
}

func (fn *formatRQFn) Setup(ctx context.Context) {
	fn.countRQ = beam.NewCounter("aggregation", "formatRQFn_count_rq")
}

func (fn *formatRQFn) ProcessElement(ctx context.Context, id uint128.Uint128, result *incrementaldpf.ReachTuple, emit func(string)) error {
	b, err := json.Marshal(&ReachRQ{R: result.R, Q: result.Q})
	if err != nil {
		return err
	}
	fn.countRQ.Inc(ctx, 1)
	emit(fmt.Sprintf("%s,%s", id.String(), base64.StdEncoding.EncodeToString(b)))
	return nil
}

// WriteReachRQ writes the R and Q values from one helper into a file.
func WriteReachRQ(s beam.Scope, col beam.PCollection, outputName string, shards int64) {
	s = s.Scope("WriteRQ")
	formatted := beam.ParDo(s, &formatRQFn{}, col)
	pipelineutils.WriteNShardedFiles(s, outputName, shards, formatted)
}

func parseRQ(line string) (uint128.Uint128, *ReachRQ, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return uint128.Zero, nil, fmt.Errorf("got %d number of columns in line %q, expected %d", got, line, want)
	}

	index, err := utils.StringToUint128(cols[0])
	if err != nil {
		return uint128.Zero, nil, err
	}

	bResult, err := base64.StdEncoding.DecodeString(cols[1])
	if err != nil {
		return uint128.Zero, nil, err
	}

	rq := &ReachRQ{}
	if err := json.Unmarshal(bResult, rq); err != nil {
		return uint128.Zero, nil, err
	}
	return index, rq, nil
}

// ReadReachRQ reads the R and Q values from a file.
func ReadReachRQ(ctx context.Context, filename string) (map[uint128.Uint128]*ReachRQ, error) {
	lines, err := utils.ReadLines(ctx, filename)
	if err != nil {
		return nil, err
	}
	result := make(map[uint128.Uint128]*ReachRQ)
	for _, line := range lines {
		id, rq, err := parseRQ(line)
		if err != nil {
			return nil, err
		}
		result[id] = rq
	}
	return result, nil
}

// formatHistogramFn converts the partial aggregation results into a string with bucket ID and wire-formatted PartialAggregationDpf.
type formatHistogramFn struct {
	countBucket beam.Counter
}

func (fn *formatHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "formatHistogramFn_bucket_count")
}

func (fn *formatHistogramFn) ProcessElement(ctx context.Context, index uint128.Uint128, result *incrementaldpf.ReachTuple, emit func(string)) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	fn.countBucket.Inc(ctx, 1)
	emit(fmt.Sprintf("%s,%s", index.String(), base64.StdEncoding.EncodeToString(b)))
	return nil
}

// WriteHistogram writes the aggregation result from one helper into a file.
func WriteHistogram(s beam.Scope, col beam.PCollection, outputName string, shards int64) {
	s = s.Scope("WriteHistogram")
	formatted := beam.ParDo(s, &formatHistogramFn{}, col)
	pipelineutils.WriteNShardedFiles(s, outputName, shards, formatted)
}

func parseHistogram(line string) (uint128.Uint128, *incrementaldpf.ReachTuple, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return uint128.Zero, nil, fmt.Errorf("got %d number of columns in line %q, expected %d", got, line, want)
	}

	index, err := utils.StringToUint128(cols[0])
	if err != nil {
		return uint128.Zero, nil, err
	}

	bResult, err := base64.StdEncoding.DecodeString(cols[1])
	if err != nil {
		return uint128.Zero, nil, err
	}

	aggregation := &incrementaldpf.ReachTuple{}
	if err := json.Unmarshal(bResult, aggregation); err != nil {
		return uint128.Zero, nil, err
	}
	return index, aggregation, nil
}

// ReadPartialHistogram reads the partial aggregation result without using a Beam pipeline.
func ReadPartialHistogram(ctx context.Context, filename string) (map[uint128.Uint128]*incrementaldpf.ReachTuple, error) {
	lines, err := utils.ReadLines(ctx, filename)
	if err != nil {
		return nil, err
	}
	result := make(map[uint128.Uint128]*incrementaldpf.ReachTuple)
	for _, line := range lines {
		index, aggregation, err := parseHistogram(line)
		if err != nil {
			return nil, err
		}
		result[index] = aggregation
	}
	return result, nil
}

// ReachResult contains the aggregation count result together with the verification result.
type ReachResult struct {
	Count        uint64
	Verification uint64
}

// CreatePartialResult generates the partial count and verification results with the aggregation generated by the helper itself and the R/Q values from the other helper.
func CreatePartialResult(otherRQ map[uint128.Uint128]*ReachRQ, selfResult map[uint128.Uint128]*incrementaldpf.ReachTuple) (map[uint128.Uint128]*ReachResult, error) {
	if len(otherRQ) != len(selfResult) {
		return nil, fmt.Errorf("validity size should be the same with result size %d, got %d", len(selfResult), len(otherRQ))
	}

	r, q := make(map[uint128.Uint128]uint64), make(map[uint128.Uint128]uint64)
	for id, result := range selfResult {
		var (
			rq *ReachRQ
			ok bool
		)
		if rq, ok = otherRQ[id]; !ok {
			return nil, fmt.Errorf("bucket %d missing from the other helper", id)
		}
		r[id] = rq.R + result.R
		q[id] = rq.Q + result.Q
	}

	partialResult := make(map[uint128.Uint128]*ReachResult)
	for id, result := range selfResult {
		partialResult[id] = &ReachResult{
			Verification: r[id]*result.Qf - q[id]*result.Rf,
			Count:        result.C,
		}
	}
	return partialResult, nil
}

func formatReachResult(id uint128.Uint128, result *ReachResult) (string, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s,%s", id.String(), base64.StdEncoding.EncodeToString(b)), nil
}

// WriteReachResult writes the count
func WriteReachResult(ctx context.Context, result map[uint128.Uint128]*ReachResult, fileURI string) error {
	var lines []string
	for i, v := range result {
		line, err := formatReachResult(i, v)
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	return utils.WriteLines(ctx, lines, fileURI)
}

func parseReachResult(line string) (uint128.Uint128, *ReachResult, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return uint128.Zero, nil, fmt.Errorf("got %d number of columns in line %q, expected %d", got, line, want)
	}

	index, err := utils.StringToUint128(cols[0])
	if err != nil {
		return uint128.Zero, nil, err
	}

	bResult, err := base64.StdEncoding.DecodeString(cols[1])
	if err != nil {
		return uint128.Zero, nil, err
	}

	aggregation := &ReachResult{}
	if err := json.Unmarshal(bResult, aggregation); err != nil {
		return uint128.Zero, nil, err
	}
	return index, aggregation, nil
}

// ReadReachResult reads the partial count and verification results from a file.
func ReadReachResult(ctx context.Context, fileUIR string) (map[uint128.Uint128]*ReachResult, error) {
	lines, err := utils.ReadLines(ctx, fileUIR)
	if err != nil {
		return nil, err
	}

	result := make(map[uint128.Uint128]*ReachResult)
	for _, line := range lines {
		id, aggregation, err := parseReachResult(line)
		if err != nil {
			return nil, err
		}
		result[id] = aggregation
	}
	return result, nil
}

// MergeReachResults merges the partial counts and the verfications.
func MergeReachResults(result1, result2 map[uint128.Uint128]*ReachResult) (map[uint128.Uint128]*ReachResult, error) {
	if len(result1) != len(result2) {
		return nil, fmt.Errorf("expect partial results with same length, got %d and %d", len(result1), len(result2))
	}

	mergedResult := make(map[uint128.Uint128]*ReachResult)
	for id, agg1 := range result1 {
		var (
			agg2 *ReachResult
			ok   bool
		)
		if agg2, ok = result2[id]; !ok {
			return nil, fmt.Errorf("missing bucket %d in result2", id)
		}
		mergedResult[id] = &ReachResult{
			Verification: agg1.Verification + agg2.Verification,
			Count:        agg1.Count + agg2.Count,
		}
	}
	return mergedResult, nil
}
