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

// Package aggregatorservice contains the functions needed for handling the aggregation requests.
package aggregatorservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"google.golang.org/api/dataflow/v1b3"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartyaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/service/query"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

// DataflowCfg contains parameters necessary for running pipelines on Dataflow.
type DataflowCfg struct {
	Project             string
	Region              string
	Zone                string
	TempLocation        string
	StagingLocation     string
	WorkerMachineType   string
	MaxNumWorkers       int
	ServiceAccountEmail string
}

// ServerCfg contains file URIs necessary for the service.
type ServerCfg struct {
	PrivateKeyParamsURI                  string
	DpfAggregatePartialReportBinary      string
	DpfAggregateReachPartialReportBinary string
	OnepartyAggregateReportBinary        string
	WorkspaceURI                         string
}

// SharedInfoHandler handles HTTP requests for the information shared with other helpers.
type SharedInfoHandler struct {
	SharedInfo *query.HelperSharedInfo
}

func (h *SharedInfoHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(h.SharedInfo)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// QueryHandler handles the request in the pubsub messages.
type QueryHandler struct {
	ServerCfg                 ServerCfg
	PipelineRunner            string
	DataflowCfg               DataflowCfg
	Origin                    string
	SharedDir                 string
	RequestPubSubTopic        string
	RequestPubsubSubscription string

	PubSubTopicClient, PubSubSubscriptionClient *pubsub.Client
	GCSClient                                   *storage.Client
	DataflowSvc                                 *dataflow.Service
}

// Setup creates the cloud API clients.
func (h *QueryHandler) Setup(ctx context.Context) error {
	topicProject, _, err := utils.ParsePubSubResourceName(h.RequestPubSubTopic)
	if err != nil {
		return err
	}
	h.PubSubTopicClient, err = pubsub.NewClient(ctx, topicProject)
	if err != nil {
		return err
	}

	subscriptionProject, _, err := utils.ParsePubSubResourceName(h.RequestPubsubSubscription)
	if err != nil {
		return err
	}

	if subscriptionProject == topicProject {
		h.PubSubSubscriptionClient = h.PubSubTopicClient
	} else {
		h.PubSubSubscriptionClient, err = pubsub.NewClient(ctx, subscriptionProject)
		if err != nil {
			return err
		}

	}

	if strings.HasPrefix(h.SharedDir, "gs://") {
		h.GCSClient, err = storage.NewClient(ctx)
	}
	if err != nil {
		return err
	}

	if h.PipelineRunner == "dataflow" {
		h.DataflowSvc, err = dataflow.NewService(ctx)
	}
	return err
}

// Close closes the cloud API clients.
func (h *QueryHandler) Close() {
	h.PubSubTopicClient.Close()
	h.PubSubSubscriptionClient.Close()

	if h.GCSClient != nil {
		h.GCSClient.Close()
	}
}

// SetupPullRequests gets ready to pull requests contained in a PubSub message subscription, and handles the request.
func (h *QueryHandler) SetupPullRequests(ctx context.Context) error {
	_, subID, err := utils.ParsePubSubResourceName(h.RequestPubsubSubscription)
	if err != nil {
		return err
	}
	sub := h.PubSubSubscriptionClient.Subscription(subID)

	// Only allow pulling one message at a time to avoid overloading the memory.
	sub.ReceiveSettings.Synchronous = true
	sub.ReceiveSettings.MaxOutstandingMessages = 1
	sub.ReceiveSettings.MaxExtension = 24 * time.Hour // extending from 60min default to 1 day
	return sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		request := &query.AggregateRequest{}
		err := json.Unmarshal(msg.Data, request)
		if err != nil {
			log.Error(err)
			msg.Nack()
			return
		}

		jobDone := false
		if h.PipelineRunner == "dataflow" {
			// check if dataflow job with queryId-level-origin is already running / finished / failed
			/*
			 -- no job with "queryId-level-origin" name --> schedule
			 -- job with "queryId-level-origin" name and state one of JOB_STATE_RUNNING, JOB_STATE_PENDING,
			    JOB_STATE_QUEUED, JOB_STATE_STOPPED --> wait and periodically check on job status
			 -- job with "queryId-level-origin" name and state one of JOB_STATE_RESOURCE_CLEANING_UP, JOB_STATE_DONE
			    --> ack message and schedule next level if applicable
			 -- job with "queryId-level-origin" name and state none of above --> assume non-recoverable failure, ack message,
			 	  don't schedule any other levels
			*/
			jobsSvc := dataflow.NewProjectsLocationsJobsService(h.DataflowSvc)
			// currently unpaged, TODO add paging through results
			jobs, err := jobsSvc.List(h.DataflowCfg.Project, h.DataflowCfg.Region).Do()
			if err != nil {
				log.Error(fmt.Errorf("Failed checking for existing dataflow jobs: %v", err))
				msg.Nack()
				return
			}

			waitStates := map[string]bool{
				"JOB_STATE_RUNNING": true,
				"JOB_STATE_PENDING": true,
				"JOB_STATE_QUEUED":  true,
				"JOB_STATE_STOPPED": true,
			}

			continueStates := map[string]bool{
				"JOB_STATE_RESOURCE_CLEANING_UP": true,
				"JOB_STATE_DONE":                 true,
			}

			jobInWaitState := false
			jobID := ""
			for _, job := range jobs.Jobs {
				if job.Name == fmt.Sprintf("%s-%v-%s", request.QueryID, request.QueryLevel, h.Origin) {
					if waitStates[job.CurrentState] {
						// wait and periodically check on job status
						log.Infof("Found dataflow job %s, %s in state %s", job.Name, job.Id, job.CurrentState)
						jobInWaitState = true
						jobID = job.Id
					} else if continueStates[job.CurrentState] {
						// use normal flow below to ack message and schedule next level if applicable
						jobDone = true
					} else {
						// assume non-recoverable failure, ack message, don't schedule any other levels
						log.Errorf("Dataflow job %s, %s found with unrecoverable state: %s ", job.Name, job.Id, job.CurrentState)
						msg.Ack()
						return
					}

					break
				}
			}

			for jobInWaitState {
				if jobID == "" {
					log.Errorf("No jobId set for job in wait state")
					msg.Nack()
					return
				}

				// check every minute for job state
				time.Sleep(1 * time.Minute)

				job, err := jobsSvc.Get(h.DataflowCfg.Project, h.DataflowCfg.Region, jobID).Do()
				if err != nil {
					log.Errorf("Failed checking existing dataflow job with id %s: %v", jobID, err)
					msg.Nack()
					return
				}

				if waitStates[job.CurrentState] {
					// continue to wait and periodically check on job status
					continue

				} else if continueStates[job.CurrentState] {
					// use normal flow below to ack message and schedule next level if applicable
					jobDone = true
					jobInWaitState = false
					// exit this wait loop
					break
				} else {
					// assume non-recoverable failure, ack message, don't schedule any other levels
					log.Errorf("Dataflow job %s, %s found with unrecoverable state: %s ", job.Name, job.Id, job.CurrentState)
					msg.Ack()
					return
				}
			}
		}

		// no job with "queryId-level-helperId" name --> schedule --> if jobDone schedule next lvl
		var aggErr error
		if request.AggregationType == query.ConversionType {
			if hierarchicalConfig, err := query.ReadHierarchicalConfigFile(ctx, request.ExpandConfigURI); err == nil {
				aggErr = h.aggregatePartialReportHierarchical(ctx, request, hierarchicalConfig, jobDone)
			} else if directConfig, err := query.ReadDirectConfigFile(ctx, request.ExpandConfigURI); err == nil {
				if jobDone {
					log.Infof("query %q complete", request.QueryID)
				} else {
					aggErr = h.aggregatePartialReportDirect(ctx, request, directConfig)
				}
			} else if err := onepartyaggregator.ValidateTargetBuckets(ctx, request.ExpandConfigURI); err == nil {
				if jobDone {
					log.Infof("query %q complete", request.QueryID)
				} else {
					aggErr = h.aggregateOnepartyReport(ctx, request)
				}
			} else {
				log.Errorf("invalid expansion configuration in URI %s", request.ExpandConfigURI)
			}
		} else if request.AggregationType == query.ReachType {
			aggErr = h.aggregatePartialReportReach(ctx, request)
		} else {
			aggErr = fmt.Errorf("expect aggregation type 'reach' or 'conversion', got %q", request.AggregationType)
		}

		if aggErr != nil {
			log.Error(aggErr)
			msg.Nack()
			return
		}
		msg.Ack()
	})
}

