# Extract `ChiSquaredResult` / `RunChiSquared` abstraction

## Context

The `analyzeSnapshot` function inlines chi-squared computation twice (once for tracks, once for
artists). A shared `MinExpected float64` field + `trackExpectedOK bool` flag merge track and artist
expected-count checks into one ambiguous warning. Problems:

- `trackExpectedOK` name lies (set by artist analysis too)
- `MinExpected == 0` is a sentinel meaning "no warning" — overloaded zero value
- Single `MinExpected` field merges two independent tests' reliability into one generic warning
- The report can't tell the reader *which* test is questionable

## Design

### New type: `ChiSquaredResult` in `types.go`

```go
type ChiSquaredResult struct {
    Chi2        float64
    DF          int
    P           float64
    Effect      float64 // Cramér's V
    MinExpected float64 // always populated; check < 5 for warning
}
```

**Replace** on `Result`:
- Remove: `TrackChi2 *float64`, `TrackDF *int`, `TrackP *float64`, `TrackEffect *float64`
- Remove: `ArtistChi2 *float64`, `ArtistDF *int`, `ArtistP *float64`, `ArtistEffect *float64`
- Remove: `MinExpected float64`
- Add: `TrackTest *ChiSquaredResult`, `ArtistTest *ChiSquaredResult`

`nil` = test not run (skipped). Non-nil = full result. No zero-value sentinel.

`Skipped` and `SkipReason` fields remain on `Result` unchanged. When `Skipped == true`, both
`TrackTest` and `ArtistTest` are nil.

### New function: `RunChiSquared` in `chisq.go`

```go
func RunChiSquared(observed, expected []float64) (ChiSquaredResult, error) {
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
        M += int(o)
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
```

Caller wraps: `fmt.Errorf("track test: %w", err)` / `fmt.Errorf("artist test: %w", err)`.

### Simplify `analyzeSnapshot` in `analysis.go`

- Track path: `tr, err := RunChiSquared(intsToF64s(observed), expSlice)` → `result.TrackTest = &tr`
- Artist path: same pattern → `result.ArtistTest = &tr`
- Remove `trackExpectedOK` flag entirely
- Remove the `minExpected < 5` conditional blocks — `MinExpected` is always populated; the report
  layer decides whether to warn
- The `if result.Skipped { ... } else { ... }` guard around the artist analysis is preserved.
  When skipped, `ArtistRows` are populated with raw counts (no residuals) and `ArtistTest` stays nil.

### Row builders

`makeTrackRows` / `makeArtistRows` signatures unchanged. They are still called in the same code
path (when the test runs), but their call sites no longer interleave with inline chi-squared
computation — that logic moved into `RunChiSquared`.

### Update `report.go`

**Helpers** stay as field-level formatters — no coupling to `ChiSquaredResult`:

```go
maybeChi2(&r.TrackTest.Chi2)
maybeDF(&r.TrackTest.DF)
maybeP(&r.TrackTest.P)
maybeEffect(&r.TrackTest.Effect)
testVerdict(r.TrackTest)
```

**`trackBlockSummary`**: Add nil-guard at top (`if r.TrackTest == nil { return "" }`). After the
`**Result**: …` line, emit per-test warning:

```
> ⚠️ **Track test warning**: minimum expected count = X.XX (< 5), chi-squared result may be unreliable.
```

**`artistBlockSummary`**: Same pattern with `r.ArtistTest`.

**Remove** the merged `MinExpected > 0` blocks at lines 184–186 and 262–263.

**`snapshotBlockData`**: Adapt to new access pattern (`r.TrackTest.Chi2` instead of `*r.TrackChi2`).

## Test plan

| Test file | Change |
|---|---|
| `chisq_test.go` (new tests) | Table-driven `TestRunChiSquared`: happy path (k≥2), k=1 guard, error propagation (length mismatch, empty, zero expected, negative observed, M=0 via all-zero observed) |
| `analysis_test.go` | Replace assertions on scattered `*float64` fields with `snap1.TrackTest.Chi2` etc. Rename `TestAnalyze_MinExpectedZero` → `TestAnalyze_MinExpectedAtLeast5`, assert `r.TrackTest != nil && r.TrackTest.MinExpected >= 5.0` |
| `report_test.go` | Update test fixtures to use `ChiSquaredResult`. Add fixture where `TrackTest.MinExpected = 2.0`, `ArtistTest.MinExpected = 10.0` — verify warning only in track section |
| Golden files | Regenerated via `go test -update ./internal/report/` |

## Files touched

| File | Change |
|---|---|
| `internal/analysis/types.go` | Add `ChiSquaredResult`, remove scattered fields |
| `internal/analysis/chisq.go` | Add `RunChiSquared` |
| `internal/analysis/analysis.go` | Simplify `analyzeSnapshot`, remove `trackExpectedOK` |
| `internal/analysis/chisq_test.go` | Add `TestRunChiSquared` (new test) |
| `internal/analysis/analysis_test.go` | Update assertions, rename MinExpected test |
| `internal/report/report.go` | Update helpers, per-test warnings, nil-guards |
| `internal/report/report_test.go` | Update test data, add diverging-warning fixture |
| `internal/report/testdata/*.golden.md` | Regenerate |

## AI attribution

All modifications wrapped with `// 🤖 AI-start` / `// 🤖 AI-end` fences.
New tests file additions use `// 🤖 AI-generated` header (if creating separate test file) or
fences (if adding to existing).

## Verification

After all changes, run:

```
go vet ./...
go test ./...
```

Fix failures before proceeding.

## Benefits

- **One call site** for chi-squared math instead of two identical inline copies
- **No zero-value sentinel** — `*ChiSquaredResult` is nil when skipped
- **Per-test reliability warnings** — reader knows which test (track/artist) is questionable
- Adding a new dimension (albums, genres) = prep vectors + one `RunChiSquared` call
- Removes the lying `trackExpectedOK` name and the shared-state hack
