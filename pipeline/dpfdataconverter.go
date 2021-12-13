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

// Package dpfdataconverter simulates the browser behavior under the DPF protocol.
package dpfdataconverter

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
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.EncryptedPartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*encryptSecretSharesFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawConversionFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*reporttypes.RawReport)(nil)))
	beam.RegisterFunction(formatPartialReportFn)
}

// parseRawConversionFn parses each line in the raw conversion file in the format: bucket ID, value.
type parseRawConversionFn struct {
	KeyBitSize      int
	countConversion beam.Counter
}

func (fn *parseRawConversionFn) Setup(ctx context.Context) {
	fn.countConversion = beam.NewCounter("aggregation", "parserawConversionFn_conversion_count")
}

func getMaxKey(s int) uint128.Uint128 {
	maxKey := uint128.Max
	if s < 128 {
		maxKey = uint128.Uint128{1, 0}.Lsh(uint(s)).Sub64(1)
	}
	return maxKey
}

//ParseRawConversion parses a raw conversion.
func ParseRawConversion(line string, keyBitSize int) (reporttypes.RawReport, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return reporttypes.RawReport{}, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key128, err := ioutils.StringToUint128(cols[0])
	if err != nil {
		return reporttypes.RawReport{}, err
	}
	if key128.Cmp(getMaxKey(keyBitSize)) == 1 {
		return reporttypes.RawReport{}, fmt.Errorf("key %q overflows the integer with %d bits", key128.String(), keyBitSize)
	}

	value64, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return reporttypes.RawReport{}, err
	}
	return reporttypes.RawReport{Bucket: key128, Value: value64}, nil
}

func (fn *parseRawConversionFn) ProcessElement(ctx context.Context, line string, emit func(reporttypes.RawReport)) error {
	conversion, err := ParseRawConversion(line, fn.KeyBitSize)
	if err != nil {
		return err
	}

	fn.countConversion.Inc(ctx, 1)
	emit(conversion)
	return nil
}

type encryptSecretSharesFn struct {
	PublicKeys1, PublicKeys2 []cryptoio.PublicKeyInfo
	KeyBitSize               int
	EncryptOutput            bool

	countReport beam.Counter
}

// TODO: Check if the chosen public key is out of date.
func getRandomPublicKey(keys []cryptoio.PublicKeyInfo) (string, *pb.StandardPublicKey, error) {
	keyInfo := keys[rand.Intn(len(keys))]
	bKey, err := base64.StdEncoding.DecodeString(keyInfo.Key)
	if err != nil {
		return "", nil, err
	}
	return keyInfo.ID, &pb.StandardPublicKey{Key: bKey}, nil
}

func encryptPartialReport(partialReport *pb.PartialReportDpf, keys []cryptoio.PublicKeyInfo, contextInfo []byte, encryptOutput bool) (*pb.EncryptedPartialReportDpf, error) {
	bDpfKey, err := proto.Marshal(partialReport.SumKey)
	if err != nil {
		return nil, err
	}

	payload := reporttypes.Payload{
		Operation: "hierarchical-histogram",
		DPFKey:    bDpfKey,
	}
	bPayload, err := ioutils.MarshalCBOR(payload)
	if err != nil {
		return nil, err
	}

	keyID, key, err := getRandomPublicKey(keys)
	if err != nil {
		return nil, err
	}
	// TODO: Remove the option of aggregating reports without encryption when HPKE is ready in Go tink.
	if !encryptOutput {
		return &pb.EncryptedPartialReportDpf{
			EncryptedReport: &pb.StandardCiphertext{Data: bPayload},
			ContextInfo:     contextInfo,
			KeyId:           keyID,
		}, nil
	}

	encrypted, err := standardencrypt.Encrypt(bPayload, contextInfo, key)
	if err != nil {
		return nil, err
	}
	return &pb.EncryptedPartialReportDpf{EncryptedReport: encrypted, ContextInfo: contextInfo, KeyId: keyID}, nil
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

// GenerateEncryptedReports splits a conversion record into DPF keys, and encrypts the partial reports.
func GenerateEncryptedReports(report reporttypes.RawReport, keyBitSize int, publicKeys1, publicKeys2 []cryptoio.PublicKeyInfo, contextInfo []byte, encryptOutput bool) (*pb.EncryptedPartialReportDpf, *pb.EncryptedPartialReportDpf, error) {
	allParams, err := dpfaggregator.GetDefaultDPFParameters(keyBitSize)
	if err != nil {
		return nil, nil, err
	}

	keyDpfSum1, keyDpfSum2, err := incrementaldpf.GenerateKeys(allParams, report.Bucket, putValueForHierarchies(allParams, report.Value))
	if err != nil {
		return nil, nil, err
	}

	encryptedReport1, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey: keyDpfSum1,
	}, publicKeys1, contextInfo, encryptOutput)
	if err != nil {
		return nil, nil, err
	}

	encryptedReport2, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey: keyDpfSum2,
	}, publicKeys2, contextInfo, encryptOutput)

	if err != nil {
		return nil, nil, err
	}

	return encryptedReport1, encryptedReport2, nil
}

