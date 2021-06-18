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

	log "github.com/golang/glog"
	"github.com/google/privacy-sandbox-aggregation-service/service/collectorservice"
)

var (
	address       = flag.String("address", "", "Address of the server.")
	helperOrigin1 = flag.String("helper_origin1", "", "Origin of helper1.")
	helperOrigin2 = flag.String("helper_origin2", "", "Origin of helper2.")
	batchDir1     = flag.String("batch_dir1", "", "Directory that stores report batches for helper1.")
	batchDir2     = flag.String("batch_dir2", "", "Directory that stores report batches for helper2.")
	batchSize     = flag.Int("batch_size", 1000000, "Number of reports to be included in each batch file.")
)

func main() {
	flag.Parse()

	srv := &http.Server{
		Addr: *address,
		Handler: &collectorservice.CollectorHandler{
			HelperOrigin1: *helperOrigin1,
			HelperOrigin2: *helperOrigin2,
			BatchDir1:     *batchDir1,
			BatchDir2:     *batchDir2,
			BatchSize:     *batchSize,
		},
		TLSConfig: &tls.Config{},
	}
	log.Exit(srv.ListenAndServe())
}
