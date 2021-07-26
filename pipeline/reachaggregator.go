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

// Package reachaggregator contains functions that aggregates the reports with the DPF protocol.
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
package reachaggregator

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
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/distributednoise"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

const numberOfHelpers = 2

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.EncryptedPartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialAggregationDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*addNoiseFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*alignVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*alignVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineVectorSegmentFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*expandDpfKeyFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*decryptPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatRQFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseEncryptedPartialReportFn)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*expandedVec)(nil)))
	beam.RegisterType(reflect.TypeOf((*ReachRQ)(nil)))
	beam.RegisterType(reflect.TypeOf((*ReachResult)(nil)))

	beam.RegisterType(reflect.TypeOf((*incrementaldpf.ReachTuple)(nil)))
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

type expandedVec struct {
	SumVec []*incrementaldpf.ReachTuple
}

// expandDpfKeyFn expands the DPF keys in a PartialReportDpf into two vectors that represent the contribution to the SUM histogram.
type expandDpfKeyFn struct {
	KeyBitSize int32
	vecCounter beam.Counter
}

func (fn *expandDpfKeyFn) Setup() {
	fn.vecCounter = beam.NewCounter("aggregation", "expandDpfFn-vec-count")
}

func (fn *expandDpfKeyFn) ProcessElement(ctx context.Context, partialReport *pb.PartialReportDpf, emit func(*expandedVec)) error {
	fn.vecCounter.Inc(ctx, 1)

	params := incrementaldpf.CreateReachUint64TupleDpfParameters(fn.KeyBitSize)
	sumCtx, err := incrementaldpf.CreateEvaluationContext([]*dpfpb.DpfParameters{params}, partialReport.GetSumKey())
	if err != nil {
		return err
	}

	vecSum, err := incrementaldpf.EvaluateReachTuple(sumCtx)
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
	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("combinetest", "flattenVecFn_input_count")
	fn.outputCounter = beam.NewCounter("combinetest", "flattenVecFn_output_count")
}

func (fn *alignVectorFn) ProcessElement(ctx context.Context, vec *expandedVec, emit func(uint64, *incrementaldpf.ReachTuple)) {
	fn.inputCounter.Inc(ctx, 1)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		emit(uint64(i), sum)
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
	StartIndex uint64

	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorSegmentFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_input_count")
	fn.outputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_output_count")
}

func (fn *alignVectorSegmentFn) ProcessElement(ctx context.Context, vec *expandedVec, emit func(uint64, *incrementaldpf.ReachTuple)) {
	fn.inputCounter.Inc(ctx, 1)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		emit(uint64(i)+fn.StartIndex, sum)
	}
}

// directCombine aggregates the expanded vectors to a single vector, and then converts it to be a PCollection.
func directCombine(scope beam.Scope, expanded beam.PCollection, vectorLength uint64) beam.PCollection {
	scope = scope.Scope("DirectCombine")
	histogram := beam.Combine(scope, &combineVectorFn{VectorLength: vectorLength}, expanded)
	return beam.ParDo(scope, &alignVectorFn{}, histogram)
}

