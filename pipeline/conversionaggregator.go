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

// Package conversionaggregator defines the aggregation pipeline for the conversion measurements.
package conversionaggregator

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"reflect"
	"strings"

	"github.com/google/differential-privacy/privacy-on-beam/pbeam"
	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/conversion"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/secretshare"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	// The following packages are required to read files from GCS or local.
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/gcs"
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

// The privacy budget used for adding Laplace noise in the aggregation.
var epsilon = math.Log(3)

const (
	delta    = 1e-5
	minValue = 0
	maxValue = 100
)

func init() {
	beam.RegisterType(reflect.TypeOf((*assemblePartialAggregationFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*combineKeyShareWithMaximumReportIDFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialAggregation)(nil)).Elem())
	beam.RegisterFunction(assembleSingleAggregatedReportFn)
	beam.RegisterFunction(extractAggregationKeyValuesFn)
	beam.RegisterFunction(formatCompleteAggregationFn)
	beam.RegisterFunction(formatPartialAggregationFn)
	beam.RegisterFunction(mergeAggregationFn)
	beam.RegisterFunction(parsePartialAggregationFn)
}

// PrivacyParams contains parameters required for aggregation with differential privacy.
//
// These parameters are the same with the those defined in
// http://google3/third_party/differential_privacy/go/plume/pbeam/count.go?l=31&rcl=340419070 and
// http://google3/third_party/differential_privacy/go/plume/pbeam/sum.go?l=42&rcl=336106761
type PrivacyParams struct {
	Epsilon, Delta, MinValue, MaxValue float64
}

func extractAggregationKeyValuesFn(data conversion.AggData) (string, int64) {
	return data.AggID, data.ValueShare
}

// aggregateSumDP and aggregateCountDP apply the sum and count aggregations with differential privacy based on the Plume libraries.
//
// Note that in Plume privacy is further protected by occasionally dropping aggregation keys with small counts (http://google3/third_party/differential_privacy/go/library/dpagg/select_partition.go?l=30&rcl=339027652).
func aggregateSumDP(s beam.Scope, pCol pbeam.PrivatePCollection, params PrivacyParams) beam.PCollection {
	s = s.Scope("AggregateSumDP")

	keyToValue := pbeam.ParDo(s, extractAggregationKeyValuesFn, pCol)
	return pbeam.SumPerKey(s, keyToValue, pbeam.SumParams{
		NoiseKind:                pbeam.LaplaceNoise{},
		MaxPartitionsContributed: 1,
		MinValue:                 params.MinValue,
		MaxValue:                 params.MaxValue,
		Epsilon:                  params.Epsilon,
		Delta:                    params.Delta,
	})
}

func extractAggregationKeysFn(data conversion.AggData) string {
	return data.AggID
}

// TODO: We should make sure the count noise is applied deterministically based on a shared seed. For now as we are using the plume count function, the noises may not be consistent for a AggID on different helpers.
func aggregateCountDP(s beam.Scope, pCol pbeam.PrivatePCollection, params PrivacyParams) beam.PCollection {
	s = s.Scope("AggregateCountDP")

	keys := pbeam.ParDo(s, extractAggregationKeysFn, pCol)

	return pbeam.DistinctPrivacyID(s, keys, pbeam.DistinctPrivacyIDParams{
		NoiseKind:                pbeam.LaplaceNoise{},
		MaxPartitionsContributed: 1,
		Epsilon:                  params.Epsilon,
		Delta:                    params.Delta,
	})
}

type assemblePartialAggregationFn struct {
	uniqueAggIDCounter beam.Counter
}

func (fn *assemblePartialAggregationFn) Setup() {
	fn.uniqueAggIDCounter = beam.NewCounter("aggregation-prototype", "unique-aggid-count")
}

func (fn *assemblePartialAggregationFn) ProcessElement(ctx context.Context, aggID string, countIter, sumIter func(*int64) bool, emit func(string, *pb.PartialAggregation)) {
	var c int64
	countIter(&c)

	var s int64
	sumIter(&s)

	fn.uniqueAggIDCounter.Inc(ctx, 1)
	emit(aggID, &pb.PartialAggregation{
		// The int64 summation is directly cast into uint32 intentionally. We are using the overflow behavior of uint32 to handle the summation of secret shares.
		PartialSum:   uint32(s),
		PartialCount: c,
	})
}

func extractKeyFn(key string, value int) string {
	return key
}

func valueToInt64Fn(key string, value int) (string, int64) {
	return key, int64(value)
}