func getFinalPartialResultURI(resultDir, queryID, origin string) string {
	return utils.JoinPath(resultDir, fmt.Sprintf("%s_%s", queryID, strings.ReplaceAll(origin, ".", "_")))
}

func (h *QueryHandler) runPipeline(ctx context.Context, binary string, args []string, request *query.AggregateRequest) error {
	if h.PipelineRunner == "dataflow" {
		args = append(args,
			"--project="+h.DataflowCfg.Project,
			"--region="+h.DataflowCfg.Region,
			"--temp_location="+h.DataflowCfg.TempLocation,
			"--staging_location="+h.DataflowCfg.StagingLocation,
			// set jobname to queryID-level-origin
			"--job_name="+fmt.Sprintf("%s-%v-%s", request.QueryID, request.QueryLevel, h.Origin),
			"--worker_binary="+h.ServerCfg.DpfAggregatePartialReportBinary,
		)
		// The zone of the worker pool. If not specified, Dataflow will pick one for the job.
		if h.DataflowCfg.Zone != "" {
			args = append(args,
				"--zone="+h.DataflowCfg.Zone,
			)
		}
		// The number of workers used at the beginning of a job. If not sepcified, one worker will be used.
		if request.NumWorkers > 0 {
			args = append(args,
				"--num_workers="+fmt.Sprint(request.NumWorkers),
			)
		}
		// The maximum number of workers allowed to be used during a job. If not specified, the maximum will be 1000 or depending on the quotas.
		if h.DataflowCfg.MaxNumWorkers > 0 {
			args = append(args,
				"--max_num_workers="+strconv.Itoa(h.DataflowCfg.MaxNumWorkers),
			)
		}
		// The worker type. If not specified, the worker type will be 'e2-standard-2'.
		if h.DataflowCfg.WorkerMachineType != "" {
			args = append(args,
				"--worker_machine_type="+h.DataflowCfg.WorkerMachineType,
			)
		}
		// The service account managing workers. If not specified, the Compute Engine default service account will be used.
		if h.DataflowCfg.ServiceAccountEmail != "" {
			args = append(args,
				"--service_account_email="+h.DataflowCfg.ServiceAccountEmail,
			)
		}
	}

	str := binary
	for _, s := range args {
		str = fmt.Sprintf("%s\n%s", str, s)
	}
	log.Infof("Running command\n%s", str)

	cmd := exec.CommandContext(ctx, binary, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("err: %s, stderr: %s", err, stderr.String())
		return err
	}
	log.Infof("output of cmd: %s", out.String())
	return nil
}