func (fn *encryptSecretSharesFn) ProcessElement(ctx context.Context, c reporttypes.RawReport, emit1 func(*pb.EncryptedPartialReportDpf), emit2 func(*pb.EncryptedPartialReportDpf)) error {
	fn.countReport.Inc(ctx, 1)

	encryptedReport1, encryptedReport2, err := GenerateEncryptedReports(c, fn.KeyBitSize, fn.PublicKeys1, fn.PublicKeys2, nil, fn.EncryptOutput)
	if err != nil {
		return err
	}

	emit1(encryptedReport1)
	emit2(encryptedReport2)
	return nil
}

// FormatEncryptedPartialReport serializes the EncryptedPartialReportDpf into a string.
func FormatEncryptedPartialReport(encrypted *pb.EncryptedPartialReportDpf) (string, error) {
	bEncrypted, err := proto.Marshal(encrypted)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bEncrypted), nil
}

// Since we store and read the data line by line through plain text files, the output is base64-encoded to avoid writing symbols in the proto wire-format that are interpreted as line breaks.
func formatPartialReportFn(encrypted *pb.EncryptedPartialReportDpf, emit func(string)) error {
	encryptedStr, err := FormatEncryptedPartialReport(encrypted)
	if err != nil {
		return err
	}
	emit(encryptedStr)
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
			PublicKeys1:   params.PublicKeys1,
			PublicKeys2:   params.PublicKeys2,
			KeyBitSize:    params.KeyBitSize,
			EncryptOutput: params.EncryptOutput,
		}, reports)
}

// GeneratePartialReportParams contains required parameters for generating partial reports.
type GeneratePartialReportParams struct {
	ConversionURI, PartialReportURI1, PartialReportURI2 string
	PublicKeys1, PublicKeys2                            []cryptoio.PublicKeyInfo
	KeyBitSize                                          int
	Shards                                              int64

	// EncryptOutput should only be used for integration test before HPKE is ready in Go Tink.
	EncryptOutput bool
}

// GeneratePartialReport splits the raw reports into two shares and encrypts them with public keys from corresponding helpers.
func GeneratePartialReport(scope beam.Scope, params *GeneratePartialReportParams) {
	scope = scope.Scope("GeneratePartialReports")

	allFiles := ioutils.AddStrInPath(params.ConversionURI, "*")
	lines := textio.Read(scope, allFiles)
	rawConversions := beam.ParDo(scope, &parseRawConversionFn{KeyBitSize: params.KeyBitSize}, lines)
	resharded := beam.Reshuffle(scope, rawConversions)

	partialReport1, partialReport2 := splitRawConversion(scope, resharded, params)
	writePartialReport(scope, partialReport1, params.PartialReportURI1, params.Shards)
	writePartialReport(scope, partialReport2, params.PartialReportURI2, params.Shards)
}

// PrefixNode represents a node in the tree that defines the conversion key prefix hierarchy.
//
// The node represents a segment of bits in the numeric conversion key, which records a certain type of information.
type PrefixNode struct {
	// A note for what type of information this node records. e.g. "campaignid", "geo", or anything contained in the conversion key.
	Class string
	// The value and length of the bit segment represented by this node.
	BitsValue uint128.Uint128
	BitsSize  uint64
	// The value and length of the prefix bits accumulated from the root to this node.
	PrefixValue uint128.Uint128
	PrefixSize  uint64
	Children    []*PrefixNode
}

// AddChildNode adds a node in the prefix tree.
func (pn *PrefixNode) AddChildNode(class string, bitsSize uint64, bitsValue uint128.Uint128) *PrefixNode {
	child := &PrefixNode{
		Class:    class,
		BitsSize: bitsSize, BitsValue: bitsValue,
		PrefixSize: bitsSize + pn.PrefixSize, PrefixValue: pn.PrefixValue.Lsh(uint(bitsSize)).Add(bitsValue),
	}
	pn.Children = append(pn.Children, child)
	return child
}

