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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/distributednoise"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
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
	beam.RegisterType(reflect.TypeOf((*createEvalCtxFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*expandDpfKeyFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*decryptPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatCompleteHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*formatHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*getBucketIDsFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*mergeHistogramFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseEncryptedPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parsePartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parsePartialHistogramFn)(nil)).Elem())

	beam.RegisterType(reflect.TypeOf((*expandedVec)(nil)))
}

// ExpandParameters contains required parameters for expanding the DPF keys.
type ExpandParameters struct {
	Level           int32
	Prefixes        []uint128.Uint128
	PreviousLevel   int32
	DirectExpansion bool
}

// parseEncryptedPartialReportFn parses each line of the input partial report and gets a StandardCiphertext, which represents a encrypted PartialReportDpf.
type parseEncryptedPartialReportFn struct {
	partialReportCounter beam.Counter
}

func (fn *parseEncryptedPartialReportFn) Setup() {
	fn.partialReportCounter = beam.NewCounter("aggregation-prototype", "encrypted-partial-report-count")
}

func (fn *parseEncryptedPartialReportFn) ProcessElement(ctx context.Context, line string, emit func(*pb.AggregatablePayload)) error {
	encrypted, err := reporttypes.DeserializeAggregatablePayload(line)
	if err != nil {
		return err
	}
	emit(encrypted)
	fn.partialReportCounter.Inc(ctx, 1)
	return nil
}

