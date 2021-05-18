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

// This binary simulates the report origins sending requests to helpers for aggregating partial reports.
package main

import (
	"context"
	"flag"

	log "github.com/golang/glog"
	"google.golang.org/grpc"
	"github.com/google/privacy-sandbox-aggregation-service/service/query"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	grpcpb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

var (
	helperAddr1 = flag.String("helper_addr1", "", "Address of helper 1.")
	helperAddr2 = flag.String("helper_addr2", "", "Address of helper 2.")

	partialReportFile1        = flag.String("partial_report_file1", "", "Input partial report for helper 1.")
	partialReportFile2        = flag.String("partial_report_file2", "", "Input partial report for helper 2.")
	hierarchicalHistogramFile = flag.String("hierarchical_histogram_file", "", "Output file for the hierarchical aggregation results.")
	expansionConfigFile       = flag.String("expansion_config_file", "", "Input file for the expansion configurations that defines the query hierarchy.")
	paramsDir                 = flag.String("params_dir", "", "Input directory that stores the parameter files.")
	partialAggregationDir     = flag.String("partial_aggregation_dir", "", "Output directory for the partial aggregation files.")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	expansionConfig, err := query.ReadExpansionConfigFile(ctx, *expansionConfigFile)
	if err != nil {
		log.Exit(err)
	}

	conn1, err := grpc.Dial(*helperAddr1, nil)
	if err != nil {
		log.Exit(err)
	}
	defer conn1.Close()

	conn2, err := grpc.Dial(*helperAddr2, nil)
	if err != nil {
		log.Exit(err)
	}
	defer conn2.Close()

	params := &query.PrefixHistogramParams{
		Prefixes:              &pb.HierarchicalPrefixes{Prefixes: []*pb.DomainPrefixes{{}}},
		SumParams:             &pb.IncrementalDpfParameters{},
		PartialReportFile1:    *partialReportFile1,
		PartialReportFile2:    *partialReportFile2,
		PartialAggregationDir: *partialAggregationDir,
		ParamsDir:             *paramsDir,
		Helper1:               grpcpb.NewAggregatorClient(conn1),
		Helper2:               grpcpb.NewAggregatorClient(conn2),
	}

	results, err := query.HierarchicalAggregation(ctx, params, expansionConfig)
	if err != nil {
		log.Exit(err)
	}
	if err := query.WriteHierarchicalResultsFile(results, *hierarchicalHistogramFile); err != nil {
		log.Exit(err)
	}
}
