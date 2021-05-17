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
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*encryptSecretSharesFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawConversionFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*RawConversion)(nil)))
	beam.RegisterFunction(formatPartialReportFn)
}

// RawConversion represents a conversion record from the browser. For the DPF protocol the record key is an integer in a known domain.
type RawConversion struct {
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

func (fn *parseRawConversionFn) ProcessElement(ctx context.Context, line string, emit func(RawConversion)) error {
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
	emit(RawConversion{
		Index: key64,
		Value: value64,
	})
	return nil
}

type encryptSecretSharesFn struct {
	PublicKey1, PublicKey2 *pb.StandardPublicKey
	// Parameters for the DPF secret key generation. These parameters need to be consistent with ones used on the helper servers.
	SumParameters []*dpfpb.DpfParameters

	countReport beam.Counter
}

func encryptPartialReport(partialReport *pb.PartialReportDpf, key *pb.StandardPublicKey) (*pb.StandardCiphertext, error) {
	bPartialReport, err := proto.Marshal(partialReport)
	if err != nil {
		return nil, err
	}

	encrypted, err := standardencrypt.Encrypt(bPartialReport, nil, key)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func (fn *encryptSecretSharesFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "encryptSecretSharesFn_report_count")
}

// putValueForHierarchies determines which value to use when the keys are expanded for different hierarchies.
//
// For now, the values are the same for all the levels.
func putValueForHierarchies(params []*dpfpb.DpfParameters, value uint64) []uint64 {
	values := make([]uint64, len(params))
	for i := range values {
		values[i] = value
	}
	return values
}

func (fn *encryptSecretSharesFn) ProcessElement(ctx context.Context, c RawConversion, emit1 func(*pb.StandardCiphertext), emit2 func(*pb.StandardCiphertext)) error {
	fn.countReport.Inc(ctx, 1)

	keyDpfSum1, keyDpfSum2, err := incrementaldpf.GenerateKeys(fn.SumParameters, c.Index, putValueForHierarchies(fn.SumParameters, c.Value))
	if err != nil {
		return err
	}

	encryptedReport1, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey: keyDpfSum1,
	}, fn.PublicKey1)
	if err != nil {
		return err
	}
	emit1(encryptedReport1)

	encryptedReport2, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey: keyDpfSum2,
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
			PublicKey1:    params.PublicKey1,
			PublicKey2:    params.PublicKey2,
			SumParameters: params.SumParameters.Params,
		}, reports)
}

// GeneratePartialReportParams contains required parameters for generating partial reports.
type GeneratePartialReportParams struct {
	ConversionFile, PartialReportFile1, PartialReportFile2 string
	SumParameters                                          *pb.IncrementalDpfParameters
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

// PrefixNode represents a node in the tree that defines the conversion key prefix hierarchy.
//
// The node represents a segment of bits in the numeric conversion key, which records a certain type of information.
type PrefixNode struct {
	// A note for what type of information this node records. e.g. "campaignid", "geo", or anything contained in the conversion key.
	Class string
	// The value and length of the bit segment represented by this node.
	BitsValue, BitsSize uint64
	// The value and length of the prefix bits accumulated from the root to this node.
	PrefixValue, PrefixSize uint64
	Children                []*PrefixNode
}

// AddChildNode adds a node in the prefix tree.
func (pn *PrefixNode) AddChildNode(class string, bitsSize, bitsValue uint64) *PrefixNode {
	child := &PrefixNode{
		Class:    class,
		BitsSize: bitsSize, BitsValue: bitsValue,
		PrefixSize: bitsSize + pn.PrefixSize, PrefixValue: pn.PrefixValue<<bitsSize + bitsValue,
	}
	pn.Children = append(pn.Children, child)
	return child
}

// CalculatePrefixes calculates the prefixes for each expansion hierarchy and prefix bit sizes.
//
// The prefixes will be used for expanding the DPF keys in different levels; and the prefix bit sizes will be used to determine DPF parameters for DPF key generation and expansion.
func CalculatePrefixes(root *PrefixNode) (*pb.HierarchicalPrefixes, []uint64) {
	var curNodes, nxtNodes []*PrefixNode
	curNodes = append(curNodes, root.Children...)

	prefixes := &pb.HierarchicalPrefixes{}
	// For the first level of expansion, the prefixes must be empty:
	// http://github.com/google/distributed_point_functions/dpf/distributed_point_function.h?l=86&rcl=368846188
	prefixes.Prefixes = append(prefixes.Prefixes, &pb.DomainPrefixes{})
	var prefixBitSizes []uint64
	for len(curNodes) > 0 {
		var prefix []uint64
		var prefixBitSize uint64
		nxtNodes = []*PrefixNode{}
		for _, node := range curNodes {
			prefix = append(prefix, node.PrefixValue)
			prefixBitSize = node.PrefixSize
			nxtNodes = append(nxtNodes, node.Children...)
		}
		prefixes.Prefixes = append(prefixes.Prefixes, &pb.DomainPrefixes{Prefix: prefix})
		prefixBitSizes = append(prefixBitSizes, prefixBitSize)
		curNodes = nxtNodes
	}
	return prefixes, prefixBitSizes
}

// CalculateParameters gets the DPF parameters for DPF key generation and expansion.
//
// 'prefixBitSizes' defines the domain sizes for the DPF key expansion on different hierarchies; and 'logN' defines the total size of domain for the final histogram.
func CalculateParameters(prefixBitSizes []uint64, logN, elementBitSizeSum int32) *pb.IncrementalDpfParameters {
	var sumParams []*dpfpb.DpfParameters
	for _, p := range prefixBitSizes {
		sumParams = append(sumParams, &dpfpb.DpfParameters{LogDomainSize: int32(p), ElementBitsize: elementBitSizeSum})
	}
	sumParams = append(sumParams, &dpfpb.DpfParameters{LogDomainSize: logN, ElementBitsize: elementBitSizeSum})
	return &pb.IncrementalDpfParameters{Params: sumParams}
}

// CreateConversionIndex generates a random conversion ID that matches one of the prefixes described by the input, or does not have any of the prefixes.
func CreateConversionIndex(prefixes []uint64, prefixBitSize, totalBitSize uint64, hasPrefix bool) (uint64, error) {
	if prefixBitSize > totalBitSize {
		return 0, fmt.Errorf("expect a prefix bit size no larger than total bit size %d, got %d", totalBitSize, prefixBitSize)
	}
	if len(prefixes) == 1<<prefixBitSize || len(prefixes) == 0 {
		if hasPrefix {
			return uint64(rand.Int63n(int64(1) << totalBitSize)), nil
		}
		return 0, errors.New("unable to generate an index without any of the prefixes")
	}

	suffixBitSize := totalBitSize - prefixBitSize
	if hasPrefix {
		return prefixes[rand.Intn(len(prefixes))]<<suffixBitSize + uint64(rand.Int63n(1<<suffixBitSize)), nil
	}

	existing := make(map[uint64]struct{})
	for _, p := range prefixes {
		existing[p] = struct{}{}
	}
	var otherPrefix uint64
	for {
		otherPrefix = uint64(rand.Int63n(int64(1) << prefixBitSize))
		if _, ok := existing[otherPrefix]; !ok {
			break
		}
	}
	return otherPrefix<<suffixBitSize | uint64(rand.Int63n(1<<suffixBitSize)), nil
}
