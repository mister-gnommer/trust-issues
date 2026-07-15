# Shuffle Randomness Report

Generated: 2026-07-08T14:30:00+02:00
Report date: 2026-07-08
Full data: [20260708-data.md](20260708-data.md)

## Alice (user_1)

### Playlist: Favourites (pl_123)

Snapshots analyzed: 2

#### Snapshot: snap_a1b2
- **Captured at**: 2026-07-01T12:00:00+02:00
- **Categories (k)**: 5
- **Total plays (N)**: 50
- **Filters**: shuffle only, skipped plays included

##### Track test
- χ² = 8.20, df = 4, p = 0.0840, V = 0.2020
- **Result**: not flagged (p >= 0.01)

###### Flagged tracks (|residual| > 3)

No tracks flagged by residual.

###### Top 10 tracks (by χ² contribution)

| Track ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| t1 | Bohemian Rhapsody | 14 | 10.00 | 1.6000 | +1.26 | No | +40.00% |
| t2 | Stairway to Heaven | 12 | 10.00 | 0.4000 | +0.63 | No | +20.00% |
| t3 | Hotel California | 10 | 10.00 | 0.0000 | +0.00 | No | +0.00% |
| t4 | Imagine | 8 | 10.00 | 0.4000 | -0.63 | No | -20.00% |
| t5 | Yesterday | 6 | 10.00 | 1.6000 | -1.26 | No | -40.00% |

##### Artist test
- χ² = 3.50, df = 3, p = 0.3210, V = 0.1530
- **Result**: not flagged (p >= 0.01)

###### Flagged artists (|residual| > 3)

No artists flagged by residual.

###### Top 10 artists (by χ² contribution)

| Artist ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| a1 | Queen | 14 | 12.50 | 0.1800 | +0.42 | No | +12.00% |
| a2 | Led Zeppelin | 12 | 12.50 | 0.0200 | -0.14 | No | -4.00% |
| a3 | Eagles | 10 | 12.50 | 0.5000 | -0.71 | No | -20.00% |
| a4 | The Beatles | 14 | 12.50 | 0.1800 | +0.42 | No | +12.00% |

Full tables: [20260708-data.md](20260708-data.md)

#### Snapshot: snap_c3d4
- **Captured at**: 2026-07-05T14:30:00+02:00
- **Categories (k)**: 3
- **Total plays (N)**: 15
- **Filters**: shuffle only, skipped plays included

##### Track test
- χ² = 12.40, df = 2, p = 0.0020, V = 0.6430
- **Result**: flagged (p < 0.01)

###### Flagged tracks (|residual| > 3)

No tracks flagged by residual.

###### Top 10 tracks (by χ² contribution)

| Track ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| t1 | Bohemian Rhapsody | 10 | 5.00 | 5.0000 | +2.24 | No | +100.00% |
| t3 | Hotel California | 3 | 5.00 | 0.8000 | -0.89 | No | -40.00% |
| t5 | Yesterday | 2 | 5.00 | 1.8000 | -1.34 | No | -60.00% |

Full tables: [20260708-data.md](20260708-data.md)

## Bob (user_2)

### Playlist: Workout Mix (pl_456)

Snapshots analyzed: 1

#### Snapshot: snap_e5f6
- **Captured at**: 2026-07-01T12:00:00+02:00
- **Categories (k)**: 2
- **Total plays (N)**: 30
- **Filters**: shuffle only, skipped plays included

##### Track test
- χ² = 0.53, df = 1, p = 0.4670, V = 0.1330
- **Result**: not flagged (p >= 0.01)

###### Flagged tracks (|residual| > 3)

No tracks flagged by residual.

###### Top 10 tracks (by χ² contribution)

| Track ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| t10 | Eye of the Tiger | 16 | 15.00 | 0.0670 | +0.26 | No | +6.67% |
| t11 | Lose Yourself | 14 | 15.00 | 0.0670 | -0.26 | No | -6.67% |

Full tables: [20260708-data.md](20260708-data.md)

---

**Notes**

- **Reproducibility:** All raw counts, expected counts, and test statistics are shown above. No database access is required to verify the results.
- **Chi-squared GOF:** χ² = Σ (O−E)²/E, df = k−1, p-value from regularized upper incomplete gamma function Q(df/2, χ²/2).
- **Cramér's V (GOF variant):** V = sqrt(χ² / (M · (k−1))), bounded [0,1]. Interpretation: 0.1 small, 0.3 medium, 0.5 large.
- **Standardized residual:** rᵢ = (Oᵢ−Eᵢ) / √Eᵢ ~ N(0,1) under H₀. Flagged if |rᵢ| > threshold.
- **Cumulative-to-date:** Analysis includes all plays up to the report time per (user, playlist, snapshot). Does not use a rolling window.
- **Per-snapshot granularity:** Each chi-squared test runs per individual (playlist, snapshot) pair for statistical correctness. Frequently-edited playlists with few plays per snapshot may produce many "insufficient data" (M < threshold) entries. No aggregate per-playlist fallback is applied.
- **Residual false-positive rate:** At |r| > 3, expect ~0.27% × k false flags per snapshot by chance under uniform shuffle (no multiple-comparison correction applied).
