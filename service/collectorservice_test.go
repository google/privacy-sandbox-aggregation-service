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
	"bufio"
	"context"
	"encoding/base64"
	"flag"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

func TestCollectPayloads(t *testing.T) {
	flag.Parse()

	contextInfo := "shared_info"
	payload1, payload2 := []byte("payload1"), []byte("payload2")
	report := &reporttypes.AggregatableReport{
		SharedInfo: contextInfo,
		AggregationServicePayloads: []*reporttypes.AggregationServicePayload{
			{Payload: base64.StdEncoding.EncodeToString(payload1)},
			{Payload: base64.StdEncoding.EncodeToString(payload2)},
		},
	}

	dir, err := ioutil.TempDir("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	brw := &bufferedReportWriter{
		batchSize: 1,
		batchDir:  dir,
		wg:        &sync.WaitGroup{},
		reportsCh: make(chan *reporttypes.AggregatableReport),
	}
	ctx := context.Background()
	brw.start(ctx, brw.reportsCh)

	brw.reportsCh <- report
	close(brw.reportsCh)
	brw.wg.Wait()

	// Verify the right data was written to the batches
	want1 := &pb.AggregatablePayload{Payload: &pb.StandardCiphertext{Data: payload1}, SharedInfo: contextInfo}
	want2 := &pb.AggregatablePayload{Payload: &pb.StandardCiphertext{Data: payload2}, SharedInfo: contextInfo}

	filesWritten := false
	dir = dir + "/mpc"
	fileInfo, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range fileInfo {
		filesWritten = true
		partialReports, err := readFile(dir, file.Name())
		if err != nil {
			t.Fatal(err)
		}

		for _, pr := range partialReports {
			diff1 := cmp.Diff(want1, pr, protocmp.Transform())
			diff2 := cmp.Diff(want2, pr, protocmp.Transform())

			if diff1 != "" && diff2 != "" {
				t.Errorf("encrypted report mismatched both reports - 1 (-want +got):\n%s", diff1)
				t.Errorf("encrypted report mismatched both reports - 2 (-want +got):\n%s", diff2)
				cmp.Diff(want2, pr, protocmp.Transform())
			}
		}
	}

	if !filesWritten {
		t.Errorf("No batch files found!")
	}
}

func readFile(dir, filename string) ([]*pb.AggregatablePayload, error) {
	file, err := os.Open(path.Join(dir, filename))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var batch []*pb.AggregatablePayload
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		partialReport, err := readPartialReport(scanner.Text())
		if err != nil {
			return nil, err
		}
		batch = append(batch, partialReport)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return batch, nil
}

func readPartialReport(line string) (*pb.AggregatablePayload, error) {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return nil, err
	}

	partialReport := &pb.AggregatablePayload{}
	if err := proto.Unmarshal(bsc, partialReport); err != nil {
		return nil, err
	}

	return partialReport, nil
}
