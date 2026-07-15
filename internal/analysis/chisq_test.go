package analysis

import (
	"fmt"
	"math"
	"testing"
)

func TestStatistic_lengthMismatch(t *testing.T) {
	_, _, err := Statistic([]int{1, 2}, []float64{1})
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}
}

func TestStatistic_empty(t *testing.T) {
	_, _, err := Statistic(nil, nil)
	if err == nil {
		t.Fatal("expected error for empty slices")
	}
}

func TestStatistic_zeroExpected(t *testing.T) {
	_, _, err := Statistic([]int{10, 20}, []float64{5, 0})
	if err == nil {
		t.Fatal("expected error for zero expected")
	}
}

func TestStatistic_singleElement(t *testing.T) {
	chi2, df, err := Statistic([]int{10}, []float64{5})
	if err != nil {
		t.Fatal(err)
	}
	if chi2 != 0 || df != 0 {
		t.Errorf("got chi2=%g df=%d, want 0, 0", chi2, df)
	}
}

func TestStatistic_knownValues(t *testing.T) {
	cases := []struct {
		name     string
		obs      []int
		exp      []float64
		wantChi2 float64
		wantDF   int
	}{
		{
			name:     "no difference",
			obs:      []int{10, 10, 10},
			exp:      []float64{10, 10, 10},
			wantChi2: 0,
			wantDF:   2,
		},
		{
			name:     "simple deviation",
			obs:      []int{15, 5},
			exp:      []float64{10, 10},
			wantChi2: (5.0 * 5.0 / 10.0) * 2,
			wantDF:   1,
		},
		{
			name:     "three categories",
			obs:      []int{25, 50, 25},
			exp:      []float64{33.3, 33.3, 33.3},
			wantChi2: 12.5126,
			wantDF:   2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chi2, df, err := Statistic(tc.obs, tc.exp)
			if err != nil {
				t.Fatal(err)
			}
			if df != tc.wantDF {
				t.Errorf("df=%d, want %d", df, tc.wantDF)
			}
			if math.Abs(chi2-tc.wantChi2) > 0.01 {
				t.Errorf("chi2=%g, want %g", chi2, tc.wantChi2)
			}
		})
	}
}

func TestPValue_errors(t *testing.T) {
	_, err := PValue(1, 0)
	if err == nil {
		t.Error("expected error for df=0")
	}
	_, err = PValue(-1, 1)
	if err == nil {
		t.Error("expected error for chi2<0")
	}
	_, err = PValue(math.NaN(), 1)
	if err == nil {
		t.Error("expected error for NaN chi2")
	}
	_, err = PValue(math.Inf(1), 1)
	if err == nil {
		t.Error("expected error for Inf chi2")
	}
}

func TestPValue_zeroChi2(t *testing.T) {
	p, err := PValue(0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if p != 1.0 {
		t.Errorf("p=%g, want 1.0", p)
	}
}

func TestPValue_knownValues(t *testing.T) {
	cases := []struct {
		chi2  float64
		df    int
		wantP float64
		tol   float64
	}{
		{3.841, 1, 0.05, 0.01},
		{6.635, 1, 0.01, 0.005},
		{5.991, 2, 0.05, 0.01},
		{9.210, 2, 0.01, 0.005},
		{7.815, 3, 0.05, 0.01},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("chi2=%.3f_df=%d", tc.chi2, tc.df), func(t *testing.T) {
			p, err := PValue(tc.chi2, tc.df)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(p-tc.wantP) > tc.tol {
				t.Errorf("PValue(%g, %d) = %g, want ~%g", tc.chi2, tc.df, p, tc.wantP)
			}
		})
	}
}

