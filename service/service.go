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
	"os/exec"
	"path"

	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"

	grpcpb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

// DataflowCfg contains parameters necessary for running pipelines on Dataflow.
type DataflowCfg struct {
	Project         string
	Region          string
	TempLocation    string
	StagingLocation string
}

// ServerCfg contains directories and file paths necessary for the service.
type ServerCfg struct {
	PrivateKeyDir                   string
	OtherHelperInfoDir              string
	ReencryptConversionKeyBinary    string
	AggregatePartialReportBinary    string
	DpfAggregatePartialReportBinary string
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

func (s *server) CreateCryptoKeys(ctx context.Context, in *pb.CreateCryptoKeysRequest) (*pb.CreateCryptoKeysResponse, error) {
	sPub, ePub, err := cryptoio.CreateKeysAndSecret(s.ServerCfg.PrivateKeyDir)
	if err != nil {
		return nil, err
	}
	return &pb.CreateCryptoKeysResponse{
		StandardPublicKey: sPub,
		ElgamalPublicKey:  ePub,
	}, nil
}

func (s *server) SaveOtherHelperInfo(ctx context.Context, in *pb.SaveOtherHelperInfoRequest) (*pb.SaveOtherHelperInfoResponse, error) {
	return &pb.SaveOtherHelperInfoResponse{}, cryptoio.SaveElGamalPublicKey(path.Join(s.ServerCfg.OtherHelperInfoDir, cryptoio.DefaultElgamalPublicKey), in.ElgamalPublicKey)
}

func (s *server) ExponentiateConversionKey(ctx context.Context, in *pb.ExponentiateConversionKeyRequest) (*pb.ExponentiateConversionKeyResponse, error) {
	args := []string{
		"--partial_report_file=" + in.PartialReportFile,
		"--exponentiated_key_file=" + in.ExponentiatedKeyFile,
		"--private_key_dir=" + s.ServerCfg.PrivateKeyDir,
		"--other_public_key_dir=" + s.ServerCfg.OtherHelperInfoDir,
		"--runner=" + s.PipelineRunner,
	}

	if s.PipelineRunner == "dataflow" {
		args = append(args,
			"--project="+s.DataflowCfg.Project,
			"--temp_location="+s.DataflowCfg.TempLocation,
			"--staging_location="+s.DataflowCfg.StagingLocation,
			"--worker_binary="+s.ServerCfg.ReencryptConversionKeyBinary,
		)
	}

	return &pb.ExponentiateConversionKeyResponse{}, exec.CommandContext(ctx, s.ServerCfg.ReencryptConversionKeyBinary, args...).Run()
}

func (s *server) AggregatePartialReport(ctx context.Context, in *pb.AggregatePartialReportRequest) (*pb.AggregatePartialReportResponse, error) {
	privateStr := "true"
	if in.IgnorePrivacy {
		privateStr = "false"
	}

	args := []string{
		"--partial_report_file=" + in.PartialReportFile,
		"--exponentiated_key_file=" + in.ExponentiatedKeyFile,
		"--partial_aggregation_file=" + in.PartialAggregationFile,
		"--private=" + privateStr,
		"--private_key_dir=" + s.ServerCfg.PrivateKeyDir,
		"--runner=" + s.PipelineRunner,
	}

	if s.PipelineRunner == "dataflow" {
		args = append(args,
			"--project="+s.DataflowCfg.Project,
			"--temp_location="+s.DataflowCfg.TempLocation,
			"--staging_location="+s.DataflowCfg.StagingLocation,
			"--worker_binary="+s.ServerCfg.AggregatePartialReportBinary,
		)
	}

	return &pb.AggregatePartialReportResponse{}, exec.CommandContext(ctx, s.ServerCfg.AggregatePartialReportBinary, args...).Run()
}

func (s *server) AggregateDpfPartialReport(ctx context.Context, in *pb.AggregateDpfPartialReportRequest) (*pb.AggregateDpfPartialReportResponse, error) {
	args := []string{
		"--partial_report_file=" + in.PartialReportFile,
		"--sum_parameters_file=" + in.SumDpfParametersFile,
		"--prefixes_file=" + in.PrefixesFile,
		"--partial_histogram_file=" + in.PartialHistogramFile,
		"--private_key_dir=" + s.ServerCfg.PrivateKeyDir,
		"--runner=" + s.PipelineRunner,
	}

	if s.PipelineRunner == "dataflow" {
		args = append(args,
			"--project="+s.DataflowCfg.Project,
			"--temp_location="+s.DataflowCfg.TempLocation,
			"--staging_location="+s.DataflowCfg.StagingLocation,
			"--worker_binary="+s.ServerCfg.DpfAggregatePartialReportBinary,
		)
	}
	return &pb.AggregateDpfPartialReportResponse{}, exec.CommandContext(ctx, s.ServerCfg.DpfAggregatePartialReportBinary, args...).Run()
}
