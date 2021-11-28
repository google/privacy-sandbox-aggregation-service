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
	"strings"

	log "github.com/golang/glog"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/floats"
	"google.golang.org/grpc"
	"lukechampine.com/uint128"
	"github.com/pborman/uuid"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/service/utils"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	grpcMetadata "google.golang.org/grpc/metadata"
	cryptopb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	grpcpb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
	servicepb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

const elementBitSize = 64

// ExpansionConfig contains the parameters that define the hierarchy for how the DPF key will be expanded.
type ExpansionConfig struct {
	PrefixLengths               []int32
	PrivacyBudgetPerPrefix      []float64
	ExpansionThresholdPerPrefix []uint64
}

// PrefixHistogramQuery contains the parameters and methods for querying the histogram of given prefixes.
type PrefixHistogramQuery struct {
	QueryID                              string
	Prefixes                             [][]uint128.Uint128
	PrefixeLengths                       []int32
	PreviousPrefixLength                 int32
	PartialReportURI1, PartialReportURI2 string
	PartialAggregationDir                string
	ParamsDir                            string
	Helper1, Helper2                     *grpc.ClientConn
	ImpersonatedSvcAccount               string
	Epsilon                              float64
	KeyBitSize                           int32
	// Dataflow Job Hints
	NumWorkers int32
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

type aggregateParams struct {
	ExpandParamsURI                            string
	PartialHistogramURI1, PartialHistogramURI2 string
	Epsilon                                    float64
}

func (phq *PrefixHistogramQuery) aggregateReports(ctx context.Context, params aggregateParams) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		newCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		audience := "https://" + strings.Split(phq.Helper1.Target(), ":")[0]

		if phq.ImpersonatedSvcAccount != "" {
			var err error
			newCtx, err = addGRPCAuthHeaderToContext(newCtx, audience, phq.ImpersonatedSvcAccount)
			if err != nil {
				log.Errorf("Helper Server 1 - Continuing without auth header: %v", err)
				return err
			}
		}

		_, err := grpcpb.NewAggregatorClient(phq.Helper1).AggregateDpfPartialReport(newCtx, &servicepb.AggregateDpfPartialReportRequest{
			PartialReportUri:     phq.PartialReportURI1,
			PartialHistogramUri:  params.PartialHistogramURI1,
			Epsilon:              params.Epsilon,
			ExpandParametersUri:  params.ExpandParamsURI,
			QueryId:              phq.QueryID,
			PreviousPrefixLength: phq.PreviousPrefixLength,
			PrefixLengths:        phq.PrefixeLengths,
			KeyBitSize:           phq.KeyBitSize,
			NumWorkers:           phq.NumWorkers,
		})
		if err != nil {
			log.Errorf("Helper Server 1: %v", err)
		}
		return err
	})

	g.Go(func() error {
		newCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		audience := "https://" + strings.Split(phq.Helper2.Target(), ":")[0]
		if phq.ImpersonatedSvcAccount != "" {
			var err error
			newCtx, err = addGRPCAuthHeaderToContext(newCtx, audience, phq.ImpersonatedSvcAccount)
			if err != nil {
				log.Errorf("Helper Server 2 - Continuing without auth header: %v", err)
				return err
			}
		}

		_, err := grpcpb.NewAggregatorClient(phq.Helper2).AggregateDpfPartialReport(newCtx, &servicepb.AggregateDpfPartialReportRequest{
			PartialReportUri:     phq.PartialReportURI2,
			PartialHistogramUri:  params.PartialHistogramURI2,
			Epsilon:              params.Epsilon,
			ExpandParametersUri:  params.ExpandParamsURI,
			QueryId:              phq.QueryID,
			PreviousPrefixLength: phq.PreviousPrefixLength,
			PrefixLengths:        phq.PrefixeLengths,
			KeyBitSize:           phq.KeyBitSize,
			NumWorkers:           phq.NumWorkers,
		})
		if err != nil {
			log.Errorf("Helper Server 2: %v", err)
		}
		return err
	})

	return g.Wait()
}

