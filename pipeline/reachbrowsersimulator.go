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

// Package reachbrowsersimulator simulates the browser behavior under the DPF protocol.
package reachbrowsersimulator

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
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/incrementaldpf"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.EncryptedPartialReportDpf)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*encryptSecretSharesFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawRecordFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*RawRecord)(nil)))
	beam.RegisterFunction(formatPartialReportFn)
}

// RawRecord represents a raw record from the browser. For the DPF protocol the record key is an integer in a known domain.
type RawRecord struct {
	Campaign   uint64
	Person     uint64
	LLRegister uint64
	Slice      string
}

// parseRawRecordFn parses each line in the raw conversion file in the format: bucket ID, value.
type parseRawRecordFn struct {
	KeyBitSize      int32
	countConversion beam.Counter
}

func (fn *parseRawRecordFn) Setup(ctx context.Context) {
	fn.countConversion = beam.NewCounter("aggregation", "parserawConversionFn_conversion_count")
}

func parseRawRecord(line string, keyBitSize int32) (RawRecord, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 4; got != want {
		return RawRecord{}, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	campaign, err := strconv.ParseUint(cols[0], 10, 64)
	if err != nil {
		return RawRecord{}, err
	}

	person, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return RawRecord{}, err
	}

	llRegister, err := strconv.ParseUint(cols[2], 10, int(keyBitSize))
	if err != nil {
		return RawRecord{}, err
	}

	return RawRecord{
		Campaign:   campaign,
		Person:     person,
		LLRegister: llRegister,
		Slice:      cols[3],
	}, nil
}

func (fn *parseRawRecordFn) ProcessElement(ctx context.Context, line string, emit func(RawRecord)) error {
	conversion, err := parseRawRecord(line, fn.KeyBitSize)
	if err != nil {
		return err
	}

	fn.countConversion.Inc(ctx, 1)
	emit(conversion)
	return nil
}

type encryptSecretSharesFn struct {
	PublicKeys1, PublicKeys2 []cryptoio.PublicKeyInfo
	KeyBitSize               int32

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

func encryptPartialReport(partialReport *pb.PartialReportDpf, keys []cryptoio.PublicKeyInfo, contextInfo []byte) (*pb.EncryptedPartialReportDpf, error) {
	bDpfKey, err := proto.Marshal(partialReport.SumKey)
	if err != nil {
		return nil, err
	}

	payload := dpfaggregator.Payload{
		Operation: "reach-experiment",
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
	encrypted, err := standardencrypt.Encrypt(bPayload, contextInfo, key)
	if err != nil {
		return nil, err
	}
	return &pb.EncryptedPartialReportDpf{EncryptedReport: encrypted, ContextInfo: contextInfo, KeyId: keyID}, nil
}

func (fn *encryptSecretSharesFn) Setup(ctx context.Context) {
	fn.countReport = beam.NewCounter("aggregation", "encryptSecretSharesFn_report_count")
}

func dataToTuple(data RawRecord) *incrementaldpf.ReachTuple {
	r := rand.Uint64()
	q := rand.Uint64()

	return &incrementaldpf.ReachTuple{
		C:  1,
		Rf: r * data.Person,
		R:  r,
		Qf: q * data.Person,
		Q:  q,
	}
}

// GenerateEncryptedReports splits a conversion record into DPF keys, and encrypts the partial reports.
func GenerateEncryptedReports(data RawRecord, keyBitSize int32, publicKeys1, publicKeys2 []cryptoio.PublicKeyInfo, contextInfo []byte) (*pb.EncryptedPartialReportDpf, *pb.EncryptedPartialReportDpf, error) {
	params := incrementaldpf.CreateReachUint64TupleDpfParameters(keyBitSize)

	keyDpfSum1, keyDpfSum2, err := incrementaldpf.GenerateReachTupleKeys(params, data.LLRegister, dataToTuple(data))
	if err != nil {
		return nil, nil, err
	}

	encryptedReport1, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey: keyDpfSum1,
	}, publicKeys1, contextInfo)
	if err != nil {
		return nil, nil, err
	}

	encryptedReport2, err := encryptPartialReport(&pb.PartialReportDpf{
		SumKey: keyDpfSum2,
	}, publicKeys2, contextInfo)

	if err != nil {
		return nil, nil, err
	}

	return encryptedReport1, encryptedReport2, nil
}

func (fn *encryptSecretSharesFn) ProcessElement(ctx context.Context, c RawRecord, emit1 func(*pb.EncryptedPartialReportDpf), emit2 func(*pb.EncryptedPartialReportDpf)) error {
	fn.countReport.Inc(ctx, 1)

	encryptedReport1, encryptedReport2, err := GenerateEncryptedReports(c, fn.KeyBitSize, fn.PublicKeys1, fn.PublicKeys2, nil)
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
			PublicKeys1: params.PublicKeys1,
			PublicKeys2: params.PublicKeys2,
			KeyBitSize:  params.KeyBitSize,
		}, reports)
}

// GeneratePartialReportParams contains required parameters for generating partial reports.
type GeneratePartialReportParams struct {
	ReachFile, PartialReportFile1, PartialReportFile2 string
	PublicKeys1, PublicKeys2                          []cryptoio.PublicKeyInfo
	KeyBitSize                                        int32
	Shards                                            int64
}

// GeneratePartialReport splits the raw reports into two shares and encrypts them with public keys from corresponding helpers.
func GeneratePartialReport(scope beam.Scope, params *GeneratePartialReportParams) {
	scope = scope.Scope("GeneratePartialReports")

	allFiles := ioutils.AddStrInPath(params.ReachFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	records := beam.ParDo(scope, &parseRawRecordFn{KeyBitSize: params.KeyBitSize}, lines)
	resharded := beam.Reshuffle(scope, records)

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

// ReadRawRecords reads conversions from a file. Each line of the file represents a conversion record.
func ReadRawRecords(ctx context.Context, conversionFile string, keyBitSize int32) ([]RawRecord, error) {
	lines, err := ioutils.ReadLines(ctx, conversionFile)
	if err != nil {
		return nil, err
	}

	var records []RawRecord
	for _, l := range lines {
		record, err := parseRawRecord(l, keyBitSize)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
