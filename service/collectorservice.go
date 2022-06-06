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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/sync/errgroup"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

const (
	/* 20% reportsChannel buffer to allow for reports being processed while writing batches but
	limiting outstanding reports to cap memory consumption */
	reportsChannelBufferFactor = 0.2

	// Supported URL paths.
	reportPath      = "/.well-known/attribution-reporting/report-aggregate-attribution"
	debugReportPath = "/.well-known/attribution-reporting/debug/report-aggregate-attribution"
)

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
		reportsCh: make(chan *reporttypes.AggregatableReport, int(float64(batchSize)*reportsChannelBufferFactor)),
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
	if req.Method == "GET" {
		log.Info("GET Request received.")
		w.WriteHeader(http.StatusOK)
		return
	}

	if req.URL.Path != reportPath && req.URL.Path != debugReportPath {
		errMsg := "Unsupported path"
		http.Error(w, errMsg, http.StatusNotFound)
		log.Error(errMsg)
	}

	report := &reporttypes.AggregatableReport{}

	buf := new(bytes.Buffer)
	buf.ReadFrom(req.Body)
	if err := json.Unmarshal(buf.Bytes(), report); err != nil {
		errMsg := "Failed in decoding aggregation report"
		http.Error(w, errMsg, http.StatusBadRequest)
		log.Error(errMsg, err)
		return
	}

	if err := report.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error(err)
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
	reportsCh  chan *reporttypes.AggregatableReport
}

func (brw *bufferedReportWriter) start(ctx context.Context, reportsCh <-chan *reporttypes.AggregatableReport) {
	log.Infof("Starting buffered report writer with %v batch size", brw.batchSize)

	batches := make(map[string]map[string][]string)
	brw.wg.Add(1)
	go func() {
		for report := range reportsCh {
			protocol, err := report.GetProtocol()
			if err != nil {
				log.Error(err)
				continue
			}
			if !report.IsDebugReport() {
				tempMap, err := report.GetSerializedEncryptedRecords()
				if err != nil {
					log.Error(err)
					continue
				}
				batchKey := protocol
				if batches[batchKey] == nil {
					batches[batchKey] = make(map[string][]string)
				}
				isBatchFull := false
				for index, payload := range tempMap {
					batches[batchKey][index] = append(batches[batchKey][index], payload)
					isBatchFull = len(batches[batchKey][index]) == brw.batchSize
				}
				if isBatchFull {
					brw.writeBatchKeyBatches(ctx, batchKey, batches[batchKey])
					batches[batchKey] = make(map[string][]string)
				}
			} else {
				// For debug reports, the encrypted payloads and cleartext payloads are both collected.
				// The payloads are only added to the batches when they are all converted successfully,
				// which ensures the "debug" and "cleartext" batches contain the same data.
				tempDebugMap, err := report.GetSerializedEncryptedRecords()
				if err != nil {
					log.Error(err)
					continue
				}
				tempClearTextMap, err := report.GetSerializedCleartextRecords()
				if err != nil {
					log.Error(err)
					continue
				}
				debugBatchKey := protocol + "-debug-encrypted"
				cleartextBatchKey := protocol + "-debug-cleartext"
				if batches[debugBatchKey] == nil {
					batches[debugBatchKey] = make(map[string][]string)
					batches[cleartextBatchKey] = make(map[string][]string)
				}
				isBatchFull := false
				for index := range tempDebugMap {
					batches[debugBatchKey][index] = append(batches[debugBatchKey][index], tempDebugMap[index])
					batches[cleartextBatchKey][index] = append(batches[cleartextBatchKey][index], tempClearTextMap[index])
					isBatchFull = len(batches[debugBatchKey][index]) == brw.batchSize
				}
				if isBatchFull {
					brw.writeBatchKeyBatches(ctx, debugBatchKey, batches[debugBatchKey])
					brw.writeBatchKeyBatches(ctx, cleartextBatchKey, batches[cleartextBatchKey])
					batches[debugBatchKey] = make(map[string][]string)
					batches[cleartextBatchKey] = make(map[string][]string)
				}
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
	for index, encryptedReports := range reports {
		index, encryptedReports := index, encryptedReports // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			if len(encryptedReports) > 0 {
				batchedReportsURI := utils.JoinPath(brw.batchDir, fmt.Sprintf("%s/%s+%s+%s", batchKey, batchKey, index, timestamp))
				log.Infof("Writing %v records in batch for %v to: %v", len(encryptedReports), index, batchedReportsURI)
				return utils.WriteLines(ctx, encryptedReports, batchedReportsURI)
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
