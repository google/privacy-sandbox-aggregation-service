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

// Package dpfbrowsersimulator simulates the browser behavior under the DPF protocol.
package dpfbrowsersimulator

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
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*encryptSecretSharesFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawConversionFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*rawConversion)(nil)))
	beam.RegisterFunction(formatPartialReportFn)
}

// For the DPF protocol the record key is an integer in a known domain.
type rawConversion struct {
	Index uint64
	Value uint64
}

// parseRawConversionFn parses each line in the raw conversion file in the format: bucket ID, value.
type parseRawConversionFn struct {
	countConversion beam.Counter
}

func (fn *parseRawConversionFn) Setup(ctx context.Context) {
	fn.countConversion = beam.NewCounter("aggregation", "parserawConversionFn_conversion_count")
}

func (fn *parseRawConversionFn) ProcessElement(ctx context.Context, line string, emit func(rawConversion)) error {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key64, err := strconv.ParseUint(cols[0], 10, 64)
	if err != nil {
		return err
	}

	value64, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return err
	}

	fn.countConversion.Inc(ctx, 1)
	emit(rawConversion{
		Index: key64,
		Value: value64,
	})
	return nil
}

type encryptSecretSharesFn struct {
	PublicKey1, PublicKey2                       *pb.StandardPublicKey
	// Parameters for the DPF secret key generation. These parameters need to be consistent with ones used on the helper servers.
	LogN, LogElementSizeSum, LogElementSizeCount uint64

	countReport beam.Counter
}

func encryptPartialReport(partialReport *pb.PartialReportDpf, key *pb.StandardPublicKey) (*pb.StandardCiphertext, error) {
	bPartialReport, err := proto.Marshal(partialReport)
	if err != nil {
		return nil, err
	}

	encrypted, err := standardencrypt.Encrypt(bPartialReport, key)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func (fn *encryptSecretSharesFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "encryptSecretSharesFn_report_count")
}

func (fn *encryptSecretSharesFn) ProcessElement(ctx context.Context, c rawConversion, emit1 func(*pb.StandardCiphertext), emit2 func(*pb.StandardCiphertext)) error {
	fn.countReport.Inc(ctx, 1)

	keyDpfSum1, keyDpfSum2, err := incrementaldpf.GenerateKeys(&dpfpb.DpfParameters{
		LogDomainSize:  int32(fn.LogN),
		ElementBitsize: 1 << fn.LogElementSizeSum,
	}, c.Index, c.Value)
	if err != nil {
		return err
	}
	keyDpfCount1, keyDpfCount2, err := incrementaldpf.GenerateKeys(&dpfpb.DpfParameters{
		LogDomainSize:  int32(fn.LogN),
		ElementBitsize: 1 << fn.LogElementSizeCount,
	}, c.Index, 1)
	if err != nil {
		return err
	}

	encryptedReport1, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey:   keyDpfSum1,
		CountKey: keyDpfCount1,
	}, fn.PublicKey1)
	if err != nil {
		return err
	}
	emit1(encryptedReport1)

	encryptedReport2, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey:   keyDpfSum2,
		CountKey: keyDpfCount2,
	}, fn.PublicKey2)

	if err != nil {
		return err
	}
	emit2(encryptedReport2)
	return nil
}

// Since we store and read the data line by line through plain text files, the output is base64-encoded to avoid writing symbols in the proto wire-format that are interpreted as line breaks.
func formatPartialReportFn(encrypted *pb.StandardCiphertext, emit func(string)) error {
	bEncrypted, err := proto.Marshal(encrypted)
	if err != nil {
		return err
	}
	emit(base64.StdEncoding.EncodeToString(bEncrypted))
	return nil
}

func writePartialReport(s beam.Scope, output beam.PCollection, outputTextName string, shards int64) {
	s = s.Scope("WriteEncryptedPartialReportDpf")
	formatted := beam.ParDo(s, formatPartialReportFn, output)
	ioutils.WriteNShardedFiles(s, outputTextName, shards, formatted)
}

// splitRawConversion splits the raw report records into secret shares and encrypts them with public keys from helpers.
func splitRawConversion(s beam.Scope, reports beam.PCollection, params *GeneratePartialReportParams) (beam.PCollection, beam.PCollection) {
	s = s.Scope("SplitRawConversion")

	return beam.ParDo2(s,
		&encryptSecretSharesFn{
			PublicKey1:          params.PublicKey1,
			PublicKey2:          params.PublicKey2,
			LogN:                params.LogN,
			LogElementSizeSum:   params.LogElementSizeSum,
			LogElementSizeCount: params.LogElementSizeCount,
		}, reports)
}

// GeneratePartialReportParams contains required parameters for generating partial reports.
type GeneratePartialReportParams struct {
	ConversionFile, PartialReportFile1, PartialReportFile2 string
	LogN, LogElementSizeSum, LogElementSizeCount           uint64
	PublicKey1, PublicKey2                                 *pb.StandardPublicKey
	Shards                                                 int64
}

// GeneratePartialReport splits the raw reports into two shares and encrypts them with public keys from corresponding helpers.
func GeneratePartialReport(scope beam.Scope, params *GeneratePartialReportParams) {
	scope = scope.Scope("GeneratePartialReports")

	allFiles := ioutils.AddStrInPath(params.ConversionFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	rawConversions := beam.ParDo(scope, &parseRawConversionFn{}, lines)
	resharded := beam.Reshuffle(scope, rawConversions)

	partialReport1, partialReport2 := splitRawConversion(scope, resharded, params)
	writePartialReport(scope, partialReport1, params.PartialReportFile1, params.Shards)
	writePartialReport(scope, partialReport2, params.PartialReportFile2, params.Shards)
}
