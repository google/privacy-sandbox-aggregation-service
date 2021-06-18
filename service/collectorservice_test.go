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

package collectorservice

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func TestMatchPayloads(t *testing.T) {
	helper1, helper2 := "helper1", "helper2"
	contextInfo := []byte("shared_info")
	payload1, payload2 := []byte("payload1"), []byte("payload2")
	report := &AggregationReport{
		SharedInfo: contextInfo,
		Payloads: []*AggregationServicePayload{
			{Origin: "helper2", Payload: payload2},
			{Origin: "helper1", Payload: payload1},
		},
	}
	want1 := &pb.EncryptedPartialReportDpf{EncryptedReport: &pb.StandardCiphertext{Data: payload1}, ContextInfo: contextInfo}
	want2 := &pb.EncryptedPartialReportDpf{EncryptedReport: &pb.StandardCiphertext{Data: payload2}, ContextInfo: contextInfo}

	got1, got2, err := matchPayloads(report, helper1, helper2)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want1, got1, protocmp.Transform()); diff != "" {
		t.Errorf("encrypted report mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want2, got2, protocmp.Transform()); diff != "" {
		t.Errorf("encrypted report mismatch (-want +got):\n%s", diff)
	}
}
