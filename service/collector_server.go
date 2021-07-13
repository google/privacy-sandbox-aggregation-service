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

package main

import (
	"crypto/tls"
	"flag"
	"net/http"
	"strconv"
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

	srv := &http.Server{
		Addr:      *address,
		Handler:   collectorservice.NewHandler(*batchSize, *batchDir),
		TLSConfig: &tls.Config{},
	}
	log.Exit(srv.ListenAndServe())
}
