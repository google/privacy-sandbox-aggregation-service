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

// Package collectorservice contains the functions needed for the HTTP service which collects
// reports sent by the browsers.
package collectorservice

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/sync/errgroup"
	"github.com/ugorji/go/codec"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfdataconverter"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reporttypes"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

/* 20% reportsChannel buffer to allow for reports being processed while writing batches but
limiting outstanding reports to cap memory consumption */
const reportsChannelBufferFactor = 0.2

// CollectorHandler handles the HTTPS requests with incoming reports.
//
// The server keeps receiving reports from the browsers and tracks the number of reports per pair
// of helper origins. When the report number reaches the predefined batch size, the reports are
// written into two files, which will be the input of the aggregation service for the corresponding helpers.
type CollectorHandler struct {
	bufferedReportWriter bufferedReportWriter
}

// NewHandler creates a new CollectorHandler with initialized values
func NewHandler(ctx context.Context, batchSize int, batchDir string) *CollectorHandler {
	brw := bufferedReportWriter{
		batchSize: batchSize,
		batchDir:  batchDir,
		wg:        &sync.WaitGroup{},
		reportsCh: make(chan *reporttypes.AggregationReport, int(float64(batchSize)*reportsChannelBufferFactor)),
	}
	brw.start(ctx, brw.reportsCh)

	return &CollectorHandler{
		bufferedReportWriter: brw,
	}
}

// Handler helper function to get Handler for http.Server
func (h *CollectorHandler) Handler() http.Handler {
	return h
}

func (h *CollectorHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	report := &reporttypes.AggregationReport{}
	if err := codec.NewDecoder(req.Body, &codec.CborHandle{}).Decode(report); err != nil {
		errMsg := "Failed in decoding CBOR message"
		http.Error(w, errMsg, http.StatusBadRequest)
		log.Error(errMsg, err)
		return
	}

	if got, want := len(report.AggregationServicePayloads), 2; got != want {
		errMsg := fmt.Sprintf("expected %d payloads, got %d", want, got)
		http.Error(w, errMsg, http.StatusBadRequest)
		log.Error(errMsg)
	}

	origin1, origin2 := report.AggregationServicePayloads[0].Origin, report.AggregationServicePayloads[1].Origin
	if origin1 == origin2 {
		errMsg := fmt.Sprintf("secret shares sending to the same helper %q", origin1)
		http.Error(w, errMsg, http.StatusBadRequest)
		log.Error(errMsg)
	}

	h.bufferedReportWriter.reportsCh <- report
}

// Shutdown function used in http.Server.RegisterOnShutdown to close channel and flush
// remaining reports
func (h *CollectorHandler) Shutdown() {
	close(h.bufferedReportWriter.reportsCh)
	h.bufferedReportWriter.wg.Wait()
}

type bufferedReportWriter struct {
	batchSize  int
	bufferSize int
	batchDir   string
	wg         *sync.WaitGroup
	reportsCh  chan *reporttypes.AggregationReport
}

func (brw *bufferedReportWriter) start(ctx context.Context, reportsCh <-chan *reporttypes.AggregationReport) {
	log.Infof("Starting buffered report writer with %v batch size", brw.batchSize)

	batches := make(map[string]map[string][]string)
	brw.wg.Add(1)
	go func() {
		for report := range reportsCh {
			origin1, origin2 := report.AggregationServicePayloads[0].Origin, report.AggregationServicePayloads[1].Origin
			if origin1 > origin2 {
				origin1, origin2 = origin2, origin1
			}
			batchKey := fmt.Sprintf("%s+%s", origin1, origin2)
			if batches[batchKey] == nil {
				batches[batchKey] = make(map[string][]string)
			}
			// Use temp map for catching if any payload fails to be processed
			tempMap := make(map[string]string)
			itemsCount := 0
			for _, payload := range report.AggregationServicePayloads {
				encryptedPayload, err := dpfdataconverter.FormatEncryptedPartialReport(&pb.EncryptedPartialReportDpf{
					EncryptedReport: &pb.StandardCiphertext{Data: payload.Payload},
					ContextInfo:     report.SharedInfo,
					KeyId:           payload.KeyID,
				})
				if err != nil {
					log.Error(err)
					break
				}
				tempMap[payload.Origin] = encryptedPayload
				itemsCount++
			}
			// Skip shares where not all payloads could be processed
			if len(report.AggregationServicePayloads) != itemsCount {
				log.Errorf("Error during processing of payload")
				continue
			}

			for origin, encryptedPayload := range tempMap {
				batches[batchKey][origin] = append(batches[batchKey][origin], encryptedPayload)
			}

			// TODO: harden against batchKey attacks
			if len(batches[batchKey][origin1]) == brw.batchSize {
				brw.writeBatchKeyBatches(ctx, batchKey, batches[batchKey])
				// reset batchKey map
				batches[batchKey] = make(map[string][]string)
			}
		}
		log.Info("Buffered Report Writer channel closed, flushing remaining reports...")
		for batchKey, reports := range batches {
			brw.writeBatchKeyBatches(ctx, batchKey, reports)
		}
		brw.wg.Done()
	}()
}

func (brw *bufferedReportWriter) writeBatchKeyBatches(ctx context.Context, batchKey string, reports map[string][]string) {
	start := time.Now()
	timestamp := start.Format(time.RFC3339Nano)
	g, ctx := errgroup.WithContext(ctx)
	for origin, encryptedReports := range reports {
		origin, encryptedReports := origin, encryptedReports // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			if len(encryptedReports) > 0 {
				batchedReportsURI := ioutils.JoinPath(brw.batchDir, fmt.Sprintf("%s/%s+%s+%s", batchKey, batchKey, origin, timestamp))
				log.Infof("Writing %v records in batch for %v to: %v", len(encryptedReports), origin, batchedReportsURI)
				return ioutils.WriteLines(ctx, encryptedReports, batchedReportsURI)
			}
			log.Infof("Empty batch, nothing to write!")
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Error(err)
	}
	log.Infof("Writing batches at %s took %s.", timestamp, time.Now().Sub(start))
}
