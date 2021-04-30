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

// Package browsersimulator simulates the behavior of browser in splitting raw conversions into partial reports.
// Functions in this package are just used for simulating the browser in the experiments, which are not used in practice.
package browsersimulator

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"
	"github.com/pborman/uuid"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/secretshare"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*pb.StandardCiphertext)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*rawConversion)(nil)))
	beam.RegisterType(reflect.TypeOf((*encryptConversionFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseRawConversionFn)(nil)).Elem())
	beam.RegisterFunction(formatPartialReportFn)
}

// SplitIntoByteShares creates two shares from the original conversion key.
//
// The shares created for the same conversion key should be consistent, so we can recover it after the aggregation.
func SplitIntoByteShares(orig []byte) ([]byte, []byte, error) {
	n := len(orig)
	if n == 0 {
		return nil, nil, fmt.Errorf("cannot create shares for empty bytes")
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, nil, err
	}

	a, err := secretshare.XorBytes(orig, b)
	if err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

// splitIntoIntShares splits the original value into two random shares.
//
// According to the overflow behavior of uint32 (https://golang.org/ref/spec#Arithmetic_operators),
// the input number can be split into random shares, which can be recovered by summing them.
func splitIntoIntShares(orig uint16) (uint32, uint32, error) {
	var a uint32
	if err := binary.Read(rand.Reader, binary.LittleEndian, &a); err != nil {
		return 0, 0, err
	}
	return a, uint32(orig) - a, nil
}

// createRandomReportID creates a unique ID for a conversion report.
//
// This is the ephemeral ID used for rekeying the partial reports with reencrypted keys from the other helper.
func createRandomReportID() string {
	return uuid.New()
}

// Conversion data collected by the browsers.
type rawConversion struct {
	Key   string
	Value uint16
}

type parseRawConversionFn struct {
	rawConversionCounter beam.Counter
}

func (fn *parseRawConversionFn) Setup() {
	fn.rawConversionCounter = beam.NewCounter("aggregation-prototype", "raw-conversion-count")
}

func (fn *parseRawConversionFn) ProcessElement(ctx context.Context, line string, emit func(rawConversion)) error {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key := cols[0]
	value16, err := strconv.ParseUint(cols[1], 10, 16)
	if err != nil {
		return err
	}

	fn.rawConversionCounter.Inc(ctx, 1)
	emit(rawConversion{
		Key:   key,
		Value: uint16(value16),
	})
	return nil
}

// ServerPublicInfo contains public keys from the helper server.
type ServerPublicInfo struct {
	StandardPublicKey *pb.StandardPublicKey
	ElGamalPublicKey  *pb.ElGamalPublicKey
}

// GetPublicInfo reads the standard and ElGamal public keys from a given directory.
func GetPublicInfo(publicKeyDir string) (*ServerPublicInfo, error) {
	sPub, err := cryptoio.ReadStandardPublicKey(path.Join(publicKeyDir, cryptoio.DefaultStandardPublicKey))
	if err != nil {
		return nil, err
	}
	ePub, err := cryptoio.ReadElGamalPublicKey(path.Join(publicKeyDir, cryptoio.DefaultElgamalPublicKey))
	if err != nil {
		return nil, err
	}
	return &ServerPublicInfo{
		StandardPublicKey: sPub,
		ElGamalPublicKey:  ePub,
	}, nil
}

// The DoFn struct for encrypting the partial reports.
//
// All beam DoFn structs are suffixed with "Fn" to distinguish them from normal structs.
type encryptConversionFn struct {
	Helper1                *ServerPublicInfo
	Helper2                *ServerPublicInfo
	encryptedReportCounter beam.Counter
}

func (fn *encryptConversionFn) Setup() {
	fn.encryptedReportCounter = beam.NewCounter("aggregation-prototype", "encrypted-report-count")
}

// Apply standard public key encryption for the partial report.
//
// Since we store and read the data line by line through plain text files, the output is base64-encoded to avoid writing symbols in the proto wire-format that are interpreted as line breaks.
func encryptPartialReport(partialReport *pb.PartialReport, key *pb.StandardPublicKey) (*pb.StandardCiphertext, error) {
	bPartialReport, err := proto.Marshal(partialReport)
	if err != nil {
		return nil, err
	}
	return standardencrypt.Encrypt(bPartialReport, nil, key)
}

// Generate encrypted partial report from the key/value shares.
func genPartialReport(key string, keyShare []byte, valueShare uint32, destServer *ServerPublicInfo, otherServer *ServerPublicInfo) (*pb.StandardCiphertext, error) {
	encryptedKey, err := elgamalencrypt.Encrypt(key, otherServer.ElGamalPublicKey)
	if err != nil {
		return nil, err
	}
	return encryptPartialReport(
		&pb.PartialReport{
			EncryptedConversionKey: encryptedKey,
			ValueShare:             valueShare,
			KeyShare:               keyShare,
		},
		destServer.StandardPublicKey)
}

func (fn *encryptConversionFn) ProcessElement(ctx context.Context, c rawConversion, emit1 func(string, *pb.StandardCiphertext), emit2 func(string, *pb.StandardCiphertext)) error {
	keyShare1, keyShare2, err := SplitIntoByteShares([]byte(c.Key))
	if err != nil {
		return err
	}
	valueShare1, valueShare2, err := splitIntoIntShares(c.Value)
	if err != nil {
		return err
	}
	reportID := createRandomReportID()

	encryptedReport1, err := genPartialReport(c.Key, keyShare1, valueShare1, fn.Helper1, fn.Helper2)
	if err != nil {
		return err
	}
	fn.encryptedReportCounter.Inc(ctx, 1)
	emit1(reportID, encryptedReport1)

	encryptedReport2, err := genPartialReport(c.Key, keyShare2, valueShare2, fn.Helper2, fn.Helper1)
	if err != nil {
		return err
	}
	fn.encryptedReportCounter.Inc(ctx, 1)
	emit2(reportID, encryptedReport2)
	return nil
}

// TODO: cover the reading/writing functions with unit test.
func formatPartialReportFn(reportID string, encrypted *pb.StandardCiphertext, emit func(string)) error {
	bEncrypted, err := proto.Marshal(encrypted)
	if err != nil {
		return err
	}
	emit(fmt.Sprintf("%s,%s", reportID, base64.StdEncoding.EncodeToString(bEncrypted)))
	return nil
}

func writePartialReport(s beam.Scope, output beam.PCollection, outputTextName string, shards int64) {
	s = s.Scope("WriteEncryptedPartialReport")
	formattedOutput := beam.ParDo(s, formatPartialReportFn, output)
	ioutils.WriteNShardedFiles(s, outputTextName, shards, formattedOutput)
}

func splitRawConversion(s beam.Scope, lines beam.PCollection, helper1, helper2 *ServerPublicInfo) (beam.PCollection, beam.PCollection) {
	s = s.Scope("SplitRawConversion")

	// Encrypt conversions as PCollection<id, Encrypted(PartialReport)> for two helpers.
	return beam.ParDo2(s,
		&encryptConversionFn{
			Helper1: helper1,
			Helper2: helper2,
		}, lines)
}

// GeneratePartialReport splits the raw conversion records and encrypts partial reports for both helpers.
//
// The input conversionFile should contains rows with a string key and uint16 integer separated by a comma. There should be no header in the file.
func GeneratePartialReport(scope beam.Scope, conversionFile, partialReportFile1, partialReportFile2 string, helperInfo1, helperInfo2 *ServerPublicInfo, shards int64) {
	scope = scope.Scope("GeneratePartialReports")

	lines := textio.ReadSdf(scope, conversionFile)
	rawConversions := beam.ParDo(scope, &parseRawConversionFn{}, lines)
	// Reshuffle here to avoid fusing the file-reading and partial report generation together for better parallelization.
	resharded := beam.Reshuffle(scope, rawConversions)

	partialReport1, partialReport2 := splitRawConversion(scope, resharded, helperInfo1, helperInfo2)
	writePartialReport(scope, partialReport1, partialReportFile1, shards)
	writePartialReport(scope, partialReport2, partialReportFile2, shards)
}