// CalculatePrefixes calculates the prefixes for each expansion hierarchy and prefix bit sizes.
//
// The prefixes will be used for expanding the DPF keys in different levels; and the prefix bit sizes will be used to determine DPF parameters for DPF key generation and expansion.
func CalculatePrefixes(root *PrefixNode) ([][]uint128.Uint128, []uint64) {
	var curNodes, nxtNodes []*PrefixNode
	curNodes = append(curNodes, root.Children...)

	var prefixes [][]uint128.Uint128
	// For the first level of expansion, the prefixes must be empty:
	// http://github.com/google/distributed_point_functions/dpf/distributed_point_function.h?l=86&rcl=368846188
	prefixes = append(prefixes, []uint128.Uint128{})
	var prefixBitSizes []uint64
	for len(curNodes) > 0 {
		var prefix []uint128.Uint128
		var prefixBitSize uint64
		nxtNodes = []*PrefixNode{}
		for _, node := range curNodes {
			prefix = append(prefix, node.PrefixValue)
			prefixBitSize = node.PrefixSize
			nxtNodes = append(nxtNodes, node.Children...)
		}
		prefixes = append(prefixes, prefix)
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
	var valueType = &dpfpb.ValueType{
		Type: &dpfpb.ValueType_Integer_{
			Integer: &dpfpb.ValueType_Integer{
				Bitsize: elementBitSizeSum,
			},
		},
	}
	for _, p := range prefixBitSizes {
		sumParams = append(sumParams, &dpfpb.DpfParameters{LogDomainSize: int32(p), ValueType: valueType})
	}
	sumParams = append(sumParams, &dpfpb.DpfParameters{LogDomainSize: logN, ValueType: valueType})
	return &pb.IncrementalDpfParameters{Params: sumParams}
}

func randUint64Bits(bitSize uint64) uint64 {
	return rand.Uint64() >> (64 - bitSize)
}

func randUint128(bitSize uint64) (uint128.Uint128, error) {
	if bitSize > 128 {
		return uint128.Zero, fmt.Errorf("expect bitSize < 128, got %d", bitSize)
	}
	if bitSize <= 64 {
		return uint128.From64(randUint64Bits(bitSize)), nil
	}
	return uint128.New(rand.Uint64(), randUint64Bits(bitSize-64)), nil
}

// CreateConversionIndex generates a random conversion ID that matches one of the prefixes described by the input, or does not have any of the prefixes.
func CreateConversionIndex(prefixes []uint128.Uint128, prefixBitSize, totalBitSize uint64, hasPrefix bool) (uint128.Uint128, error) {
	if prefixBitSize > totalBitSize {
		return uint128.Zero, fmt.Errorf("expect a prefix bit size no larger than total bit size %d, got %d", totalBitSize, prefixBitSize)
	}
	if len(prefixes) == 1<<prefixBitSize || len(prefixes) == 0 {
		if hasPrefix {
			return randUint128(totalBitSize)
		}
		return uint128.Zero, errors.New("unable to generate an index without any of the prefixes")
	}

	suffixBitSize := totalBitSize - prefixBitSize
	if hasPrefix {
		suffix, err := randUint128(suffixBitSize)
		if err != nil {
			return uint128.Zero, err
		}
		return prefixes[rand.Intn(len(prefixes))].Lsh(uint(suffixBitSize)).Add(suffix), nil
	}

	existing := make(map[uint128.Uint128]struct{})
	for _, p := range prefixes {
		existing[p] = struct{}{}
	}
	var (
		otherPrefix uint128.Uint128
		err         error
	)
	for {
		otherPrefix, err = randUint128(prefixBitSize)
		if err != nil {
			return uint128.Zero, err
		}
		if _, ok := existing[otherPrefix]; !ok {
			break
		}
	}
	suffix, err := randUint128(suffixBitSize)
	if err != nil {
		return uint128.Zero, err
	}
	return otherPrefix.Lsh(uint(suffixBitSize)).Or(suffix), nil
}

// ReadRawConversions reads conversions from a file. Each line of the file represents a conversion record.
func ReadRawConversions(ctx context.Context, conversionFile string, keyBitSize int) ([]reporttypes.RawReport, error) {
	lines, err := ioutils.ReadLines(ctx, conversionFile)
	if err != nil {
		return nil, err
	}

	var conversions []reporttypes.RawReport
	for _, l := range lines {
		conversion, err := ParseRawConversion(l, keyBitSize)
		if err != nil {
			return nil, err
		}
		conversions = append(conversions, conversion)
	}
	return conversions, nil
}