// ReadEncryptedPartialReport reads each line from a file, and parses it as a partial report that contains a encrypted DPF key and the context info.
func ReadEncryptedPartialReport(scope beam.Scope, partialReportFile string) beam.PCollection {
	scope = scope.Scope("ReadEncryptedPartialReport")
	allFiles := pipelineutils.AddStrInPath(partialReportFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	reshuffledLines := beam.Reshuffle(scope, lines)
	return beam.ParDo(scope, &parseEncryptedPartialReportFn{}, reshuffledLines)
}

// decryptPartialReportFn decrypts the StandardCiphertext and gets a PartialReportDpf with the private key from the helper server.
type decryptPartialReportFn struct {
	StandardPrivateKeys map[string]*pb.StandardPrivateKey

	isEncryptedBundle   bool
	nonencryptedCounter beam.Counter
}

func (fn *decryptPartialReportFn) Setup() {
	fn.isEncryptedBundle = true
	fn.nonencryptedCounter = beam.NewCounter("aggregation", "unpack-nonencrypted-count")
}

func (fn *decryptPartialReportFn) ProcessElement(ctx context.Context, encrypted *pb.AggregatablePayload, emit func(*pb.PartialReportDpf)) error {
	privateKey, ok := fn.StandardPrivateKeys[encrypted.KeyId]
	if !ok && encrypted.KeyId != "" {
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

type createEvalCtxFn struct {
	PreviousLevel int32
	KeyBitSize    int

	ctxCounter beam.Counter
}

func (fn *createEvalCtxFn) Setup() {
	fn.ctxCounter = beam.NewCounter("aggregation", "createEvalCtxFn-ctx-count")
}

func (fn *createEvalCtxFn) ProcessElement(ctx context.Context, partialReport *pb.PartialReportDpf, emit func(*dpfpb.EvaluationContext)) error {
	// As the default DpfParameters are known for the given key bit size, we do not need to set sumCtx.Parameters.
	// That way, we can save some data when copying evaluation context from Go to C++.
	sumCtx := &dpfpb.EvaluationContext{}
	sumCtx.Key = partialReport.SumKey
	sumCtx.PreviousHierarchyLevel = fn.PreviousLevel

	emit(sumCtx)
	fn.ctxCounter.Inc(ctx, 1)

	return nil
}

// CreateEvaluationContext creates the DPF evaluation context from the decrypted keys.
func CreateEvaluationContext(s beam.Scope, decryptedReport beam.PCollection, expandParams *ExpandParameters, keyBitSize int) beam.PCollection {
	s = s.Scope("CreateEvaluationContext")
	return beam.ParDo(s, &createEvalCtxFn{
		KeyBitSize:    keyBitSize,
		PreviousLevel: expandParams.PreviousLevel,
	}, decryptedReport)
}

// parsePartialReport parses each line of the input file and gets a DPF evaluation context.
type parsePartialReportFn struct {
	partialReportCounter beam.Counter
}

func (fn *parsePartialReportFn) Setup() {
	fn.partialReportCounter = beam.NewCounter("aggregation", "read-partial-report-count")
}

func (fn *parsePartialReportFn) ProcessElement(ctx context.Context, line string, emit func(*pb.PartialReportDpf)) error {
	b, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return err
	}

	partialReport := &pb.PartialReportDpf{}
	if err := proto.Unmarshal(b, partialReport); err != nil {
		return err
	}
	emit(partialReport)

	fn.partialReportCounter.Inc(ctx, 1)
	return nil
}

// ReadPartialReport reads each line from a file, and parses it as a PartialReport.
func ReadPartialReport(scope beam.Scope, partialReportFile string) beam.PCollection {
	scope = scope.Scope("ReadPartialReport")
	allFiles := pipelineutils.AddStrInPath(partialReportFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	reshuffledLines := beam.Reshuffle(scope, lines)
	return beam.ParDo(scope, &parsePartialReportFn{}, reshuffledLines)
}

type formatPartialReportFn struct {
	partialReportCounter beam.Counter
}

func (fn *formatPartialReportFn) Setup() {
	fn.partialReportCounter = beam.NewCounter("aggregation", "write-partial-report-count")
}

func (fn *formatPartialReportFn) ProcessElement(ctx context.Context, partialReport *pb.PartialReportDpf, emit func(string)) error {
	b, err := proto.Marshal(partialReport)
	if err != nil {
		return err
	}
	emit(base64.StdEncoding.EncodeToString(b))

	fn.partialReportCounter.Inc(ctx, 1)
	return nil
}

// writePartialReport writes the decrypted partial reports into a file.
func writePartialReport(s beam.Scope, col beam.PCollection, outputName string, shards int64) {
	s = s.Scope("WritePartialReport")
	formatted := beam.ParDo(s, &formatPartialReportFn{}, col)
	pipelineutils.WriteNShardedFiles(s, outputName, shards, formatted)
}

type expandedVec struct {
	SumVec []uint64
}

// expandDpfKeyFn expands the DPF keys in a PartialReportDpf into a vector that represent the contribution to the SUM histogram.
type expandDpfKeyFn struct {
	ExpandParams *ExpandParameters
	KeyBitSize   int

	vecCounter      beam.Counter
	cPrefixes       unsafe.Pointer
	cPrefixesLength int64
}

func (fn *expandDpfKeyFn) Setup() {
	fn.vecCounter = beam.NewCounter("aggregation", "expandDpfFn-vec-count")
	fn.cPrefixes, fn.cPrefixesLength = incrementaldpf.CreateCUint128ArrayUnsafe(fn.ExpandParams.Prefixes)
}

func (fn *expandDpfKeyFn) Teardown() {
	incrementaldpf.FreeUnsafePointer(fn.cPrefixes)
}

func (fn *expandDpfKeyFn) ProcessElement(ctx context.Context, evalCtx *dpfpb.EvaluationContext, emitVec func(*expandedVec)) error {
	if int32(fn.ExpandParams.Level) <= evalCtx.PreviousHierarchyLevel {
		return fmt.Errorf("expect current level higher than the previous level %d, got %d", evalCtx.PreviousHierarchyLevel, fn.ExpandParams.Level)
	}

	var (
		vecSum []uint64
		err    error
	)

	// The DpfParameters are not needed here (either in the evaluation context or as a direct input), as they are known when the key bit size is given.
	// That way, we can reduce the data copied from Go to C++.
	if fn.ExpandParams.DirectExpansion {
		vecSum, err = incrementaldpf.EvaluateAt64UnsafeDefault(fn.KeyBitSize, int(fn.ExpandParams.Level), fn.cPrefixes, fn.cPrefixesLength, evalCtx.Key)
	} else {
		vecSum, err = incrementaldpf.EvaluateUntil64UnsafeDefault(fn.KeyBitSize, int(fn.ExpandParams.Level), fn.cPrefixes, fn.cPrefixesLength, evalCtx)
	}
	if err != nil {
		return err
	}

	emitVec(&expandedVec{SumVec: vecSum})

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
	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("combinetest", "flattenVecFn_input_count")
	fn.outputCounter = beam.NewCounter("combinetest", "flattenVecFn_output_count")
}

func (fn *alignVectorFn) ProcessElement(ctx context.Context, vec *expandedVec, bucketIDsIter func(*[]uint128.Uint128) bool, emit func(uint128.Uint128, *pb.PartialAggregationDpf)) error {
	fn.inputCounter.Inc(ctx, 1)

	bucketIDs := []uint128.Uint128{}
	bucketIDsIter(&bucketIDs)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		if len(bucketIDs) != 0 {
			emit(bucketIDs[i], &pb.PartialAggregationDpf{PartialSum: sum})
		} else {
			emit(uint128.From64(uint64(i)), &pb.PartialAggregationDpf{PartialSum: sum})
		}
	}
	return nil
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

	inputCounter  beam.Counter
	outputCounter beam.Counter
}

func (fn *alignVectorSegmentFn) Setup(ctx context.Context) {
	fn.inputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_input_count")
	fn.outputCounter = beam.NewCounter("aggregation", "alignVectorSegmentFn_output_count")
}

func (fn *alignVectorSegmentFn) ProcessElement(ctx context.Context, vec *expandedVec, bucketIDsIter func(*[]uint128.Uint128) bool, emit func(uint128.Uint128, *pb.PartialAggregationDpf)) error {
	fn.inputCounter.Inc(ctx, 1)

	bucketIDs := []uint128.Uint128{}
	bucketIDsIter(&bucketIDs)

	for i, sum := range vec.SumVec {
		fn.outputCounter.Inc(ctx, 1)
		if len(bucketIDs) != 0 {
			emit(bucketIDs[uint64(i)+fn.StartIndex], &pb.PartialAggregationDpf{PartialSum: sum})
		} else {
			emit(uint128.From64(uint64(i)+fn.StartIndex), &pb.PartialAggregationDpf{PartialSum: sum})
		}
	}
	return nil
}

// directCombine aggregates the expanded vectors to a single vector, and then converts it to be a PCollection.
func directCombine(scope beam.Scope, expanded, bucketIDs beam.PCollection, vectorLength uint64) beam.PCollection {
	scope = scope.Scope("DirectCombine")
	histogram := beam.Combine(scope, &combineVectorFn{VectorLength: vectorLength}, expanded)
	return beam.ParDo(scope, &alignVectorFn{}, histogram, beam.SideInput{Input: bucketIDs})
}

// There is an issue when combining large vectors (large domain size):  https://issues.apache.org/jira/browse/BEAM-11916
// As a workaround, we split the vectors into pieces and combine the collection of the smaller vectors instead.
func segmentCombine(scope beam.Scope, expanded, bucketIDs beam.PCollection, vectorLength, segmentLength uint64) beam.PCollection {
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
		results[i] = beam.ParDo(scope, &alignVectorSegmentFn{StartIndex: uint64(i) * segmentLength}, pHistogram, beam.SideInput{Input: bucketIDs})
	}
	return beam.Flatten(scope, results...)
}

type addNoiseFn struct {
	Epsilon       float64
	L1Sensitivity uint64
}

func (fn *addNoiseFn) ProcessElement(ctx context.Context, id uint128.Uint128, pa *pb.PartialAggregationDpf, emit func(uint128.Uint128, *pb.PartialAggregationDpf)) error {
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

type getBucketIDsFn struct {
	Level, PreviousLevel int32
	KeyBitSize           int

	dpfParams        []*dpfpb.DpfParameters
	bucketIDsCounter beam.Counter
}

func (fn *getBucketIDsFn) Setup() error {
	fn.bucketIDsCounter = beam.NewCounter("aggregation-prototype", "bucket-id-count")
	var err error
	fn.dpfParams, err = incrementaldpf.GetDefaultDPFParameters(fn.KeyBitSize)
	return err
}

func (fn *getBucketIDsFn) ProcessElement(ctx context.Context, prefixes []uint128.Uint128, emit func([]uint128.Uint128)) error {
	bucketIDs, err := incrementaldpf.CalculateBucketID(fn.dpfParams, prefixes, fn.Level, fn.PreviousLevel)
	if err != nil {
		return err
	}
	fn.bucketIDsCounter.Inc(ctx, int64(len(bucketIDs)))
	emit(bucketIDs)
	return nil
}

// ExpandAndCombineHistogram calculates histograms from the DPF keys and combines them.
func ExpandAndCombineHistogram(scope beam.Scope, evaluationContext beam.PCollection, expandParams *ExpandParameters, dpfParams []*dpfpb.DpfParameters, combineParams *CombineParams, keyBitSize int) (beam.PCollection, error) {
	prefixes := beam.Create(scope, expandParams.Prefixes)
	var (
		bucketIDs    beam.PCollection
		vectorLength uint64
		err          error
	)
	if expandParams.DirectExpansion {
		bucketIDs = prefixes
		vectorLength = uint64(len(expandParams.Prefixes))
		if vectorLength == 0 {
			return beam.PCollection{}, errors.New("expect nonempty bucket IDs for direct query")
		}
	} else {
		bucketIDs = beam.ParDo(scope, &getBucketIDsFn{Level: expandParams.Level, PreviousLevel: expandParams.PreviousLevel, KeyBitSize: keyBitSize}, prefixes)
		vectorLength, err = incrementaldpf.GetVectorLength(dpfParams, expandParams.Prefixes, expandParams.Level, expandParams.PreviousLevel)
		if err != nil {
			return beam.PCollection{}, err
		}
	}

	expanded := beam.ParDo(scope, &expandDpfKeyFn{
		ExpandParams: expandParams,
		KeyBitSize:   keyBitSize,
	}, evaluationContext)

	var rawResult beam.PCollection
	if combineParams.DirectCombine {
		rawResult = directCombine(scope, expanded, bucketIDs, vectorLength)
	} else {
		rawResult = segmentCombine(scope, expanded, bucketIDs, vectorLength, combineParams.SegmentLength)
	}

	if combineParams.Epsilon > 0 {
		return addNoise(scope, rawResult, combineParams.Epsilon, combineParams.L1Sensitivity), nil
	}
	return rawResult, nil
}

// AggregatePartialReportParams contains necessary parameters for function AggregatePartialReport().
type AggregatePartialReportParams struct {
	// Input partial report file path, each line contains an encrypted PartialReportDpf.
	PartialReportURI string
	// Output partial aggregation file path, each line contains a bucket index and a wire-formatted PartialAggregationDpf.
	PartialHistogramURI string
	// Output the decrypted partial report to track the expansion state.
	DecryptedReportURI string
	// Number of shards when writing the output file.
	Shards int64
	// The private keys for the standard encryption from the helper server.
	HelperPrivateKeys map[string]*pb.StandardPrivateKey
	KeyBitSize        int
	ExpandParams      *ExpandParameters
	CombineParams     *CombineParams
}

// AggregatePartialReport reads the partial report and calculates partial aggregation results from it.
func AggregatePartialReport(scope beam.Scope, params *AggregatePartialReportParams) error {
	dpfParams, err := incrementaldpf.GetDefaultDPFParameters(params.KeyBitSize)
	if err != nil {
		return err
	}

	if err := CheckExpansionParameters(dpfParams, params.ExpandParams); err != nil {
		return err
	}

	scope = scope.Scope("AggregatePartialreportDpf")

	isFinalLevel := params.ExpandParams.Level == int32(len(dpfParams)-1)
	var decryptedReport beam.PCollection
	if params.ExpandParams.PreviousLevel < 0 {
		encrypted := ReadEncryptedPartialReport(scope, params.PartialReportURI)
		decryptedReport = DecryptPartialReport(scope, encrypted, params.HelperPrivateKeys)
		if !isFinalLevel && !params.ExpandParams.DirectExpansion {
			writePartialReport(scope, decryptedReport, params.DecryptedReportURI, params.Shards)
		}
	} else {
		decryptedReport = ReadPartialReport(scope, params.PartialReportURI)
	}
	evalCtx := CreateEvaluationContext(scope, decryptedReport, params.ExpandParams, params.KeyBitSize)
	partialHistogram, err := ExpandAndCombineHistogram(scope, evalCtx, params.ExpandParams, dpfParams, params.CombineParams, params.KeyBitSize)
	if err != nil {
		return err
	}

	writeHistogram(scope, partialHistogram, params.PartialHistogramURI)
	return nil
}

// formatHistogramFn converts the partial aggregation results into a string with bucket ID and wire-formatted PartialAggregationDpf.
type formatHistogramFn struct {
	countBucket beam.Counter
}

func (fn *formatHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "formatHistogramFn_bucket_count")
}

func (fn *formatHistogramFn) ProcessElement(ctx context.Context, index uint128.Uint128, result *pb.PartialAggregationDpf, emit func(string)) error {
	b, err := proto.Marshal(result)
	if err != nil {
		return err
	}
	fn.countBucket.Inc(ctx, 1)
	emit(fmt.Sprintf("%s,%s", index.String(), base64.StdEncoding.EncodeToString(b)))
	return nil
}

func writeHistogram(s beam.Scope, col beam.PCollection, outputName string) {
	s = s.Scope("WriteHistogram")
	formatted := beam.ParDo(s, &formatHistogramFn{}, col)
	textio.Write(s, outputName, formatted)
}

// parsePartialHistogramFn parses each line from the partial aggregation file, and gets a pair of bucket ID and PartialAggregationDpf.
type parsePartialHistogramFn struct {
	countBucket beam.Counter
}

func (fn *parsePartialHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "parsePartialHistogramFn_bucket_count")
}

