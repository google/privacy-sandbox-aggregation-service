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

// Package conversion contains functions for exponentiation on conversion keys.
package conversion

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*decryptPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*exponentiateKeyFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*rekeyByAggregationIDFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.PartialReport)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*pb.ElGamalCiphertext)(nil)).Elem())

	beam.RegisterFunction(parseExponentiatedKeyFn)
	beam.RegisterFunction(formatExponentiatedKeyFn)
}

type decryptPartialReportFn struct {
	StandardPrivateKey *pb.StandardPrivateKey
}

// Decrypt the partial reports sent to the helper.
//
// Each line contains a report ID and encrypted partial report (base64-encoded) separated by a comma.
func (fn *decryptPartialReportFn) ProcessElement(line string, emit func(string, *pb.PartialReport)) error {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	reportID := cols[0]
	marshalledBytes, err := base64.StdEncoding.DecodeString(cols[1])
	if err != nil {
		return err
	}

	ciphertext := &pb.StandardCiphertext{}
	if err := proto.Unmarshal(marshalledBytes, ciphertext); err != nil {
		return err
	}

	// Decrypt to get the PartialReport.
	b, err := standardencrypt.Decrypt(ciphertext, fn.StandardPrivateKey)
	if err != nil {
		return fmt.Errorf("decrypt failed for cipherText: %s", ciphertext.String())
	}

	partialReport := &pb.PartialReport{}
	if err := proto.Unmarshal(b, partialReport); err != nil {
		return err
	}
	emit(reportID, partialReport)

	return nil
}

// DecryptPartialReport decrypts the input data to get the PCollection<reportId, *pb.PartialReport>
func DecryptPartialReport(s beam.Scope, lines beam.PCollection, standardPrivateKey *pb.StandardPrivateKey) beam.PCollection {
	s = s.Scope("DecryptPartialReport")
	return beam.ParDo(s, &decryptPartialReportFn{StandardPrivateKey: standardPrivateKey}, lines)
}

type exponentiateKeyFn struct {
	// The index of the exponentiation.
	Secret           string
	ElGamalPublicKey *pb.ElGamalPublicKey
}

func (fn *exponentiateKeyFn) ProcessElement(reportID string, pr *pb.PartialReport, emit func(string, *pb.ElGamalCiphertext)) error {
	exponentiated, err := elgamalencrypt.ExponentiateOnCiphertext(pr.EncryptedConversionKey, fn.ElGamalPublicKey, fn.Secret)
	if err != nil {
		return err
	}
	emit(reportID, exponentiated)
	return nil
}

func formatExponentiatedKeyFn(reportID string, encryptedKey *pb.ElGamalCiphertext, emit func(string)) error {
	b, err := proto.Marshal(encryptedKey)
	if err != nil {
		return err
	}
	emit(fmt.Sprintf("%s,%s", reportID, base64.StdEncoding.EncodeToString(b)))
	return nil
}

func writeExponentiatedKey(s beam.Scope, col beam.PCollection, outputName string) {
	s = s.Scope("WriteExponentiatedKey")
	formattedOutput := beam.ParDo(s, formatExponentiatedKeyFn, col)
	textio.Write(s, outputName, formattedOutput)
}

// exponentiateKey outputs a PCollection<reportID, *pb.ElGamalCiphertext> for the other helper.
func exponentiateKey(s beam.Scope, col beam.PCollection, secret string, publicKey *pb.ElGamalPublicKey) beam.PCollection {
	s = s.Scope("ExponentiateKey")
	return beam.ParDo(s, &exponentiateKeyFn{Secret: secret, ElGamalPublicKey: publicKey}, col)
}

// ServerPrivateInfo contains the private keys and secret from the helper server.
type ServerPrivateInfo struct {
	StandardPrivateKey *pb.StandardPrivateKey
	ElGamalPrivateKey  *pb.ElGamalPrivateKey
	Secret             string
}

// GetPrivateInfo reads the standard and ElGamal private keys together with the ElGamal secret from a given directory.
func GetPrivateInfo(privateKeyDir string) (*ServerPrivateInfo, error) {
	sPriv, err := cryptoio.ReadStandardPrivateKey(privateKeyDir)
	if err != nil {
		return nil, err
	}
	ePriv, err := cryptoio.ReadElGamalPrivateKey(privateKeyDir)
	if err != nil {
		return nil, err
	}
	secret, err := cryptoio.ReadElGamalSecret(privateKeyDir)
	if err != nil {
		return nil, err
	}
	return &ServerPrivateInfo{
		StandardPrivateKey: sPriv,
		ElGamalPrivateKey:  ePriv,
		Secret:             secret,
	}, nil
}

