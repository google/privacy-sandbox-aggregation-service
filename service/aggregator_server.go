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

// This binary hosts the aggregation service.
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
	"github.com/google/privacy-sandbox-aggregation-service/service/aggregatorservice"
	"github.com/google/privacy-sandbox-aggregation-service/service/query"
)

var (
	address = flag.String("address", ":8080", "Address of the server.")

	privateKeyParamsURI                  = flag.String("private_key_params_uri", "", "Input file that stores the required parameters to fetch the private keys.")
	dpfAggregatePartialReportBinary      = flag.String("dpf_aggregate_partial_report_binary", "/dpf_aggregate_partial_report_pipeline", "Binary for partial report aggregation with DPF protocol.")
	dpfAggregateReachPartialReportBinary = flag.String("dpf_aggregate_reach_partial_report_binary", "/dpf_aggregate_reach_partial_report_pipeline", "Binary for partial report aggregation for Reach.")
	workspaceURI                         = flag.String("workspace_uri", "", "The Private location to save the intermediate query states.")
	// The PubSub subscription should enable the retry policy with a exponential backoff delay.
	// Recommended retry policy: min_retry_delay=60s, max_retry_delay=600s.
	// The subscription should also have a dead-letter topic where messages will be forwarded after 10 failed delivery attemps.
	pubsubSubscription = flag.String("pubsub_subscription", "", "The PubSub subscription where to pull the request message. The value should be a fully qualified subscription URI.")
	pubsubTopic        = flag.String("pubsub_topic", "", "PubSub topic to send aggregation requests to. The value may be a fully qualified topic URI.")
	origin             = flag.String("origin", "", "Origin of the helper.")
	sharedDir          = flag.String("shared_dir", "", "Shared directory for the intermediate results, where other helper can read them.")

	pipelineRunner            = flag.String("pipeline_runner", "direct", "Runner for the Beam pipeline: direct or dataflow.")
	dataflowProject           = flag.String("dataflow_project", "", "GCP project of the Dataflow service.")
	dataflowRegion            = flag.String("dataflow_region", "", "Region of Dataflow workers.")
	dataflowZone              = flag.String("dataflow_zone", "", "Zone of Dataflow workers.")
	dataflowTempLocation      = flag.String("dataflow_temp_location", "", "TempLocation for the Dataflow pipeline.")
	dataflowStagingLocation   = flag.String("dataflow_staging_location", "", "StagingLocation for the Dataflow pipeline.")
	dataflowMaxNumWorkers     = flag.Int("dataflow_max_num_workers", 500, "Maximum number of Dataflow workers. If not specified, the maximum will be 1000 or depending on the quotas.")
	dataflowWorkerMachineType = flag.String("dataflow_worker_machine_type", "e2-standard-2", "Dataflow worker machine type.")
	dataflowServiceAccount    = flag.String("dataflow_service_account", "", "Service account that manages the workers.")

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
	log.Infof("Aggregation server %q listening on address %q", *origin, *address)
	log.Infof("Running server version: %v, build: %v\n", version, buildDate)
	log.Infof("PubSub subscription: %s\n", *pubsubSubscription)
	log.Infof("PubSub topic: %s\n", *pubsubTopic)
	log.Infof("Private Key Params URI: %s\n", *privateKeyParamsURI)
	log.Infof("Pipeline binary: %s\n", *dpfAggregatePartialReportBinary)
	log.Infof("Shared directory: %s\n", *sharedDir)

	sharedInfoHandler := &aggregatorservice.SharedInfoHandler{
		SharedInfo: &query.HelperSharedInfo{
			Origin:      *origin,
			SharedDir:   *sharedDir,
			PubSubTopic: *pubsubTopic,
		},
	}
	srv := http.Server{
		Addr:      *address,
		Handler:   sharedInfoHandler,
		TLSConfig: &tls.Config{},
	}

	ctx := context.Background()
	queryHandler := aggregatorservice.QueryHandler{
		ServerCfg: aggregatorservice.ServerCfg{
			PrivateKeyParamsURI:                  *privateKeyParamsURI,
			DpfAggregatePartialReportBinary:      *dpfAggregatePartialReportBinary,
			DpfAggregateReachPartialReportBinary: *dpfAggregateReachPartialReportBinary,
			WorkspaceURI:                         *workspaceURI,
		},
		PipelineRunner: *pipelineRunner,
		DataflowCfg: aggregatorservice.DataflowCfg{
			Project:             *dataflowProject,
			Region:              *dataflowRegion,
			Zone:                *dataflowZone,
			TempLocation:        *dataflowTempLocation,
			StagingLocation:     *dataflowStagingLocation,
			MaxNumWorkers:       *dataflowMaxNumWorkers,
			WorkerMachineType:   *dataflowWorkerMachineType,
			ServiceAccountEmail: *dataflowServiceAccount,
		},
		Origin:                    *origin,
		SharedDir:                 *sharedDir,
		RequestPubSubTopic:        *pubsubTopic,
		RequestPubsubSubscription: *pubsubSubscription,
	}

	if err := queryHandler.Setup(ctx); err != nil {
		log.Exit(err)
	}
	defer queryHandler.Close()

	// Create channel to listen for signals.
	signalChan := make(chan os.Signal, 1)
	// SIGINT handles Ctrl+C locally.
	// SIGTERM handles e.g. Cloud Run termination signal.
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	cctx, cancel := context.WithCancel(ctx)
	go func() {
		err := queryHandler.SetupPullRequests(cctx)
		if err != nil {
			log.Fatalf("Pull Subscription error: %v", err)
		}
	}()

	// Receive output from signalChan.
	sig := <-signalChan
	log.Infof("%s signal caught", sig)
	cancel()
}
