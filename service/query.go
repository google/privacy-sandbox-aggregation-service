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

// Package query contains functions for querying the aggregation results with hierarchical DPF key expansion.
package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/sync/errgroup"
	"github.com/pborman/uuid"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	cryptopb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	grpcpb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
	servicepb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

const elementBitSize = 64

// ExpansionConfig contains the parameters that define the hierarchy for how the DPF key will be expanded.
type ExpansionConfig struct {
	PrefixLengths               []int32
	ExpansionThresholdPerPrefix []uint64
}

// WriteExpansionConfigFile writes the ExpansionConfig into a file.
func WriteExpansionConfigFile(ctx context.Context, config *ExpansionConfig, filename string) error {
	bc, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return ioutils.WriteBytes(ctx, bc, filename)
}

func validateExpansionConfig(config *ExpansionConfig) error {
	if len(config.PrefixLengths) == 0 {
		return errors.New("expect nonempty PrefixLengths")
	}

	if got, want := len(config.ExpansionThresholdPerPrefix), len(config.PrefixLengths); got != want {
		return fmt.Errorf("expect len(ExpansionThreshold)=%d, got %d", want, got)
	}
	var cur int32
	for _, l := range config.PrefixLengths {
		if l <= 0 {
			return fmt.Errorf("prefix length should be positive, got %d", l)
		}
		if l <= cur {
			return errors.New("prefix lengths should be in ascending order")
		}
		cur = l
	}
	return nil
}

// ReadExpansionConfigFile reads the ExpansionConfig from a file and validate it.
func ReadExpansionConfigFile(ctx context.Context, filename string) (*ExpansionConfig, error) {
	bc, err := ioutils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	config := &ExpansionConfig{}
	if err := json.Unmarshal(bc, config); err != nil {
		return nil, err
	}
	return config, validateExpansionConfig(config)
}

type aggregateParams struct {
	PrefixesFile                                 string
	SumParamsFile                                string
	PartialHistogramFile1, PartialHistogramFile2 string
	PartialReportFile1, PartialReportFile2       string
}

func aggregateReports(ctx context.Context, params aggregateParams, client1, client2 grpcpb.AggregatorClient) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		_, err := client1.AggregateDpfPartialReport(ctx, &servicepb.AggregateDpfPartialReportRequest{
			PartialReportFile:    params.PartialReportFile1,
			PartialHistogramFile: params.PartialHistogramFile1,
			PrefixesFile:         params.PrefixesFile,
			SumDpfParametersFile: params.SumParamsFile,
		})
		return err
	})

	g.Go(func() error {
		_, err := client2.AggregateDpfPartialReport(ctx, &servicepb.AggregateDpfPartialReportRequest{
			PartialReportFile:    params.PartialReportFile2,
			PartialHistogramFile: params.PartialHistogramFile2,
			PrefixesFile:         params.PrefixesFile,
			SumDpfParametersFile: params.SumParamsFile,
		})
		return err
	})

	return g.Wait()
}

// PrefixHistogramParams contains the parameters for querying the histogram of given prefixes.
type PrefixHistogramParams struct {
	Prefixes                               *cryptopb.HierarchicalPrefixes
	SumParams                              *cryptopb.IncrementalDpfParameters
	PartialReportFile1, PartialReportFile2 string
	PartialAggregationDir                  string
	ParamsDir                              string
	Helper1, Helper2                       grpcpb.AggregatorClient
}