// aggregateOrig counts and sums the share values for each unique AggID in the PCollection<AggData> WITHOUT differential privacy.
func aggregateOrig(s beam.Scope, col beam.PCollection) beam.PCollection {
	cCol := beam.ParDo(s, extractAggregationKeysFn, col)
	aggCountOrig := stats.Count(s, cCol)
	// Convert the value type to be consistent with the DP aggregation results.
	aggCount := beam.ParDo(s, valueToInt64Fn, aggCountOrig)

	sCol := beam.ParDo(s, extractAggregationKeyValuesFn, col)
	aggSum := stats.SumPerKey(s, sCol)

	joined := beam.CoGroupByKey(s, aggCount, aggSum)
	return beam.ParDo(s, &assemblePartialAggregationFn{}, joined)
}

// aggregateDP calculates the count and sum for each unique AggID in the input PCollection<AggData> WITH differential privacy.
//
// Despite adding a Laplace noise in summation, privacy is further protected by dropping AggIDs with small numbers of privacy IDs: http://google3/third_party/differential_privacy/go/library/dpagg/select_partition.go?l=30&rcl=346079936
func aggregateDP(s beam.Scope, col beam.PCollection, params PrivacyParams) beam.PCollection {
	spec := pbeam.NewPrivacySpec(params.Epsilon, params.Delta)
	// There are no privacy identifiers in the conversion data, which is needed by the Plume
	// aggregation functions. We use the ephemeral report ID as the record-level privacy identifier.
	// In the final API, we can achieve user-level bounds by capping contributions on the client side.
	pCol := pbeam.MakePrivateFromStruct(s, col, spec, "ReportID")

	aggCount := aggregateCountDP(s, pCol, PrivacyParams{
		Epsilon: params.Epsilon / 2,
		Delta:   params.Delta / 2,
	})
	aggSum := aggregateSumDP(s, pCol, PrivacyParams{
		Epsilon:  params.Epsilon / 2,
		Delta:    params.Delta / 2,
		MaxValue: params.MaxValue,
		MinValue: params.MinValue,
	})

	joined := beam.CoGroupByKey(s, aggCount, aggSum)
	return beam.ParDo(s, &assemblePartialAggregationFn{}, joined)
}

func assembleSingleAggregatedReportFn(aggID string, keySharesIter func(*string) bool, resultsIter func(**pb.PartialAggregation) bool, emit func(string, *pb.PartialAggregation)) {
	var keyShare string
	keySharesIter(&keyShare)

	var result *pb.PartialAggregation
	resultsIter(&result)
	if result == nil {
		// Emit nothing if there's no aggregation info. Some keys may have been ignored because of differential privacy.
		return
	}

	result.KeyShare = []byte(keyShare)
	emit(aggID, result)
}

// For the same conversion key, we keep the key share with the maximum ephemeral report ID.
type combineKeyShareWithMaximumReportIDFn struct{}

func (fn *combineKeyShareWithMaximumReportIDFn) MergeAccumulators(a, b conversion.IDKeyShare) conversion.IDKeyShare {
	if a.ReportID > b.ReportID {
		return a
	}
	return b
}

func (fn *combineKeyShareWithMaximumReportIDFn) ExtractOutput(idKeyShare conversion.IDKeyShare) string {
	return string(idKeyShare.KeyShare)
}

// AggregateDataShare outputs a PCollection<AggID, PartialAggregation> for the input data, applying differential privacy or not based on the input parameters.
func AggregateDataShare(s beam.Scope, aggIDKeyShare, aggData beam.PCollection, ignorePrivacy bool, params PrivacyParams) beam.PCollection {
	s = s.Scope("AggregateShares")
	aggregatedKeyShares := beam.CombinePerKey(s, &combineKeyShareWithMaximumReportIDFn{}, aggIDKeyShare)

	var aggIDResult beam.PCollection
	if ignorePrivacy {
		aggIDResult = aggregateOrig(s, aggData)
	} else {
		aggIDResult = aggregateDP(s, aggData, params)
	}
	joined := beam.CoGroupByKey(s, aggregatedKeyShares, aggIDResult)
	return beam.ParDo(s, assembleSingleAggregatedReportFn, joined)
}

func formatPartialAggregationFn(aggID string, result *pb.PartialAggregation, emit func(string)) error {
	b, err := proto.Marshal(result)
	if err != nil {
		return err
	}
	emit(fmt.Sprintf("%s,%s", base64.StdEncoding.EncodeToString([]byte(aggID)), base64.StdEncoding.EncodeToString(b)))
	return nil
}

// Format and write the partial aggregation results in PCollection<AggID, PartialAggregation>.
func writePartialAggregation(s beam.Scope, aggIDResult beam.PCollection, fileName string) {
	s = s.Scope("WritePartialAggregation")
	formatted := beam.ParDo(s, formatPartialAggregationFn, aggIDResult)
	textio.Write(s, fileName, formatted)
}

// AggregateParams contains necessary parameters for PartialReport aggregation.
type AggregateParams struct {
	PartialReport, ReencryptedKey, PartialAggregation string
	HelperInfo                                        *conversion.ServerPrivateInfo
	IgnorePrivacy                                     bool
}

