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
package dpfaggregator

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*decryptPartialReportFn)(nil)).Elem())
	beam.RegisterType(reflect.TypeOf((*parseEncryptedPartialReportFn)(nil)).Elem())
}

type parseEncryptedPartialReportFn struct {
	partialReportCounter beam.Counter
}

func (fn *parseEncryptedPartialReportFn) Setup() {
	fn.partialReportCounter = beam.NewCounter("aggregation-prototype", "partial-report-count")
}

func (fn *parseEncryptedPartialReportFn) ProcessElement(ctx context.Context, line string, emit func(*pb.StandardCiphertext)) error {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return err
	}

	ciphertext := &pb.StandardCiphertext{}
	if err := proto.Unmarshal(bsc, ciphertext); err != nil {
		return err
	}
	emit(ciphertext)
	return nil
}

func readPartialReport(scope beam.Scope, partialReportFile string) beam.PCollection {
	allFiles := ioutils.AddStrInPath(partialReportFile, "*")
	lines := textio.ReadSdf(scope, allFiles)
	return beam.ParDo(scope, &parseEncryptedPartialReportFn{}, lines)
}

type decryptPartialReportFn struct {
	StandardPrivateKey *pb.StandardPrivateKey
}

func (fn *decryptPartialReportFn) ProcessElement(encrypted *pb.StandardCiphertext, emit func(*pb.PartialReportDpf)) error {
	b, err := standardencrypt.Decrypt(encrypted, fn.StandardPrivateKey)
	if err != nil {
		return fmt.Errorf("decrypt failed for cipherText: %s", encrypted.String())
	}

	partialReport := &pb.PartialReportDpf{}
	if err := proto.Unmarshal(b, partialReport); err != nil {
		return err
	}
	emit(partialReport)
	return nil
}

// DecryptPartialReport decrypts every line in the input file with the helper private key, and gets the partial report.
func DecryptPartialReport(s beam.Scope, encryptedReport beam.PCollection, standardPrivateKey *pb.StandardPrivateKey) beam.PCollection {
	s = s.Scope("DecryptPartialReport")
	return beam.ParDo(s, &decryptPartialReportFn{StandardPrivateKey: standardPrivateKey}, encryptedReport)
}