// getPrefixHistogram calls the RPC methods on both helpers and merges the generated partial aggregation results.
func (phq *PrefixHistogramQuery) getPrefixHistogram(ctx context.Context) ([]dpfaggregator.CompleteHistogram, error) {
	expandParams := &dpfaggregator.ExpandParameters{
		PreviousLevel: phq.PreviousPrefixLength - 1,
		Prefixes:      phq.Prefixes,
	}
	for _, l := range phq.PrefixeLengths {
		expandParams.Levels = append(expandParams.Levels, l-1)
	}

	expandParamsURI := fmt.Sprintf("%s/expand_params%s.txt", phq.ParamsDir, phq.QueryID)
	if err := dpfaggregator.SaveExpandParameters(ctx, expandParams, expandParamsURI); err != nil {
		return nil, err
	}

	tempPartialResultURI1 := fmt.Sprintf("%s/%s_1.txt", phq.PartialAggregationDir, phq.QueryID)
	tempPartialResultURI2 := fmt.Sprintf("%s/%s_2.txt", phq.PartialAggregationDir, phq.QueryID)

	if err := phq.aggregateReports(ctx, aggregateParams{
		ExpandParamsURI:      expandParamsURI,
		PartialHistogramURI1: tempPartialResultURI1,
		PartialHistogramURI2: tempPartialResultURI2,
	}); err != nil {
		return nil, err
	}
	partial1, err := dpfaggregator.ReadPartialHistogram(ctx, tempPartialResultURI1)
	if err != nil {
		return nil, err
	}
	partial2, err := dpfaggregator.ReadPartialHistogram(ctx, tempPartialResultURI2)
	if err != nil {
		return nil, err
	}

	return dpfaggregator.MergePartialResult(partial1, partial2)
}

// HierarchicalAggregation queries the hierarchical aggregation results.
func (phq *PrefixHistogramQuery) HierarchicalAggregation(ctx context.Context, epsilon float64, config *ExpansionConfig) ([]HierarchicalResult, error) {
	phq.QueryID = uuid.New()

	var results []HierarchicalResult
	phq.PreviousPrefixLength = 0
	phq.Prefixes = [][]uint128.Uint128{{}}
	for i, threshold := range config.ExpansionThresholdPerPrefix {
		// The user is supposed to query one hierarchy at a time.
		phq.PrefixeLengths = []int32{config.PrefixLengths[i]}
		// Use naive composition by simply splitting the epsilon based on the privacy budget config.
		phq.Epsilon = epsilon * config.PrivacyBudgetPerPrefix[i]
		result, err := phq.getPrefixHistogram(ctx)
		if err != nil {
			return nil, err
		}
		phq.Prefixes = [][]uint128.Uint128{
			getNextNonemptyPrefixes(result, threshold),
		}
		results = append(results, HierarchicalResult{PrefixLength: config.PrefixLengths[i], Histogram: result, ExpansionThreshold: threshold})
		phq.PreviousPrefixLength = config.PrefixLengths[i]
	}
	return results, nil
}

// WriteHierarchicalResultsFile writes the hierarchical query results into a file.
func WriteHierarchicalResultsFile(ctx context.Context, results []HierarchicalResult, filename string) error {
	br, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return ioutils.WriteBytes(ctx, br, filename)
}

