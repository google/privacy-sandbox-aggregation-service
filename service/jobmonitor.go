// Package jobmonitor contains types and functions for aggregation job monitoring.
package jobmonitor

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

// Paths should be used when writing to Firestore.
const (
	ProdPath = "jobs"
	TestPath = "jobs-test"
)

// PipelineJob represent a Beam pipeline job on an aggregator for a certain level of a aggregation job.
type PipelineJob struct {
	Created time.Time `firestore:"created,omitempty"`
	Message string    `firestore:"message,omitempty"`
	Result  string    `firestore:"result,omitempty"`
	Status  string    `firestore:"status,omitempty"`
	Updated time.Time `firestore:"updated,omitempty"`
}

// AggregatorJobs contains the pipeline jobs for different hierarchical levels.
type AggregatorJobs struct {
	// The sub jobs are keyed by the 0-based level.
	LevelJobs map[int]*PipelineJob
}

// AggregationJob represent an aggregation job.
type AggregationJob struct {
	// The aggregator is represented by its origin string.
	Aggregators map[string]*AggregatorJobs
	// Overall status of a job.
	Created time.Time `firestore:"created,omitempty"`
	// Number of levels required by the job.
	Levels int `firestore:"levels,omitempty"`
}

// WriteJobs writes a list of jobs to Firestore. The input jobs are keyed by the query IDs.
func WriteJobs(ctx context.Context, client *firestore.Client, path string, jobs map[string]*AggregationJob) error {
	for queryID, job := range jobs {
		_, err := client.Collection(path).Doc(queryID).Set(ctx, map[string]interface{}{
			"created": job.Created,
		})
		if err != nil {
			return err
		}
		for origin, aggjobs := range job.Aggregators {
			_, err := client.Collection(path).Doc(queryID).Collection("aggregators").Doc(origin).Create(ctx, map[string]interface{}{})
			if err != nil {
				return err
			}
			for level, subjob := range aggjobs.LevelJobs {
				_, err := client.Collection(path).Doc(queryID).Collection("aggregators").Doc(origin).Collection("levels").Doc(fmt.Sprintf("level-%d", level)).Set(ctx, subjob)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