// getPrefixHistogram calls the RPC methods on both helpers and merges the generated partial aggregation results.
func getPrefixHistogram(ctx context.Context, params *PrefixHistogramParams) ([]dpfaggregator.CompleteHistogram, error) {
	tempID := uuid.New()
	prefixFile := fmt.Sprintf("%s/prefixes%s.txt", params.ParamsDir, tempID)
	if err := cryptoio.SavePrefixes(ctx, prefixFile, params.Prefixes); err != nil {
		return nil, err
	}
	sumParamsFile := fmt.Sprintf("%s/sum_params%s.txt", params.ParamsDir, tempID)
	if err := cryptoio.SaveDPFParameters(ctx, sumParamsFile, params.SumParams); err != nil {
		return nil, err
	}

	tempPartialResultFile1 := fmt.Sprintf("%s/%s_1.txt", params.PartialAggregationDir, tempID)
	tempPartialResultFile2 := fmt.Sprintf("%s/%s_2.txt", params.PartialAggregationDir, tempID)

	if err := aggregateReports(ctx, aggregateParams{
		PrefixesFile:          prefixFile,
		SumParamsFile:         sumParamsFile,
		PartialHistogramFile1: tempPartialResultFile1,
		PartialHistogramFile2: tempPartialResultFile2,
	}, params.Helper1, params.Helper2); err != nil {
		return nil, err
	}
	partial1, err := dpfaggregator.ReadPartialHistogram(ctx, tempPartialResultFile1)
	if err != nil {
		return nil, err
	}
	partial2, err := dpfaggregator.ReadPartialHistogram(ctx, tempPartialResultFile2)
	if err != nil {
		return nil, err
	}

	for idx := range partial2 {
		if _, ok := partial1[idx]; !ok {
			return nil, fmt.Errorf("index %d appears in partial2, missing in partial1", idx)
		}
	}

	var result []dpfaggregator.CompleteHistogram
	for idx := range partial1 {
		if _, ok := partial2[idx]; !ok {
			return nil, fmt.Errorf("index %d appears in partial1, missing in partial2", idx)
		}
		result = append(result, dpfaggregator.CompleteHistogram{
			Index: idx,
			Sum:   partial1[idx].PartialSum + partial2[idx].PartialSum,
		})
	}
	return result, nil
}

func getNextNonemptyPrefixes(result []dpfaggregator.CompleteHistogram, threshold uint64) []uint64 {
	var prefixes []uint64
	for _, r := range result {
		if r.Sum >= threshold {
			prefixes = append(prefixes, r.Index)
		}
	}
	return prefixes
}

func extendPrefixDomains(sumParams *cryptopb.IncrementalDpfParameters, prefixLength int32) {
	sumParams.Params = append(sumParams.Params, &dpfpb.DpfParameters{
		LogDomainSize:  prefixLength,
		ElementBitsize: elementBitSize,
	})
}

// HierarchicalResult records the aggregation result at certain prefix length.
//
// TODO: Add PrivacyBudgetConsumed field
type HierarchicalResult struct {
	PrefixLength int32
	Histogram    []dpfaggregator.CompleteHistogram
	// The threshold applied to the above result, which generates prefixes for the next-level expansion.
	ExpansionThreshold uint64
}

// HierarchicalAggregation queries the hierarchical aggregation results.
func HierarchicalAggregation(ctx context.Context, params *PrefixHistogramParams, config *ExpansionConfig) ([]HierarchicalResult, error) {
	var results []HierarchicalResult
	for i, threshold := range config.ExpansionThresholdPerPrefix {
		extendPrefixDomains(params.SumParams, config.PrefixLengths[i])
		result, err := getPrefixHistogram(ctx, params)
		if err != nil {
			return nil, err
		}
		params.Prefixes.Prefixes = append(params.Prefixes.Prefixes, &cryptopb.DomainPrefixes{Prefix: getNextNonemptyPrefixes(result, threshold)})
		results = append(results, HierarchicalResult{PrefixLength: config.PrefixLengths[i], Histogram: result, ExpansionThreshold: threshold})
	}
	return results, nil
}

// WriteHierarchicalResultsFile writes the hierarchical query results into a file.
func WriteHierarchicalResultsFile(results []HierarchicalResult, filename string) error {
	br, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, br, os.ModePerm)
}

// ReadHierarchicalResultsFile reads the hierarchical query results from a file.
func ReadHierarchicalResultsFile(filename string) ([]HierarchicalResult, error) {
	br, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var results []HierarchicalResult
	if err := json.Unmarshal(br, &results); err != nil {
		return nil, err
	}
	return results, nil
}
