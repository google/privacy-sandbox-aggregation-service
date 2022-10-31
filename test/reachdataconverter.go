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

// Package reachdataconverter contains functions for generating test reports for the Reach frequency.
package reachdataconverter

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelinetypes"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
	"github.com/google/privacy-sandbox-aggregation-service/test/dpfdataconverter"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.AggregatablePayload)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*createEncryptedPartialReportsFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawReachReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pipelinetypes.RawReachReport)(nil)))
}

func parseRawReachReport(line string, keyBitSize int) (pipelinetypes.RawReachReport, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 4; got != want {
		return pipelinetypes.RawReachReport{}, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	campaign, err := strconv.ParseUint(cols[0], 10, 64)
	if err != nil {
		return pipelinetypes.RawReachReport{}, err
	}

	person, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return pipelinetypes.RawReachReport{}, err
	}

	llRegister, err := utils.StringToUint128(cols[2])
	if err != nil {
		return pipelinetypes.RawReachReport{}, err
	}

	return pipelinetypes.RawReachReport{
		Campaign:   campaign,
		Person:     person,
		LLRegister: llRegister,
		Slice:      cols[3],
	}, nil
}

type parseRawReachReportFn struct {
	KeyBitSize  int
	countReport beam.Counter
}

func (fn *parseRawReachReportFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("reach", "report_count")
}

func (fn *parseRawReachReportFn) ProcessElement(ctx context.Context, line string, emit func(pipelinetypes.RawReachReport)) error {
	report, err := parseRawReachReport(line, fn.KeyBitSize)
	if err != nil {
		return err
	}

	fn.countReport.Inc(ctx, 1)
	emit(report)
	return nil
}

type createEncryptedPartialReportsFn struct {
	PublicKeys1, PublicKeys2 *reporttypes.PublicKeys
	KeyBitSize               int

	UseHierarchy               bool
	FullHierarchy              bool
	PrefixBitSize, EvalBitSize int

	countReport beam.Counter
}

func reachReportToTuple(report pipelinetypes.RawReachReport) *incrementaldpf.ReachTuple {
	r := rand.Uint64()
	q := rand.Uint64()

	return &incrementaldpf.ReachTuple{
		C:  1,
		Rf: r * report.Person,
		R:  r,
		Qf: q * report.Person,
		Q:  q,
	}
}

func (fn *createEncryptedPartialReportsFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("reach", "encrypt_report_count")
}

func (fn *createEncryptedPartialReportsFn) ProcessElement(ctx context.Context, report pipelinetypes.RawReachReport, emit1 func(*pb.AggregatablePayload), emit2 func(*pb.AggregatablePayload)) error {
	var params []*dpfpb.DpfParameters
	var tuples []*incrementaldpf.ReachTuple
	tuple := reachReportToTuple(report)
	if fn.UseHierarchy {
		var err error
		params, err = incrementaldpf.GetTupleDPFParameters(fn.PrefixBitSize, fn.EvalBitSize, fn.KeyBitSize, fn.FullHierarchy)
		if err != nil {
			return err
		}
		tuples = make([]*incrementaldpf.ReachTuple, len(params))
		for i := range tuples {
			tuples[i] = tuple
		}
	} else {
		params = []*dpfpb.DpfParameters{incrementaldpf.CreateReachUint64TupleDpfParameters(int32(fn.KeyBitSize))}
		tuples = []*incrementaldpf.ReachTuple{tuple}
	}

	key1, key2, err := incrementaldpf.GenerateReachTupleKeys(params, report.LLRegister, tuples)
	if err != nil {
		return err
	}
	encryptedReport1, encryptedReport2, err := dpfdataconverter.EncryptPartialReports(key1, key2, fn.PublicKeys1, fn.PublicKeys2, "", true /*encryptOutput*/)
	if err != nil {
		return err
	}

	emit1(encryptedReport1)
	emit2(encryptedReport2)
	fn.countReport.Inc(ctx, 1)

	return nil
}

// createEncryptedPartialReports splits the raw report records into secret shares and encrypts them with public keys from helpers.
func createEncryptedPartialReports(s beam.Scope, reports beam.PCollection, params *GeneratePartialReportParams) (beam.PCollection, beam.PCollection) {
	s = s.Scope("SplitRawConversion")

	return beam.ParDo2(s,
		&createEncryptedPartialReportsFn{
			PublicKeys1:   params.PublicKeys1,
			PublicKeys2:   params.PublicKeys2,
			KeyBitSize:    params.KeyBitSize,
			UseHierarchy:  params.UseHierarchy,
			FullHierarchy: params.FullHierarchy,
			PrefixBitSize: params.PrefixBitSize,
			EvalBitSize:   params.EvalBitSize,
		}, reports)
}

// GeneratePartialReportParams contains required parameters for generating partial reports.
type GeneratePartialReportParams struct {
	ReachReportURI, PartialReportURI1, PartialReportURI2 string
	PublicKeys1, PublicKeys2                             *reporttypes.PublicKeys
	KeyBitSize                                           int
	Shards                                               int64

	UseHierarchy               bool
	FullHierarchy              bool
	PrefixBitSize, EvalBitSize int
}

// GeneratePartialReport splits the raw reports into two shares and encrypts them with public keys from corresponding helpers.
func GeneratePartialReport(scope beam.Scope, params *GeneratePartialReportParams) {
	scope = scope.Scope("GeneratePartialReports")

	allFiles := pipelineutils.AddStrInPath(params.ReachReportURI, "*")
	lines := textio.ReadSdf(scope, allFiles)
	records := beam.ParDo(scope, &parseRawReachReportFn{KeyBitSize: params.KeyBitSize}, lines)
	resharded := beam.Reshuffle(scope, records)

	partialReport1, partialReport2 := createEncryptedPartialReports(scope, resharded, params)
	dpfdataconverter.WritePartialReport(scope, partialReport1, params.PartialReportURI1, params.Shards)
	dpfdataconverter.WritePartialReport(scope, partialReport2, params.PartialReportURI2, params.Shards)
}
