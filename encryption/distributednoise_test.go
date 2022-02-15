package distributednoise

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
	"github.com/grd/stat"
)

func TestGeometricMechanismNoise(t *testing.T) {
	const (
		numberOfSamples = 1e6
		tolerance       = 1e-2
		numNoiseShares  = 2
	)

	for _, tc := range []struct {
		l1Sensitivity uint64
		epsilon       float64
	}{
		{
			l1Sensitivity: 1,
			epsilon:       0.5,
		},
		{
			l1Sensitivity: 2,
			epsilon:       0.5,
		},
		{
			l1Sensitivity: 4,
			epsilon:       0.5,
		},
	} {
		p := math.Exp(-tc.epsilon / float64(tc.l1Sensitivity))
		wantMean := 0.0
		// TODO: Add a citation for the variance of symmetric geometric distribution.
		wantVariance := 2 * p / ((1.0 - p) * (1.0 - p))
		noisedSamples := make(stat.Float64Slice, numberOfSamples)
		for j := 0; j < numNoiseShares; j++ {
			for i := 0; i < numberOfSamples; i++ {
				noise, err := DistributedGeometricMechanismRand(tc.epsilon, tc.l1Sensitivity, numNoiseShares)
				if err != nil {
					t.Fatal(err)
				}
				noisedSamples[i] += float64(noise)
			}
		}
		gotMean, gotVariance := stat.Mean(noisedSamples), stat.Variance(noisedSamples)
		if !floats.EqualWithinAbsOrRel(gotMean, wantMean, tolerance, tolerance) {
			t.Errorf("Mean mismatch, want: %v, got: %v", wantMean, gotMean)
		}
		if !floats.EqualWithinAbsOrRel(gotVariance, wantVariance, tolerance, tolerance) {
			t.Errorf("Variance mismatch, want: %v, got: %v", wantVariance, gotVariance)
		}
	}
}