func (h *QueryHandler) aggregatePartialReportHierarchical(ctx context.Context, request *query.AggregateRequest, config *query.HierarchicalConfig, jobDone bool) error {
	finalLevel := int32(len(config.PrefixLengths)) - 1
	if request.QueryLevel > finalLevel {
		return fmt.Errorf("expect request level <= finalLevel %d, got %d", finalLevel, request.QueryLevel)
	}
	if !jobDone {
		partialReportURI := request.PartialReportURI
		outputDecryptedReportURI := ""
		if request.QueryLevel > 0 {
			// If it is not the first-level aggregation, check if the result from the partner helper is ready for the previous level.
			exist, err := utils.IsGCSObjectExist(ctx, h.GCSClient,
				query.GetRequestPartialResultURI(request.PartnerSharedInfo.SharedDir, request.QueryID, request.QueryLevel-1),
			)
			if err != nil {
				return err
			}
			if !exist {
				// When the partial result from the partner helper is not ready, nack the message with an error.
				return fmt.Errorf("result from %s for level %d of query %s is not ready", request.PartnerSharedInfo.Origin, request.QueryLevel-1, request.QueryID)
			}

			// If it is not the first-level aggregation, the pipeline should read the decrypted reports instead of the original encrypted ones.
			partialReportURI = query.GetRequestDecryptedReportURI(h.ServerCfg.WorkspaceURI, request.QueryID)
		} else {
			outputDecryptedReportURI = query.GetRequestDecryptedReportURI(h.ServerCfg.WorkspaceURI, request.QueryID)
		}

		expandParamsURI, err := query.GetRequestExpandParamsURI(ctx, config, request,
			h.ServerCfg.WorkspaceURI,
			h.SharedDir,
			request.PartnerSharedInfo.SharedDir,
		)
		if err != nil {
			return err
		}

		var outputResultURI string
		// The final-level results are not supposed to be shared with the partner helpers.
		if request.QueryLevel == finalLevel {
			outputResultURI = getFinalPartialResultURI(request.ResultDir, request.QueryID, h.Origin)
		} else {
			outputResultURI = query.GetRequestPartialResultURI(h.SharedDir, request.QueryID, request.QueryLevel)
		}

		args := []string{
			"--partial_report_uri=" + partialReportURI,
			"--expand_parameters_uri=" + expandParamsURI,
			"--partial_histogram_uri=" + outputResultURI,
			"--decrypted_report_uri=" + outputDecryptedReportURI,
			"--epsilon=" + fmt.Sprintf("%f", request.TotalEpsilon*config.PrivacyBudgetPerPrefix[request.QueryLevel]),
			"--private_key_params_uri=" + h.ServerCfg.PrivateKeyParamsURI,
			"--key_bit_size=" + fmt.Sprint(request.KeyBitSize),
			"--runner=" + h.PipelineRunner,
		}

		if err := h.runPipeline(ctx, h.ServerCfg.DpfAggregatePartialReportBinary, args, request); err != nil {
			return err
		}
	}

	if request.QueryLevel == finalLevel {
		log.Infof("query %q complete", request.QueryID)
		return nil
	}

	// If the hierarchical query is not finished yet, publish the requests for the next-level aggregation.
	request.QueryLevel++
	_, topic, err := utils.ParsePubSubResourceName(h.RequestPubSubTopic)
	if err != nil {
		return err
	}
	return utils.PublishRequest(ctx, h.PubSubTopicClient, topic, request)
}