func parseHistogram(line string) (uint128.Uint128, *pb.PartialAggregationDpf, error) {
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

	aggregation := &pb.PartialAggregationDpf{}
	if err := proto.Unmarshal(bResult, aggregation); err != nil {
		return uint128.Zero, nil, err
	}
	return index, aggregation, nil
}

func (fn *parsePartialHistogramFn) ProcessElement(ctx context.Context, line string, emit func(uint128.Uint128, *pb.PartialAggregationDpf)) error {
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
	allFiles := pipelineutils.AddStrInPath(partialHistogramFile, "*")
	lines := textio.ReadSdf(s, allFiles)
	return beam.ParDo(s, &parsePartialHistogramFn{}, lines)
}

// CompleteHistogram represents the final aggregation result in a histogram.
type CompleteHistogram struct {
	Bucket uint128.Uint128
	Sum    uint64
}

// mergeHistogramFn merges the two PartialAggregationDpf messages for the same bucket ID by summing the results.
type mergeHistogramFn struct {
	countBucket beam.Counter
}

func (fn *mergeHistogramFn) Setup() {
	fn.countBucket = beam.NewCounter("aggregation", "mergeHistogramFn_bucket_count")
}

func (fn *mergeHistogramFn) ProcessElement(ctx context.Context, index uint128.Uint128, pHisIter1 func(**pb.PartialAggregationDpf) bool, pHisIter2 func(**pb.PartialAggregationDpf) bool, emit func(CompleteHistogram)) error {
	var hist1 *pb.PartialAggregationDpf
	if !pHisIter1(&hist1) {
		return fmt.Errorf("expect two shares for bucket ID %s, missing from helper1", index.String())
	}
	var hist2 *pb.PartialAggregationDpf
	if !pHisIter2(&hist2) {
		return fmt.Errorf("expect two shares for bucket ID %s, missing from helper2", index.String())
	}

	if hist1.PartialSum+hist2.PartialSum == 0 {
		return nil
	}

	fn.countBucket.Inc(ctx, 1)
	emit(CompleteHistogram{
		Bucket: index,
		Sum:    hist1.PartialSum + hist2.PartialSum,
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
	return fmt.Sprintf("%s,%d", result.Bucket.String(), result.Sum)
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
func ReadPartialHistogram(ctx context.Context, filename string) (map[uint128.Uint128]*pb.PartialAggregationDpf, error) {
	lines, err := utils.ReadLines(ctx, filename)
	if err != nil {
		return nil, err
	}
	result := make(map[uint128.Uint128]*pb.PartialAggregationDpf)
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
func WriteCompleteHistogram(ctx context.Context, filename string, results map[uint128.Uint128]CompleteHistogram) error {
	var lines []string
	for _, result := range results {
		lines = append(lines, fmt.Sprintf("%s,%d", result.Bucket.String(), result.Sum))
	}
	return utils.WriteLines(ctx, lines, filename)
}

// MergePartialResult merges the partial histograms without using a beam pipeline.
func MergePartialResult(partial1, partial2 map[uint128.Uint128]*pb.PartialAggregationDpf) ([]CompleteHistogram, error) {
	if len(partial1) != len(partial2) {
		return nil, fmt.Errorf("partial results have different lengths: %d and %d", len(partial1), len(partial2))
	}

	var result []CompleteHistogram
	for idx := range partial1 {
		if _, ok := partial2[idx]; !ok {
			return nil, fmt.Errorf("index %s appears in partial1, missing in partial2", idx.String())
		}
		result = append(result, CompleteHistogram{
			Bucket: idx,
			Sum:    partial1[idx].PartialSum + partial2[idx].PartialSum,
		})
	}
	return result, nil
}

// SaveExpandParameters save the ExpandParams into a file.
func SaveExpandParameters(ctx context.Context, params *ExpandParameters, uri string) error {
	b, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return utils.WriteBytes(ctx, b, uri, nil)
}

// ReadExpandParameters reads the ExpandParams from a file.
func ReadExpandParameters(ctx context.Context, uri string) (*ExpandParameters, error) {
	b, err := utils.ReadBytes(ctx, uri)
	if err != nil {
		return nil, err
	}
	params := &ExpandParameters{}
	if err := json.Unmarshal(b, params); err != nil {
		return nil, err
	}
	return params, nil
}

// CheckExpansionParameters checks if the DPF parameters and prefixes are valid for the hierarchical expansion.
func CheckExpansionParameters(dpfParams []*dpfpb.DpfParameters, expandParams *ExpandParameters) error {
	if err := incrementaldpf.CheckLevels(dpfParams, expandParams.Level, expandParams.PreviousLevel); err != nil {
		return err
	}

	if expandParams.DirectExpansion && expandParams.PreviousLevel != -1 {
		return fmt.Errorf("expect PreviousLevel = -1 for direct expansion, got %d", expandParams.PreviousLevel)
	}

	if !expandParams.DirectExpansion {
		if expandParams.PreviousLevel == -1 && len(expandParams.Prefixes) != 0 {
			return fmt.Errorf("prefixes should be empty for the first level hierarchical expansion, got %+v", expandParams.Prefixes)
		}
		if expandParams.PreviousLevel > -1 && len(expandParams.Prefixes) == 0 {
			return fmt.Errorf("prefix cannot be empty for previous level %d", expandParams.PreviousLevel)
		}
	}
	return nil
}
