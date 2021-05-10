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
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	grpcpb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
	servicepb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

var (
	helperAddr1 = flag.String("helper_addr1", "", "Address of helper 1.")
	helperAddr2 = flag.String("helper_addr2", "", "Address of helper 2.")

	partialReportFile1      = flag.String("partial_report_file1", "", "Input partial report for helper 1.")
	partialReportFile2      = flag.String("partial_report_file2", "", "Input partial report for helper 2.")
	exponentiatedKeyFile1   = flag.String("exponentiated_key_file1", "", "Input/output exponentiated key for helper 1.")
	exponentiatedKeyFile2   = flag.String("exponentiated_key_file2", "", "Input/output exponentiated key for helper 2.")
	partialAggregationFile1 = flag.String("partial_aggregation_file1", "", "Output partial aggregation from helper 1.")
	partialAggregationFile2 = flag.String("partial_aggregation_file2", "", "Output partial aggregation from helper 2.")
	ignorePrivacy           = flag.Bool("ignore_privacy", false, "If ignore privacy during aggregation.")
)

// Send RPC requests to both helpers. Helpers simultaneously exponentiate the encrypted keys for the other helper, and aggregate partial reports after that.
func aggregateReports(ctx context.Context) error {
	conn1, err := grpc.Dial(*helperAddr1, nil)
	if err != nil {
		return err
	}
	defer conn1.Close()
	client1 := grpcpb.NewAggregatorClient(conn1)

	conn2, err := grpc.Dial(*helperAddr2, nil)
	if err != nil {
		return err
	}
	defer conn2.Close()
	client2 := grpcpb.NewAggregatorClient(conn2)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if _, err := client1.ExponentiateConversionKey(ctx, &servicepb.ExponentiateConversionKeyRequest{
			PartialReportFile:    *partialReportFile1,
			ExponentiatedKeyFile: *exponentiatedKeyFile1,
		}); err != nil {
			return err
		}
		_, err := client2.AggregatePartialReport(ctx, &servicepb.AggregatePartialReportRequest{
			PartialReportFile:      *partialReportFile2,
			ExponentiatedKeyFile:   *exponentiatedKeyFile1,
			PartialAggregationFile: *partialAggregationFile2,
			IgnorePrivacy:          *ignorePrivacy,
		})
		return err
	})

	g.Go(func() error {
		if _, err := client2.ExponentiateConversionKey(ctx, &servicepb.ExponentiateConversionKeyRequest{
			PartialReportFile:    *partialReportFile2,
			ExponentiatedKeyFile: *exponentiatedKeyFile2,
		}); err != nil {
			return err
		}
		_, err := client1.AggregatePartialReport(ctx, &servicepb.AggregatePartialReportRequest{
			PartialReportFile:      *partialReportFile1,
			ExponentiatedKeyFile:   *exponentiatedKeyFile2,
			PartialAggregationFile: *partialAggregationFile1,
			IgnorePrivacy:          *ignorePrivacy,
		})
		return err
	})

	return g.Wait()
}

func main() {
	flag.Parse()

	if err := aggregateReports(context.Background()); err != nil {
		log.Exit(err)
	}
}