func (h *QueryHandler) aggregatePartialReportReach(ctx context.Context, request *query.AggregateRequest) error {
	outputResultURI := getFinalPartialResultURI(request.ResultDir, request.QueryID, h.Origin)
	outputValidityURI := utils.JoinPath(request.ResultDir, fmt.Sprintf("%s_%s_validity", request.QueryID, strings.ReplaceAll(h.Origin, ".", "_")))
	args := []string{
		"--partial_report_uri=" + request.PartialReportURI,
		"--partial_histogram_uri=" + outputResultURI,
		"--partial_validity_uri=" + outputValidityURI,
		"--private_key_params_uri=" + h.ServerCfg.PrivateKeyParamsURI,
		"--key_bit_size=" + fmt.Sprint(request.KeyBitSize),
		"--runner=" + h.PipelineRunner,
	}

	if err := h.runPipeline(ctx, h.ServerCfg.DpfAggregateReachPartialReportBinary, args, request); err != nil {
		return err
	}

	log.Infof("query %q complete", request.QueryID)
	return nil
}

func (h *QueryHandler) aggregatePartialReportDirect(ctx context.Context, request *query.AggregateRequest, config *query.DirectConfig) error {
	expandParamsURI := utils.JoinPath(h.ServerCfg.WorkspaceURI, fmt.Sprintf("%s_%s", request.QueryID, query.DefaultExpandParamsFile))
	if err := dpfaggregator.SaveExpandParameters(ctx, &dpfaggregator.ExpandParameters{
		Level:           request.KeyBitSize - 1,
		Prefixes:        config.BucketIDs,
		DirectExpansion: true,
		PreviousLevel:   -1,
	}, expandParamsURI); err != nil {
		return err
	}

	outputResultURI := getFinalPartialResultURI(request.ResultDir, request.QueryID, h.Origin)
	args := []string{
		"--partial_report_uri=" + request.PartialReportURI,
		"--expand_parameters_uri=" + expandParamsURI,
		"--partial_histogram_uri=" + outputResultURI,
		"--epsilon=" + fmt.Sprintf("%f", request.TotalEpsilon),
		"--private_key_params_uri=" + h.ServerCfg.PrivateKeyParamsURI,
		"--key_bit_size=" + fmt.Sprint(request.KeyBitSize),
		"--runner=" + h.PipelineRunner,
	}

	if err := h.runPipeline(ctx, h.ServerCfg.DpfAggregatePartialReportBinary, args, request); err != nil {
		return err
	}

	log.Infof("query %q complete", request.QueryID)
	return nil
}

func (h *QueryHandler) aggregateOnepartyReport(ctx context.Context, request *query.AggregateRequest) error {
	outputResultURI := getFinalPartialResultURI(request.ResultDir, request.QueryID, h.Origin)
	args := []string{
		"--encrypted_report_uri=" + request.PartialReportURI,
		"--target_bucket_uri=" + request.ExpandConfigURI,
		"--partial_histogram_uri=" + outputResultURI,
		"--private_key_params_uri=" + h.ServerCfg.PrivateKeyParamsURI,
		"--epsilon=" + fmt.Sprintf("%f", request.TotalEpsilon),
		"--runner=" + h.PipelineRunner,
	}

	if err := h.runPipeline(ctx, h.ServerCfg.OnepartyAggregateReportBinary, args, request); err != nil {
		return err
	}

	log.Infof("query %q complete", request.QueryID)
	return nil
}

// ReadHelperSharedInfo reads the helper shared info from a URL.
func ReadHelperSharedInfo(client *http.Client, url, token string) (*query.HelperSharedInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Status != "200 OK" {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Infof("%v: %s", resp.Status, string(body))
		return nil, fmt.Errorf("Error reading shared info from %s: %s", url, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	info := &query.HelperSharedInfo{}
	if err := json.Unmarshal([]byte(body), info); err != nil {
		return nil, err
	}
	return info, nil
}
