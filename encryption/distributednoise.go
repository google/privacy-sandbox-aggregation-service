// Package distributednoise generates random noise for the aggregation results.
package distributednoise

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat/distuv"
)

// polyaRand generates a random value that follows the Polya distribution.
func polyaRand(r, p float64) int64 {
	// The polya rand number can be drawn with a mixture of Gamma-Poisson distribution:
	// https://en.wikipedia.org/wiki/Negative_binomial_distribution
	gamma := distuv.Gamma{Alpha: r, Beta: (1 - p) / p}.Rand()
	return int64(distuv.Poisson{Lambda: gamma}.Rand())
}

// DistributedGeometricMechanismRand generates noise such that adding `numNoiseShares` separate
// samples drawn from this method added together will be distributed according to the two-sided
// geometric mechansim (aka Discrete Laplace distribution).
//
// For one-sided Geometric distribution (https://en.wikipedia.org/wiki/Geometric_distribution),
// we have: Geom(p) = Polya(1, 1 - p) = sum_i^numHelper Polya(1/i, p);
// By substracting two geometric random values, we can get the noise that follows two-sided distribution.
func DistributedGeometricMechanismRand(epsilon float64, l1Sensitivity, numNoiseShares uint64) (int64, error) {
	roundingResult := float64(numNoiseShares) * (1.0 / float64(numNoiseShares))
	if !floats.EqualWithinAbsOrRel(roundingResult, 1.0, 1e-6, 1e-6) {
		return 0, fmt.Errorf("rounding error, expect numNoiseShares*(1/numNoiseShares) == 1, got %v", roundingResult)
	}

	r, p := 1.0/float64(numNoiseShares), math.Exp(-epsilon/float64(l1Sensitivity))
	return polyaRand(r, p) - polyaRand(r, p), nil
}
