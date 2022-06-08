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
	projectID      = flag.String("project_id", "", "GCP project ID.")
	succeededCount = flag.Int("succeeded_count", 0, "Number of succeeded jobs.")
	pendingCount   = flag.Int("pending_count", 0, "Number of pending jobs.")
	failedCount    = flag.Int("failed_count", 0, "Number of failed jobs.")
)

func main() {
	flag.Parse()

	jobs := make(map[string]*jobmonitor.AggregationJob)
	for i := 0; i < *succeededCount; i++ {
		id, job := createSucceededJob()
		jobs[id] = job
	}
	for i := 0; i < *pendingCount; i++ {
		id, job := createPendingJob()
		jobs[id] = job
	}
	for i := 0; i < *failedCount; i++ {
		id, job := createFailJobOneFail()
		jobs[id] = job
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, *projectID)
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