// ReadHierarchicalResultsFile reads the hierarchical query results from a file.
func ReadHierarchicalResultsFile(ctx context.Context, filename string) ([]HierarchicalResult, error) {
	br, err := ioutils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	var results []HierarchicalResult
	if err := json.Unmarshal(br, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// WriteExpansionConfigFile writes the ExpansionConfig into a file.
func WriteExpansionConfigFile(ctx context.Context, config *ExpansionConfig, filename string) error {
	bc, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return ioutils.WriteBytes(ctx, bc, filename)
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

// Default basic file names.
const (
	DefaultExpandParamsFile    = "EXPANDPARAMS"
	DefaultPartialResultFile   = "PARTIALRESULT"
	DefaultDecryptedReportFile = "DECRYPTEDREPORT"
)

// HelperSharedInfo stores information that is shared by other helpers.
type HelperSharedInfo struct {
	Origin string
	// The shared directory where the other helpers will read the intermediate results.
	SharedDir   string
	PubSubTopic string
}

// AggregateRequest contains infomation that are necessary for the query.
type AggregateRequest struct {
	PartialReportURI string
	ExpandConfigURI  string
	QueryID          string
	QueryLevel       int32
	TotalEpsilon     float64
	KeyBitSize       int32
	IsHierarchical   bool

	PartnerSharedInfo *HelperSharedInfo
	ResultDir         string
	// Dataflow Job Hints
	NumWorkers int32
}

// GetRequestPartialResultURI returns the URI of the expected result file for a request.
func GetRequestPartialResultURI(sharedDir, queryID string, level int32) string {
	return ioutils.JoinPath(sharedDir, fmt.Sprintf("%s_%s_%d", queryID, DefaultPartialResultFile, level))
}

// GetRequestDecryptedReportURI returns the URI of the decrypted report file.
func GetRequestDecryptedReportURI(workDir, queryID string) string {
	return ioutils.JoinPath(workDir, fmt.Sprintf("%s_%s", queryID, DefaultDecryptedReportFile))
}

// GetRequestExpandParamsURI calculates the expand parameters, saves it into a file and returns the URI.
func GetRequestExpandParamsURI(ctx context.Context, config *ExpansionConfig, request *AggregateRequest, workDir, sharedDir, partnerSharedDir string) (string, error) {
	finalLevel := int32(len(config.PrefixLengths)) - 1
	if request.QueryLevel > finalLevel {
		return "", fmt.Errorf("expect request level <= final level %d, got %d", finalLevel, request.QueryLevel)
	}

	var (
		results []dpfaggregator.CompleteHistogram
		err     error
	)
	// Read the parameters and results of the previous level.
	if request.QueryLevel > 0 {
		partial1, err := dpfaggregator.ReadPartialHistogram(ctx, GetRequestPartialResultURI(sharedDir, request.QueryID, request.QueryLevel-1))
		if err != nil {
			return "", err
		}
		partial2, err := dpfaggregator.ReadPartialHistogram(ctx, GetRequestPartialResultURI(partnerSharedDir, request.QueryID, request.QueryLevel-1))
		if err != nil {
			return "", err
		}

		results, err = dpfaggregator.MergePartialResult(partial1, partial2)
		if err != nil {
			return "", err
		}
	}

	expandParams, err := getCurrentLevelParams(request.QueryLevel, results, config)
	if err != nil {
		return "", err
	}

	expandParamsURI := getRequestExpandParamsURI(workDir, request)
	if err := dpfaggregator.SaveExpandParameters(ctx, expandParams, expandParamsURI); err != nil {
		return "", err
	}

	return expandParamsURI, nil
}

func validateExpansionConfig(config *ExpansionConfig) error {
	if len(config.PrefixLengths) == 0 {
		return errors.New("expect nonempty PrefixLengths")
	}

	if got, want := len(config.PrivacyBudgetPerPrefix), len(config.PrefixLengths); got != want {
		return fmt.Errorf("expect len(PrivacyBudgetPerPrefix)=%d, got %d", want, got)
	}
	var totalBudget float64
	for _, p := range config.PrivacyBudgetPerPrefix {
		if p > 1.0 {
			return fmt.Errorf("budget for each prefix length should not be larger than 1, got %v", p)
		}
		totalBudget += p
	}
	if !floats.EqualWithinAbsOrRel(totalBudget, 1.0, 1e-6, 1e-6) {
		return fmt.Errorf("total budget should add up to 1, got %v", totalBudget)
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

func getNextNonemptyPrefixes(result []dpfaggregator.CompleteHistogram, threshold uint64) []uint128.Uint128 {
	var prefixes []uint128.Uint128
	for _, r := range result {
		if r.Sum >= threshold {
			prefixes = append(prefixes, r.Bucket)
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

func addGRPCAuthHeaderToContext(ctx context.Context, audience, impersonatedSvcAccount string) (context.Context, error) {
	token, err := utils.GetAuthorizationToken(ctx, audience, impersonatedSvcAccount)
	if err != nil {
		return ctx, err
	}

	// Add AccessToken to grpcContext
	return grpcMetadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token), nil
}

func getCurrentLevelParams(queryLevel int32, previousResults []dpfaggregator.CompleteHistogram, config *ExpansionConfig) (*dpfaggregator.ExpandParameters, error) {
	expandParams := &dpfaggregator.ExpandParameters{
		// The DPF levels correspond to the query prefix lengths.
		Levels: []int32{config.PrefixLengths[queryLevel] - 1},
	}
	if previousResults == nil {
		expandParams.PreviousLevel = -1
		expandParams.Prefixes = [][]uint128.Uint128{{}}
		return expandParams, nil
	}

	expandParams.PreviousLevel = config.PrefixLengths[queryLevel-1] - 1
	expandParams.Prefixes = [][]uint128.Uint128{
		getNextNonemptyPrefixes(previousResults, config.ExpansionThresholdPerPrefix[queryLevel-1]),
	}

	return expandParams, nil
}

func getRequestExpandParamsURI(workDir string, request *AggregateRequest) string {
	return ioutils.JoinPath(workDir, fmt.Sprintf("%s_%s_%d", request.QueryID, DefaultExpandParamsFile, request.QueryLevel))
}
