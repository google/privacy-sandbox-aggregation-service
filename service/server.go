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
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"

	log "github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"github.com/google/privacy-sandbox-aggregation-service/service/service"

	pb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

var (
	port = flag.Int("port", 3389, "Port for the server.")

	keyDir                       = flag.String("key_dir", "", "Directory for the private keys and secrets used by the PRF protocol.")
	otherHelperInfoDir           = flag.String("other_helper_info_dir", "", "Directory storing information of the other helper.")
	reencryptConversionKeyBinary = flag.String("reencrypt_conversion_key_binary", "", "Binary for conversion key reencryption.")
	aggregatePartialReportBinary = flag.String("aggregate_partial_report_binary", "", "Binary for partial report aggregation.")

	privateKeyParamsURI             = flag.String("private_key_params_uri", "", "Input file that stores the required parameters to fetch the private keys.")
	dpfAggregatePartialReportBinary = flag.String("dpf_aggregate_partial_report_binary", "/dpf_aggregate_partial_report", "Binary for partial report aggregation with DPF protocol.")
	workspaceURI                    = flag.String("workspace_uri", "", "The Private directory to save the intermediate query states.")

	pipelineRunner          = flag.String("pipeline_runner", "direct", "Runner for the Beam pipeline: direct or dataflow.")
	dataflowProject         = flag.String("dataflow_project", "", "GCP project of the Dataflow service.")
	dataflowRegion          = flag.String("dataflow_region", "", "Region of Dataflow workers.")
	dataflowTempLocation    = flag.String("dataflow_temp_location", "", "TempLocation for the Dataflow pipeline.")
	dataflowStagingLocation = flag.String("dataflow_staging_location", "", "StagingLocation for the Dataflow pipeline.")

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
	log.Infof("Running aggregation helper version: %v, build: %v\n", version, buildDate)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Exit(err)
	}
	log.Infof("Server is now listening on port: %d", *port)

	server := grpc.NewServer()
	pb.RegisterAggregatorServer(server, service.New(
		service.ServerCfg{
			PrivateKeyDir:                *keyDir,
			OtherHelperInfoDir:           *otherHelperInfoDir,
			ReencryptConversionKeyBinary: *reencryptConversionKeyBinary,
			AggregatePartialReportBinary: *aggregatePartialReportBinary,

			PrivateKeyParamsURI:             *privateKeyParamsURI,
			DpfAggregatePartialReportBinary: *dpfAggregatePartialReportBinary,
			WorkspaceURI:                    *workspaceURI,
		},
		*pipelineRunner,
		service.DataflowCfg{
			Project:         *dataflowProject,
			Region:          *dataflowRegion,
			TempLocation:    *dataflowTempLocation,
			StagingLocation: *dataflowStagingLocation,
		}))
	reflection.Register(server)
	if err := server.Serve(lis); err != nil {
		log.Exit(err)
	}
}
