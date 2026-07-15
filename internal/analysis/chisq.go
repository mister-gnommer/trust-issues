package analysis

import (
	"errors"
	"fmt"
	"math"
)

const (
	maxIter = 200
	epsilon = 1e-15
)

func Statistic(observed []int, expected []float64) (chi2 float64, df int, err error) {
	if len(observed) != len(expected) {
		return 0, 0, fmt.Errorf("length mismatch: observed=%d expected=%d", len(observed), len(expected))
	}
	if len(observed) == 0 {
		return 0, 0, errors.New("empty slices")
	}
	if len(observed) == 1 {
		return 0, 0, nil
	}
	for i, obs := range observed {
		if obs < 0 {
			return 0, 0, fmt.Errorf("observed[%d] = %d < 0", i, obs)
		}
	}
	for i, exp := range expected {
		if exp <= 0 {
			return 0, 0, fmt.Errorf("expected[%d] = %g <= 0", i, exp)
		}
	}
	var sum float64
	for i, obs := range observed {
		diff := float64(obs) - expected[i]
		sum += diff * diff / expected[i]
	}
	return sum, len(observed) - 1, nil
}

func PValue(chi2 float64, df int) (float64, error) {
	if df < 1 {
		return 0, fmt.Errorf("df=%d < 1", df)
	}
	if chi2 < 0 {
		return 0, fmt.Errorf("chi2=%g < 0", chi2)
	}
	if math.IsNaN(chi2) || math.IsInf(chi2, 0) {
		return 0, fmt.Errorf("chi2=%g is not finite", chi2)
	}
	if chi2 == 0 {
		return 1.0, nil
	}
	s := float64(df) / 2
	x := chi2 / 2
	var p float64
	if x < s+1 {
		g, err := LowerRegGamma(s, x)
		if err != nil {
			return 0, fmt.Errorf("lower regularized gamma: %w", err)
		}
		p = 1 - g
	} else {
		g, err := UpperRegGamma(s, x)
		if err != nil {
			return 0, fmt.Errorf("upper regularized gamma: %w", err)
		}
		p = g
	}
	if p < 0 || p > 1 || math.IsNaN(p) {
		return 0, fmt.Errorf("p-value %g out of [0,1] (chi2=%g, df=%d)", p, chi2, df)
	}
	return p, nil
}

func LowerRegGamma(s, x float64) (float64, error) {
	if x == 0 {
		return 0, nil
	}
	lnGamma, _ := math.Lgamma(s)
	ap := s
	sum := 1.0 / s
	del := sum
	converged := false
	for i := 1; i <= maxIter; i++ {
		ap++
		del *= x / ap
		sum += del
		if math.Abs(del) < math.Abs(sum)*epsilon {
			converged = true
			break
		}
	}
	if !converged {
		return 0, fmt.Errorf("lower regularized gamma: series did not converge in %d iterations (s=%g, x=%g)", maxIter, s, x)
	}
	return math.Exp(s*math.Log(x)-x-lnGamma) * sum, nil
}

func UpperRegGamma(s, x float64) (float64, error) {
	if x == 0 {
		return 1, nil
	}
	lnGamma, _ := math.Lgamma(s)

	b := x + 1 - s
	c := 1.0 / 1e-30
	d := 1.0 / b
	if d == 0 {
		d = 1.0 / 1e-30
	}
	cf := d

	converged := false
	for i := 1; i <= maxIter; i++ {
		a := float64(i) * (s - float64(i))
		b += 2
		d = 1.0 / (a*d + b)
		if d == 0 {
			d = 1.0 / 1e-30
		}
		c = b + a/c
		if c == 0 {
			c = 1.0 / 1e-30
		}
		del := d * c
		cf *= del
		if math.Abs(del-1) < epsilon {
			converged = true
			break
		}
	}
	if !converged {
		return 0, fmt.Errorf("upper regularized gamma: continued fraction did not converge in %d iterations (s=%g, x=%g)", maxIter, s, x)
	}

	return math.Exp(s*math.Log(x)-x-lnGamma) * cf, nil
}

func EffectSize(chi2 float64, M int, k int) (float64, error) {
	if chi2 < 0 {
		return 0, fmt.Errorf("chi2=%g < 0", chi2)
	}
	if M <= 0 {
		return 0, fmt.Errorf("M=%d <= 0", M)
	}
	if k < 2 {
		return 0, fmt.Errorf("k=%d < 2", k)
	}
	denom := float64(M) * float64(k-1)
	v := math.Sqrt(chi2 / denom)
	// Clamp to 1.0 as a guard against numerical issues;
	// mathematically Cramér's V is bounded [0,1] for GOF.
	if v > 1 {
		v = 1
	}
	return v, nil
}

func RunChiSquared(observed []int, expected []float64) (ChiSquaredResult, error) {
	if len(observed) < 2 {
		return ChiSquaredResult{}, fmt.Errorf("need at least 2 categories, got %d", len(observed))
	}
	chi2, df, err := Statistic(observed, expected)
	if err != nil {
		return ChiSquaredResult{}, fmt.Errorf("statistic: %w", err)
	}
	p, err := PValue(chi2, df)
	if err != nil {
		return ChiSquaredResult{}, fmt.Errorf("p-value: %w", err)
	}
	M := 0
	for _, o := range observed {
		M += o
	}
	effect, err := EffectSize(chi2, M, len(observed))
	if err != nil {
		return ChiSquaredResult{}, fmt.Errorf("effect size: %w", err)
	}
	minExpected := expected[0]
	for _, e := range expected[1:] {
		if e < minExpected {
			minExpected = e
		}
	}
	return ChiSquaredResult{Chi2: chi2, DF: df, P: p, Effect: effect, MinExpected: minExpected}, nil
}

func Residual(observed int, expected float64) (float64, error) {
	if observed < 0 {
		return 0, fmt.Errorf("observed=%d < 0", observed)
	}
	if expected <= 0 {
		return 0, fmt.Errorf("expected=%g <= 0", expected)
	}
	return (float64(observed) - expected) / math.Sqrt(expected), nil
}
