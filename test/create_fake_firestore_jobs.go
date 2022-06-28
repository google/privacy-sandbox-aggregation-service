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

// This binary creates test job records in Firestore.
package main

import (
	"context"
	"flag"
	"math/rand"
	"time"

	log "github.com/golang/glog"
	"cloud.google.com/go/firestore"
	"github.com/pborman/uuid"
	"github.com/google/privacy-sandbox-aggregation-service/service/jobmonitor"
)

var (
	project   = flag.String("project", "", "GCP project ID.")
	succeeded = flag.Int("succeeded", 5, "Number of succeeded jobs.")
	pending   = flag.Int("pending", 5, "Number of pending jobs.")
	failed    = flag.Int("failed_count", 5, "Number of failed jobs.")
)

func main() {
	flag.Parse()

	if *project == "" {
		log.Exitf("empty project ID")
	}

	jobs := make(map[string]*jobmonitor.AggregationJob)
	for i := 0; i < *succeeded; i++ {
		id, job := createSucceededJob()
		jobs[id] = job
	}
	for i := 0; i < *pending; i++ {
		id, job := createPendingJob()
		jobs[id] = job
	}
	for i := 0; i < *failed; i++ {
		id, job := createFailJobOneFail()
		jobs[id] = job
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, *project)
	if err != nil {
		log.Fatalf("Failed to create a firestore client: %v", err)
	}
	defer client.Close()

	err = jobmonitor.WriteJobs(ctx, client, jobmonitor.TestPath, jobs)
	if err != nil {
		log.Fatalf("Failed adding job: %v", err)
	}
}

// randomTimestamp creates a random timestamp within a year
func randomTimestamp() time.Time {
	now := time.Now()
	then := now.Add(-1 * time.Hour * 24 * 365)
	randomTime := rand.Int63n(now.Unix()-then.Unix()) + then.Unix()
	return time.Unix(randomTime, 0)
}

func createSucceededJob() (string, *jobmonitor.AggregationJob) {
	randTime := randomTimestamp()
	return uuid.New(), &jobmonitor.AggregationJob{Aggregators: map[string]*jobmonitor.AggregatorJobs{
		"aggregator1": {LevelJobs: map[int]*jobmonitor.PipelineJob{
			0: {
				Created: randTime.Add(-20 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Add(-10 * time.Minute).Unix(),
			},
			1: {
				Created: randTime.Add(-9 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Unix(),
			},
		}},
		"aggregator2": {LevelJobs: map[int]*jobmonitor.PipelineJob{
			0: {
				Created: randTime.Add(-20 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Add(-10 * time.Minute).Unix(),
			},
			1: {
				Created: randTime.Add(-9 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Unix(),
			},
		}},
	}}
}

func createFailJobOneFail() (string, *jobmonitor.AggregationJob) {
	randTime := time.Now()
	return uuid.New(), &jobmonitor.AggregationJob{Aggregators: map[string]*jobmonitor.AggregatorJobs{
		"aggregator1": {LevelJobs: map[int]*jobmonitor.PipelineJob{
			0: {
				Created: randTime.Add(-20 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Add(-10 * time.Minute).Unix(),
			},
			1: {
				Created: randTime.Add(-9 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Unix(),
			},
		}},
		"aggregator2": {LevelJobs: map[int]*jobmonitor.PipelineJob{
			0: {
				Created: randTime.Add(-20 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Add(-10 * time.Minute).Unix(),
			},
			1: {
				Created: randTime.Add(-9 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "failed",
				Updated: randTime.Unix(),
			},
		}},
	}}
}

func createPendingJob() (string, *jobmonitor.AggregationJob) {
	randTime := time.Now()
	return uuid.New(), &jobmonitor.AggregationJob{Aggregators: map[string]*jobmonitor.AggregatorJobs{
		"aggregator1": {LevelJobs: map[int]*jobmonitor.PipelineJob{
			0: {
				Created: randTime.Add(-20 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Add(-10 * time.Minute).Unix(),
			},
			1: {
				Created: randTime.Add(-9 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Unix(),
			},
		}},
		"aggregator2": {LevelJobs: map[int]*jobmonitor.PipelineJob{
			0: {
				Created: randTime.Add(-20 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "finished",
				Updated: randTime.Add(-10 * time.Minute).Unix(),
			},
			1: {
				Created: randTime.Add(-9 * time.Minute).Unix(),
				Message: "TBD",
				Result:  "TBD",
				Status:  "pending",
				Updated: randTime.Unix(),
			},
		}},
	}}
}