// There is an issue when combining large vectors (large domain size):  https://issues.apache.org/jira/browse/BEAM-11916
// As a workaround, we split the vectors into pieces and combine the collection of the smaller vectors instead.
func segmentCombine(scope beam.Scope, expanded beam.PCollection, vectorLength, segmentLength uint64) beam.PCollection {
	scope = scope.Scope("SegmentCombine")
	results := make([]beam.PCollection, vectorLength/segmentLength)
	for i := range results {
		pHistogram := beam.Combine(scope, &combineVectorSegmentFn{StartIndex: uint64(i) * segmentLength, Length: segmentLength}, expanded)
		results[i] = beam.ParDo(scope, &alignVectorSegmentFn{StartIndex: uint64(i) * segmentLength}, pHistogram)
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

// AggregatePartialReportParams contains necessary parameters for function AggregatePartialReport().
type AggregatePartialReportParams struct {
	// Input partial report file path, each line contains an encrypted PartialReportDpf.
	PartialReportFile string
	// Output partial aggregation file path, each line contains a bucket index and a wire-formatted PartialAggregationDpf.
	PartialHistogramFile string
	// The private keys for the standard encryption from the helper server.
	HelperPrivateKeys map[string]*pb.StandardPrivateKey
	// Weather to use directCombine() or segmentCombine() when combining the expanded vectors.
	DirectCombine bool
	// The segment length when using segmentCombine().
	SegmentLength uint64
	// Number of shards when writing the output file.
	Shards int64
	// Bit size of the conversion keys.
	KeyBitSize int32
	// Privacy budget for adding noise to the aggregation.
	Epsilon       float64
	L1Sensitivity uint64
}

// ExpandAndCombineHistogram calculates histograms from the DPF keys and combines them.
func ExpandAndCombineHistogram(scope beam.Scope, partialReport beam.PCollection, params *AggregatePartialReportParams) (beam.PCollection, error) {
	expanded := beam.ParDo(scope, &expandDpfKeyFn{
		KeyBitSize: params.KeyBitSize,
	}, partialReport)

	vectorLength := uint64(1) << uint64(params.KeyBitSize)
	var rawResult beam.PCollection
	if params.DirectCombine {
		rawResult = directCombine(scope, expanded, vectorLength)
	} else {
		rawResult = segmentCombine(scope, expanded, vectorLength, params.SegmentLength)
	}

	if params.Epsilon > 0 {
		rawResult = addNoise(scope, rawResult, params.Epsilon, params.L1Sensitivity)
	}

	return rawResult, nil
}

// AggregatePartialReport reads the partial report and calculates partial aggregation results from it.
func AggregatePartialReport(scope beam.Scope, params *AggregatePartialReportParams) error {
	scope = scope.Scope("AggregatePartialreportDpf")

	encrypted := ReadPartialReport(scope, params.PartialReportFile)
	resharded := beam.Reshuffle(scope, encrypted)

	partialReport := DecryptPartialReport(scope, resharded, params.HelperPrivateKeys)
	partialHistogram, err := ExpandAndCombineHistogram(scope, partialReport, params)
	if err != nil {
		return err
	}

	WriteHistogram(scope, partialHistogram, params.PartialHistogramFile, params.Shards)
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

func (fn *formatRQFn) ProcessElement(ctx context.Context, id uint64, result *incrementaldpf.ReachTuple, emit func(string)) error {
	b, err := json.Marshal(&ReachRQ{R: result.R, Q: result.Q})
	if err != nil {
		return err
	}
	fn.countRQ.Inc(ctx, 1)
	emit(fmt.Sprintf("%d,%s", id, base64.StdEncoding.EncodeToString(b)))
	return nil
}

// WriteReachRQ writes the R and Q values from one helper into a file.
func WriteReachRQ(s beam.Scope, col beam.PCollection, outputName string, shards int64) {
	s = s.Scope("WriteRQ")
	formatted := beam.ParDo(s, &formatRQFn{}, col)
	ioutils.WriteNShardedFiles(s, outputName, shards, formatted)
}

func parseRQ(line string) (uint64, *ReachRQ, error) {
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

	rq := &ReachRQ{}
	if err := json.Unmarshal(bResult, rq); err != nil {
		return 0, nil, err
	}
	return index, rq, nil
}

// ReadReachRQ reads the R and Q values from a file.
func ReadReachRQ(ctx context.Context, filename string) (map[uint64]*ReachRQ, error) {
	lines, err := ioutils.ReadLines(ctx, filename)
	if err != nil {
		return nil, err
	}
	result := make(map[uint64]*ReachRQ)
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

func (fn *formatHistogramFn) ProcessElement(ctx context.Context, index uint64, result *incrementaldpf.ReachTuple, emit func(string)) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	fn.countBucket.Inc(ctx, 1)
	emit(fmt.Sprintf("%d,%s", index, base64.StdEncoding.EncodeToString(b)))
	return nil
}

// WriteHistogram writes the aggregation result from one helper into a file.
func WriteHistogram(s beam.Scope, col beam.PCollection, outputName string, shards int64) {
	s = s.Scope("WriteHistogram")
	formatted := beam.ParDo(s, &formatHistogramFn{}, col)
	ioutils.WriteNShardedFiles(s, outputName, shards, formatted)
}

func parseHistogram(line string) (uint64, *incrementaldpf.ReachTuple, error) {
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

	aggregation := &incrementaldpf.ReachTuple{}
	if err := json.Unmarshal(bResult, aggregation); err != nil {
		return 0, nil, err
	}
	return index, aggregation, nil
}

// ReadPartialHistogram reads the partial aggregation result without using a Beam pipeline.
func ReadPartialHistogram(ctx context.Context, filename string) (map[uint64]*incrementaldpf.ReachTuple, error) {
	lines, err := ioutils.ReadLines(ctx, filename)
	if err != nil {
		return nil, err
	}
	result := make(map[uint64]*incrementaldpf.ReachTuple)
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
func CreatePartialResult(otherRQ map[uint64]*ReachRQ, selfResult map[uint64]*incrementaldpf.ReachTuple) (map[uint64]*ReachResult, error) {
	for id := range otherRQ {
		if _, ok := selfResult[id]; !ok {
			return nil, fmt.Errorf("bucket %d missing from the aggregation results", id)
		}
	}

	r, q := make(map[uint64]uint64), make(map[uint64]uint64)
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

	partialResult := make(map[uint64]*ReachResult)
	for id, result := range selfResult {
		partialResult[id] = &ReachResult{
			Verification: r[id]*result.Qf - q[id]*result.Rf,
			Count:        result.C,
		}
	}
	return partialResult, nil
}

func formatReachResult(id uint64, result *ReachResult) (string, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d,%s", id, base64.StdEncoding.EncodeToString(b)), nil
}

// WriteReachResult writes the count
func WriteReachResult(ctx context.Context, result map[uint64]*ReachResult, fileURI string) error {
	var lines []string
	for i, v := range result {
		line, err := formatReachResult(i, v)
		if err != nil {
			return err
		}
		lines = append(lines, line)
	}
	return ioutils.WriteLines(ctx, lines, fileURI)
}

func parseReachResult(line string) (uint64, *ReachResult, error) {
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

	aggregation := &ReachResult{}
	if err := json.Unmarshal(bResult, aggregation); err != nil {
		return 0, nil, err
	}
	return index, aggregation, nil
}

// ReadReachResult reads the partial count and verification results from a file.
func ReadReachResult(ctx context.Context, fileUIR string) (map[uint64]*ReachResult, error) {
	lines, err := ioutils.ReadLines(ctx, fileUIR)
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]*ReachResult)
	for _, line := range lines {
		id, aggregation, err := parseReachResult(line)
		if err != nil {
			return nil, err
		}
		result[id] = aggregation
	}
	return result, nil
}

// CheckAndmergeReachResults merges the partial count results and does the verfication.
func CheckAndmergeReachResults(result1, result2 map[uint64]*ReachResult) (map[uint64]*ReachResult, error) {
	for id := range result2 {
		if _, ok := result1[id]; !ok {
			return nil, fmt.Errorf("missing bucket %d in result1", id)
		}
	}

	mergedResult := make(map[uint64]*ReachResult)
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
