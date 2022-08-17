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

	"gonum.org/v1/gonum/floats"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

const (
	// ReachType specifies the aggregation type for the Reach/Frequency measurement.
	ReachType = "reach"
	// ConversionType specifies the aggregation type for the conversion measurement.
	ConversionType = "conversion"

	elementBitSize = 64
)

// HierarchicalConfig contains the parameters for the hierarchical query model.
type HierarchicalConfig struct {
	PrefixLengths               []int32
	PrivacyBudgetPerPrefix      []float64
	ExpansionThresholdPerPrefix []uint64
}

// DirectConfig contains the parameters for the direct query model.
type DirectConfig struct {
	BucketIDs []uint128.Uint128
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

// WriteHierarchicalResultsFile writes the hierarchical query results into a file.
func WriteHierarchicalResultsFile(ctx context.Context, results []HierarchicalResult, filename string) error {
	br, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return utils.WriteBytes(ctx, br, filename, nil)
}

// ReadHierarchicalResultsFile reads the hierarchical query results from a file.
func ReadHierarchicalResultsFile(ctx context.Context, filename string) ([]HierarchicalResult, error) {
	br, err := utils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	var results []HierarchicalResult
	if err := json.Unmarshal(br, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// WriteHierarchicalConfigFile writes the HierarchicalConfig into a file.
func WriteHierarchicalConfigFile(ctx context.Context, config *HierarchicalConfig, filename string) error {
	bc, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return utils.WriteBytes(ctx, bc, filename, nil)
}

// ReadHierarchicalConfigFile reads the HierarchicalConfig from a file and validate it.
func ReadHierarchicalConfigFile(ctx context.Context, filename string) (*HierarchicalConfig, error) {
	bc, err := utils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	config := &HierarchicalConfig{}
	if err := json.Unmarshal(bc, config); err != nil {
		return nil, err
	}
	return config, validateHierarchicalConfig(config)
}

// ReadDirectConfigFile reads the DirectConfig from a file and validate it.
func ReadDirectConfigFile(ctx context.Context, filename string) (*DirectConfig, error) {
	bc, err := utils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	config := &DirectConfig{}
	if err := json.Unmarshal(bc, config); err != nil {
		return nil, err
	}
	return config, validateDirectConfig(config)
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
	// The type of aggregation, should be "conversion" or "reach".
	AggregationType  string
	PartialReportURI string
	ExpandConfigURI  string
	QueryID          string
	QueryLevel       int32
	TotalEpsilon     float64
	KeyBitSize       int32

	PartnerSharedInfo *HelperSharedInfo
	ResultDir         string
	// Dataflow Job Hints
	NumWorkers int32
}

// GetRequestPartialResultURI returns the URI of the expected result file for a request.
func GetRequestPartialResultURI(sharedDir, queryID string, level int32) string {
	return utils.JoinPath(sharedDir, fmt.Sprintf("%s_%s_%d", queryID, DefaultPartialResultFile, level))
}

// GetRequestDecryptedReportURI returns the URI of the decrypted report file.
func GetRequestDecryptedReportURI(workDir, queryID string) string {
	return utils.JoinPath(workDir, fmt.Sprintf("%s_%s", queryID, DefaultDecryptedReportFile))
}

// GetRequestExpandParamsURI calculates the expand parameters, saves it into a file and returns the URI.
func GetRequestExpandParamsURI(ctx context.Context, config *HierarchicalConfig, request *AggregateRequest, workDir, sharedDir, partnerSharedDir string) (string, error) {
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

func validateHierarchicalConfig(config *HierarchicalConfig) error {
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

func validateDirectConfig(config *DirectConfig) error {
	if len(config.BucketIDs) == 0 {
		return errors.New("expect nonempty bucket IDs")
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

func getCurrentLevelParams(queryLevel int32, previousResults []dpfaggregator.CompleteHistogram, config *HierarchicalConfig) (*dpfaggregator.ExpandParameters, error) {
	expandParams := &dpfaggregator.ExpandParameters{
		// The DPF levels correspond to the query prefix lengths.
		Level:           config.PrefixLengths[queryLevel] - 1,
		DirectExpansion: false,
	}
	if previousResults == nil {
		expandParams.PreviousLevel = -1
		return expandParams, nil
	}

	expandParams.PreviousLevel = config.PrefixLengths[queryLevel-1] - 1
	expandParams.Prefixes = getNextNonemptyPrefixes(previousResults, config.ExpansionThresholdPerPrefix[queryLevel-1])

	return expandParams, nil
}

func getRequestExpandParamsURI(workDir string, request *AggregateRequest) string {
	return utils.JoinPath(workDir, fmt.Sprintf("%s_%s_%d", request.QueryID, DefaultExpandParamsFile, request.QueryLevel))
}