// AggregatePartialReport calculates the AggID and aggregates data for each unique AggID.
func AggregatePartialReport(scope beam.Scope, aggParams *AggregateParams, privacyParams PrivacyParams) {
	scope = scope.Scope("AggregatePartialReport")

	reportIDKeys := conversion.ReadExponentiatedKeys(scope, aggParams.ReencryptedKey)

	encrypted := conversion.ReadPartialReport(scope, aggParams.PartialReport)
	resharded := beam.Reshuffle(scope, encrypted)
	partialReport := conversion.DecryptPartialReport(scope, resharded, aggParams.HelperInfo.StandardPrivateKey)

	aggIDKeyShare, aggData := conversion.RekeyByAggregationID(scope, reportIDKeys, partialReport, aggParams.HelperInfo.ElGamalPrivateKey, aggParams.HelperInfo.Secret)
	partialAggregation := AggregateDataShare(scope, aggIDKeyShare, aggData, aggParams.IgnorePrivacy, privacyParams)

	writePartialAggregation(scope, partialAggregation, aggParams.PartialAggregation)
}

func parsePartialAggregationFn(line string, emit func(string, *pb.PartialAggregation)) error {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return fmt.Errorf("got %d number of columns in line %q, expected %d", got, line, want)
	}

	bAggID, err := base64.StdEncoding.DecodeString(cols[0])
	if err != nil {
		return err
	}

	bResult, err := base64.StdEncoding.DecodeString(cols[1])
	if err != nil {
		return err
	}

	aggregation := &pb.PartialAggregation{}
	if err := proto.Unmarshal(bResult, aggregation); err != nil {
		return err
	}
	emit(string(bAggID), aggregation)
	return nil
}

func readPartialAggregation(s beam.Scope, partialAggregationFile string) beam.PCollection {
	s = s.Scope("ReadPartialAggregation")
	lines := textio.ReadSdf(s, partialAggregationFile)
	return beam.ParDo(s, parsePartialAggregationFn, lines)
}

// CompleteResult contains the final aggregation result for each conversion key.
type CompleteResult struct {
	ConversionKey string
	Sum           uint32
	Count         int64
}

func mergeAggregationFn(aggID string, pAggIter1 func(**pb.PartialAggregation) bool, pAggIter2 func(**pb.PartialAggregation) bool, emit func(CompleteResult)) error {
	aggregation1 := &pb.PartialAggregation{}
	if !pAggIter1(&aggregation1) {
		log.Printf("expect two shares for aggregation ID %q, missing from helper1", aggID)
		return nil
	}
	aggregation2 := &pb.PartialAggregation{}
	if !pAggIter2(&aggregation2) {
		log.Printf("expect two shares for aggregation ID %q, missing from helper2", aggID)
		return nil
	}
	conversionKey, err := secretshare.CombineByteShares(aggregation1.GetKeyShare(), aggregation2.GetKeyShare())
	if err != nil {
		return err
	}

	emit(CompleteResult{
		ConversionKey: string(conversionKey),
		Sum:           secretshare.CombineIntShares(aggregation1.GetPartialSum(), aggregation2.GetPartialSum()),
		Count:         aggregation1.GetPartialCount() + aggregation2.GetPartialCount(),
	})
	return nil
}

// MergeAggregation combines the partial aggregations to get the complete results.
func MergeAggregation(s beam.Scope, pAgg1, pAgg2 beam.PCollection) beam.PCollection {
	s = s.Scope("MergePartialAggregations")
	joined := beam.CoGroupByKey(s, pAgg1, pAgg2)
	return beam.ParDo(s, mergeAggregationFn, joined)
}

func formatCompleteAggregationFn(result CompleteResult) string {
	return fmt.Sprintf("%s,%d,%d", string(result.ConversionKey), result.Count, result.Sum)
}

func writeCompleteAggregation(s beam.Scope, aggIDResult beam.PCollection, fileName string) {
	s = s.Scope("WriteCompleteAggregation")
	formatted := beam.ParDo(s, formatCompleteAggregationFn, aggIDResult)
	textio.Write(s, fileName, formatted)
}

// MergePartialAggregation calculates the complete result from the aggregated shares.
func MergePartialAggregation(scope beam.Scope, partialAggregationFile1, partialAggregationFile2, completeAggregationFile string) {
	scope = scope.Scope("CompleteAggregate")

	partialAggregation1 := readPartialAggregation(scope, partialAggregationFile1)
	partialAggregation2 := readPartialAggregation(scope, partialAggregationFile2)
	completeAggregation := MergeAggregation(scope, partialAggregation1, partialAggregation2)
	writeCompleteAggregation(scope, completeAggregation, completeAggregationFile)
}
