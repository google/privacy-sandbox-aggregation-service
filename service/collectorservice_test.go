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

func TestCollectPayloads(t *testing.T) {
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

	got := make(map[string][]*pb.EncryptedPartialReportDpf)
	gotKey1, gotKey2, err := collectPayloads(report, got)
	if err != nil {
		t.Fatal(err)
	}

	wantKey1, wantKey2 := "helper1+helper2+1", "helper1+helper2+2"
	want := map[string][]*pb.EncryptedPartialReportDpf{
		wantKey1: {want1},
		wantKey2: {want2},
	}

	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("encrypted report mismatch (-want +got):\n%s", diff)
	}

	if wantKey1 != gotKey1 || wantKey2 != gotKey2 {
		t.Errorf("want keys %q and %q, got %q and %q", wantKey1, wantKey2, gotKey1, gotKey2)
	}
}
