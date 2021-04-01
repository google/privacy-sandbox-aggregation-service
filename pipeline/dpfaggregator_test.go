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

package dpfaggregator

import (
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

type standardEncryptFn struct {
	PublicKey *pb.StandardPublicKey
}

func (fn *standardEncryptFn) ProcessElement(report *pb.PartialReportDpf, emit func(*pb.StandardCiphertext)) error {
	b, err := proto.Marshal(report)
	if err != nil {
		return err
	}
	result, err := standardencrypt.Encrypt(b, fn.PublicKey)
	if err != nil {
		return err
	}
	emit(result)
	return nil
}

func TestDecryptPartialReport(t *testing.T) {
	priv, pub, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	reports := []*pb.PartialReportDpf{
		{
			SumKey:   &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 2, Low: 1}},
			CountKey: &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 3, Low: 2}},
		},
		{
			SumKey:   &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 4, Low: 3}},
			CountKey: &dpfpb.DpfKey{Seed: &dpfpb.Block{High: 5, Low: 4}},
		},
	}

	pipeline, scope := beam.NewPipelineWithRoot()

	wantReports := beam.CreateList(scope, reports)
	encryptedReports := beam.ParDo(scope, &standardEncryptFn{PublicKey: pub}, wantReports)
	getReports := DecryptPartialReport(scope, encryptedReports, priv)

	passert.Equals(scope, getReports, wantReports)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}