func TestPValue_largeChi2(t *testing.T) {
	p, err := PValue(100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if p > 1e-10 {
		t.Errorf("p=%g should be very small for large chi2", p)
	}
}

func TestEffectSize_errors(t *testing.T) {
	_, err := EffectSize(10, 0, 3)
	if err == nil {
		t.Error("expected error for M=0")
	}
	_, err = EffectSize(10, 50, 1)
	if err == nil {
		t.Error("expected error for k=1")
	}
}

func TestEffectSize_cramersV(t *testing.T) {
	cases := []struct {
		chi2  float64
		M, k  int
		wantV float64
	}{
		{0, 100, 5, 0},
		{10, 50, 5, math.Sqrt(10.0 / (50 * 4))},
		{40, 20, 3, math.Sqrt(40.0 / (20 * 2))},
	}
	for _, tc := range cases {
		v, err := EffectSize(tc.chi2, tc.M, tc.k)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(v-tc.wantV) > 1e-10 {
			t.Errorf("V=%g, want %g", v, tc.wantV)
		}
	}
}

func TestEffectSize_clampsToOne(t *testing.T) {
	v, err := EffectSize(1000, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if v > 1 {
		t.Errorf("V=%g > 1", v)
	}
}

func TestResidual_errors(t *testing.T) {
	_, err := Residual(10, 0)
	if err == nil {
		t.Error("expected error for expected=0")
	}
	_, err = Residual(10, -1)
	if err == nil {
		t.Error("expected error for negative expected")
	}
}

func TestResidual_knownValues(t *testing.T) {
	cases := []struct {
		obs   int
		exp   float64
		wantR float64
	}{
		{10, 5, (10 - 5) / math.Sqrt(5)},
		{5, 10, (5 - 10) / math.Sqrt(10)},
		{0, 5, (0 - 5) / math.Sqrt(5)},
		{25, 25, 0},
	}
	for _, tc := range cases {
		r, err := Residual(tc.obs, tc.exp)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(r-tc.wantR) > 1e-10 {
			t.Errorf("r=%g, want %g", r, tc.wantR)
		}
	}
}

func TestLowerRegGamma_edgeCases(t *testing.T) {
	got, err := LowerRegGamma(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("LowerRegGamma(1,0) = %g, want 0", got)
	}
}

func TestUpperRegGamma_edgeCases(t *testing.T) {
	got, err := UpperRegGamma(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Errorf("UpperRegGamma(1,0) = %g, want 1", got)
	}
}

func TestPValue_extremeValues(t *testing.T) {
	t.Run("extreme chi2 underflows to 0", func(t *testing.T) {
		p, err := PValue(1e6, 5)
		if err != nil {
			t.Fatal(err)
		}
		if p >= 1e-300 {
			t.Errorf("PValue(1e6, 5) = %e, want < 1e-300", p)
		}
	})

	t.Run("tiny chi2 evaluates to 1", func(t *testing.T) {
		p, err := PValue(1e-10, 5)
		if err != nil {
			t.Fatal(err)
		}
		if p != 1.0 {
			t.Errorf("PValue(1e-10, 5) = %g, want 1.0", p)
		}
	})
}

func TestPValue_dispatchSwitches(t *testing.T) {
	p1, err := PValue(1, 5)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := PValue(20, 5)
	if err != nil {
		t.Fatal(err)
	}
	if p1 <= 0 || p1 > 1 {
		t.Errorf("small chi2 p=%g out of range", p1)
	}
	if p2 <= 0 || p2 > 1 {
		t.Errorf("large chi2 p=%g out of range", p2)
	}
}

func TestRunChiSquared(t *testing.T) {
	t.Run("happy path k>=2", func(t *testing.T) {
		res, err := RunChiSquared([]int{15, 5}, []float64{10, 10})
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(res.Chi2-5.0) > 0.01 {
			t.Errorf("Chi2=%g, want 5.0", res.Chi2)
		}
		if res.DF != 1 {
			t.Errorf("DF=%d, want 1", res.DF)
		}
		if res.P < 0.02 || res.P > 0.03 {
			t.Errorf("P=%g, want ~0.0253", res.P)
		}
		if res.Effect <= 0 {
			t.Errorf("Effect=%g, want >0", res.Effect)
		}
		if res.MinExpected != 10.0 {
			t.Errorf("MinExpected=%g, want 10.0", res.MinExpected)
		}
	})

	t.Run("k=1 rejected", func(t *testing.T) {
		_, err := RunChiSquared([]int{10}, []float64{5})
		if err == nil {
			t.Fatal("expected error for k=1")
		}
	})

	t.Run("error propagation", func(t *testing.T) {
		// Errors from Statistic are surfaced through RunChiSquared.
		_, err := RunChiSquared([]int{1, 2}, []float64{1})
		if err == nil {
			t.Fatal("expected error for length mismatch")
		}
	})

	t.Run("all-zero observed (M=0)", func(t *testing.T) {
		_, err := RunChiSquared([]int{0, 0}, []float64{2, 2})
		if err == nil {
			t.Fatal("expected error for M=0")
		}
	})

	t.Run("min expected computed correctly", func(t *testing.T) {
		res, err := RunChiSquared([]int{5, 10, 15}, []float64{8, 12, 10})
		if err != nil {
			t.Fatal(err)
		}
		if res.MinExpected != 8.0 {
			t.Errorf("MinExpected=%g, want 8.0", res.MinExpected)
		}
	})
}
