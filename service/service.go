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

// Package service implements the RPC method for the conversion aggregation server.
package service

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"

	grpcpb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

const defaultEvaluationContextFile = "EVALUATIONCONTEXT"

// DataflowCfg contains parameters necessary for running pipelines on Dataflow.
type DataflowCfg struct {
	Project         string
	Region          string
	TempLocation    string
	StagingLocation string
}

// ServerCfg contains directories and file paths necessary for the service.
type ServerCfg struct {
	PrivateKeyParamsURI             string
	DpfAggregatePartialReportBinary string
	// The private directory to save the intermediate query states.
	WorkspaceURI string
}

type server struct {
	ServerCfg      ServerCfg
	PipelineRunner string
	DataflowCfg    DataflowCfg
}

// New creates an Aggregator server.
func New(serverCfg ServerCfg, runner string, dataflowCfg DataflowCfg) grpcpb.AggregatorServer {
	return &server{ServerCfg: serverCfg, PipelineRunner: runner, DataflowCfg: dataflowCfg}
}

func (s *server) AggregateDpfPartialReport(ctx context.Context, in *pb.AggregateDpfPartialReportRequest) (*pb.AggregateDpfPartialReportResponse, error) {
	outputEvaluationContextURI := ioutils.JoinPath(s.ServerCfg.WorkspaceURI, fmt.Sprintf("%s_%s_%d", defaultEvaluationContextFile, in.QueryId, in.PrefixLength))

	inputPartialReportURI := in.PartialReportUri
	if in.PreviousPrefixLength >= 0 {
		inputPartialReportURI = ioutils.JoinPath(s.ServerCfg.WorkspaceURI, fmt.Sprintf("%s_%s_%d", defaultEvaluationContextFile, in.QueryId, in.PreviousPrefixLength))
	}

	args := []string{
		"--partial_report_uri=" + inputPartialReportURI,
		"--expand_parameters_uri=" + in.ExpandParametersUri,
		"--evaluation_context_uri=" + outputEvaluationContextURI,
		"--partial_histogram_uri=" + in.PartialHistogramUri,
		"--epsilon=" + fmt.Sprintf("%f", in.Epsilon),
		"--private_key_params_uri=" + s.ServerCfg.PrivateKeyParamsURI,
		"--runner=" + s.PipelineRunner,
	}

	if s.PipelineRunner == "dataflow" {
		args = append(args,
			"--project="+s.DataflowCfg.Project,
			"--region="+s.DataflowCfg.Region,
			"--temp_location="+s.DataflowCfg.TempLocation,
			"--staging_location="+s.DataflowCfg.StagingLocation,
			"--worker_binary="+s.ServerCfg.DpfAggregatePartialReportBinary,
			"--num_workers="+fmt.Sprint(in.NumWorkers),
		)
	}
	return &pb.AggregateDpfPartialReportResponse{}, exec.CommandContext(ctx, s.ServerCfg.DpfAggregatePartialReportBinary, args...).Run()
}
