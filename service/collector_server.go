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

// This binary hosts the collector service.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	log "github.com/golang/glog"
	"github.com/google/privacy-sandbox-aggregation-service/service/collectorservice"
)

var (
	address   = flag.String("address", "", "Address of the server.")
	batchDir  = flag.String("batch_dir", "", "Directory that stores report batches.")
	batchSize = flag.Int("batch_size", 1000000, "Number of reports to be included in each batch file.")

	version string // set by linker -X
	build   string // set by linker -X
)

func main() {
	flag.Parse()

	buildDate := time.Unix(0, 0)
	if i, err := strconv.ParseInt(build, 10, 64); err != nil {
		log.Error(err)
	} else {
		buildDate = time.Unix(i, 0)
	}

	log.Info("- Debugging enabled - \n")
	log.Infof("Running collector server version: %v, build: %v\n", version, buildDate)
	log.Infof("Listening to %v", *address)
	log.Infof("Batch size %v, Batch Dir: %v", *batchSize, *batchDir)

	handler := collectorservice.NewHandler(context.Background(), *batchSize, *batchDir)
	srv := &http.Server{
		Addr:      *address,
		Handler:   handler.Handler(),
		TLSConfig: &tls.Config{},
	}

	// Create channel to listen for signals.
	signalChan := make(chan os.Signal, 1)
	// SIGINT handles Ctrl+C locally.
	// SIGTERM handles e.g. Cloud Run termination signal.
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Receive output from signalChan.
	sig := <-signalChan
	log.Infof("%s signal caught", sig)

	// Timeout if waiting for connections to return idle.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Gracefully shutdown the server by waiting on existing requests (except websockets).
	if err := srv.Shutdown(ctx); err != nil {
		log.Infof("server shutdown failed: %+v", err)
	}

	// Flush all remaining reports
	handler.Shutdown()

	log.Infof("server exited")
}