// ExponentiateConversionKey applies the exponential operation on the conversion keys with a secret.
func ExponentiateConversionKey(scope beam.Scope, partialReportFile, exponentiatedKeyFile string, helperInfo *ServerPrivateInfo, otherPublicKey *pb.ElGamalPublicKey) {
	scope = scope.Scope("ExponentiateConversionKey")

	lines := textio.Read(scope, partialReportFile)
	resharded := beam.Reshuffle(scope, lines)

	partialReport := DecryptPartialReport(scope, resharded, helperInfo.StandardPrivateKey)
	idKeys := exponentiateKey(scope, partialReport, helperInfo.Secret, otherPublicKey)

	writeExponentiatedKey(scope, idKeys, exponentiatedKeyFile)
}

func parseExponentiatedKeyFn(line string, emit func(string, *pb.ElGamalCiphertext)) error {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	reportID := cols[0]
	b, err := base64.StdEncoding.DecodeString(cols[1])
	if err != nil {
		return err
	}

	exponentiatedKey := &pb.ElGamalCiphertext{}
	if err := proto.Unmarshal(b, exponentiatedKey); err != nil {
		return err
	}
	emit(reportID, exponentiatedKey)
	return nil
}

// ReadExponentiatedKeys reads the exponentiated conversion keys from the other helper.
func ReadExponentiatedKeys(s beam.Scope, inputName string) beam.PCollection {
	s = s.Scope("ReadExponentiatedKey")
	lines := textio.Read(s, inputName)
	return beam.ParDo(s, parseExponentiatedKeyFn, lines)
}

// AggData contains columns for aggregating partial reports.
type AggData struct {
	ReportID string
	// The key for COUNT and SUM aggregation.
	AggID string
	// The private aggregation package will convert the integer value types into int64:
	// http://google3/third_party/differential_privacy/go/plume/pbeam/sum.go?l=106&rcl=336106761
	// The value shares are converted from uint32 to int64 here to make this explicit.
	ValueShare int64
}

// IDKeyShare contains the corresponding report ID for the key share, which will be used to decide which key share to keep in the aggregation.
type IDKeyShare struct {
	ReportID string
	KeyShare []byte
}

type rekeyByAggregationIDFn struct {
	ElGamalPrivateKey *pb.ElGamalPrivateKey
	Secret            string
}

// Join the exponentiated key from the other helper with the partial report using the report ID, and calculate the aggregation IDs for the key/value shares.
func (fn *rekeyByAggregationIDFn) ProcessElement(id string, encryptedKeyIter func(**pb.ElGamalCiphertext) bool, partialReportIter func(**pb.PartialReport) bool, emitIDKey func(string, IDKeyShare), emitAggData func(AggData)) error {
	var exponentiatedKey *pb.ElGamalCiphertext
	if !encryptedKeyIter(&exponentiatedKey) {
		return fmt.Errorf("no matched exponentiated key")
	}

	var partialReport *pb.PartialReport
	if !partialReportIter(&partialReport) {
		return fmt.Errorf("no matched partial report")
	}

	decryptedKey, err := elgamalencrypt.Decrypt(exponentiatedKey, fn.ElGamalPrivateKey)
	if err != nil {
		return err
	}
	aggID, err := elgamalencrypt.ExponentiateOnECPointStr(decryptedKey, fn.Secret)
	if err != nil {
		return err
	}

	emitIDKey(aggID, IDKeyShare{
		ReportID: id,
		KeyShare: partialReport.KeyShare,
	})

	emitAggData(AggData{
		ReportID:   id,
		AggID:      aggID,
		ValueShare: int64(partialReport.ValueShare),
	})

	return nil
}

// RekeyByAggregationID outputs PCollection<AggID, IDKeyShare> and PCollection<AggData>.
//
// The externalKey is the PCollection<ReportID, ExponentiatedKey> calculated by the other helper, and report is a PCollection<ReportID, PartialReport>.
func RekeyByAggregationID(s beam.Scope, externalKey, report beam.PCollection, privateKey *pb.ElGamalPrivateKey, secret string) (beam.PCollection, beam.PCollection) {
	s = s.Scope("RekeyByAggregationID")
	joined := beam.CoGroupByKey(s, externalKey, report)
	return beam.ParDo2(s, &rekeyByAggregationIDFn{
		ElGamalPrivateKey: privateKey,
		Secret:            secret,
	}, joined)
}