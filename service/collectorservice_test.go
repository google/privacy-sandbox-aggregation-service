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
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

func TestCollectPayloads(t *testing.T) {
	flag.Parse()

	contextInfo := []byte("shared_info")
	payload1, payload2 := []byte("payload1"), []byte("payload2")
	report := &reporttypes.AggregationReport{
		SharedInfo: contextInfo,
		AggregationServicePayloads: []*reporttypes.AggregationServicePayload{
			{Origin: "helper2", Payload: payload2},
			{Origin: "helper1", Payload: payload1},
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
		reportsCh: make(chan *reporttypes.AggregationReport),
	}
	ctx := context.Background()
	brw.start(ctx, brw.reportsCh)

	brw.reportsCh <- report
	close(brw.reportsCh)
	brw.wg.Wait()

	// Verify the right data was written to the batches
	want1 := &pb.EncryptedPartialReportDpf{EncryptedReport: &pb.StandardCiphertext{Data: payload1}, ContextInfo: contextInfo}
	want2 := &pb.EncryptedPartialReportDpf{EncryptedReport: &pb.StandardCiphertext{Data: payload2}, ContextInfo: contextInfo}

	filesWritten := false
	dir = dir + "/helper1+helper2"
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

func TestCollectPayloadsMultipleOriginCombos(t *testing.T) {
	flag.Parse()

	contextInfo := []byte("shared_info")
	payload1, payload2, payload3 := []byte("payload1"), []byte("payload2"), []byte("payload2")
	reporth1h2 := &reporttypes.AggregationReport{
		SharedInfo: contextInfo,
		AggregationServicePayloads: []*reporttypes.AggregationServicePayload{
			{Origin: "helper2", Payload: payload2},
			{Origin: "helper1", Payload: payload1},
		},
	}

	reporth1h3 := &reporttypes.AggregationReport{
		SharedInfo: contextInfo,
		AggregationServicePayloads: []*reporttypes.AggregationServicePayload{
			{Origin: "helper1", Payload: payload1},
			{Origin: "helper3", Payload: payload3},
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
		reportsCh: make(chan *reporttypes.AggregationReport),
	}
	ctx := context.Background()
	brw.start(ctx, brw.reportsCh)

	brw.reportsCh <- reporth1h2
	brw.reportsCh <- reporth1h3
	close(brw.reportsCh)
	brw.wg.Wait()

	// Verify the right data was written to the batches
	want1 := &pb.EncryptedPartialReportDpf{EncryptedReport: &pb.StandardCiphertext{Data: payload1}, ContextInfo: contextInfo}
	want2 := &pb.EncryptedPartialReportDpf{EncryptedReport: &pb.StandardCiphertext{Data: payload2}, ContextInfo: contextInfo}
	want3 := &pb.EncryptedPartialReportDpf{EncryptedReport: &pb.StandardCiphertext{Data: payload3}, ContextInfo: contextInfo}

	originCombos := map[string][]*pb.EncryptedPartialReportDpf{
		"helper1+helper2": {want1, want2},
		"helper1+helper3": {want1, want3},
	}

	matchedOriginCombo := map[string]bool{
		"helper1+helper2": false,
		"helper1+helper3": false,
	}

	filesWritten := false

	for key, expected := range originCombos {
		bdir := dir + "/" + key
		fileInfo, err := ioutil.ReadDir(bdir)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range fileInfo {
			filesWritten = true
			matchedFile := false
			partialReports, err := readFile(bdir, file.Name())
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(file.Name(), key) {
				matchedFile = true
				matchedOriginCombo[key] = true
				for _, pr := range partialReports {
					diff1 := cmp.Diff(expected[0], pr, protocmp.Transform())
					diff2 := cmp.Diff(expected[1], pr, protocmp.Transform())

					if diff1 != "" && diff2 != "" {
						t.Errorf("encrypted report mismatched both reports - 1 (-want +got):\n%s", diff1)
						t.Errorf("encrypted report mismatched both reports - 2 (-want +got):\n%s", diff2)
					}
				}
			}
			if !matchedFile {
				t.Errorf("No matching expected originCombo for file %s", file.Name())
			}
		}
	}

	if !filesWritten {
		t.Errorf("No batch files found!")
	}

	for key, found := range matchedOriginCombo {
		if !found {
			t.Errorf("No files for originCombo '%s' found", key)
		}
	}
}

func readFile(dir, filename string) ([]*pb.EncryptedPartialReportDpf, error) {
	file, err := os.Open(path.Join(dir, filename))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var batch []*pb.EncryptedPartialReportDpf
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

func readPartialReport(line string) (*pb.EncryptedPartialReportDpf, error) {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return nil, err
	}

	partialReport := &pb.EncryptedPartialReportDpf{}
	if err := proto.Unmarshal(bsc, partialReport); err != nil {
		return nil, err
	}

	return partialReport, nil
}
