# Shuffle Randomness Report — Data Appendix

Generated: 2026-06-21T03:00:00+02:00
Report date: 2026-06-21
Data appendix for [20260621.md](20260621.md)

## Alice (u1)

### Playlist: Favourites (p1)

#### Snapshot: snap1
- **Captured at**: 2026-06-20T12:00:00+02:00
- **Categories (k)**: 4
- **Total plays (N)**: 8
- **Filters**: shuffle only, skipped plays included
- **Track test**: χ² = 7.00, df = 3, p = 0.0719, V = 0.5401
- **Residual threshold**: |residual| > 3

**Full per-track table**

| Track ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| t1 | Track One | 5 | 2.00 | 4.5000 | +2.12 | No | +150.00% |
| t4 | Track Four | 0 | 2.00 | 2.0000 | -1.41 | No | -100.00% |
| t3 | Track Three | 1 | 2.00 | 0.5000 | -0.71 | No | -50.00% |
| t2 | Track Two | 2 | 2.00 | 0.0000 | +0.00 | No | +0.00% |

---

#### Snapshot: snap2
- **Captured at**: 2026-06-20T14:30:00+02:00
- **Categories (k)**: 3
- **Total plays (N)**: 2
- **Filters**: shuffle only, skipped plays included
- **Skipped**: total plays < min_plays

**Full per-track table**

| Track ID | Name | Observed | Expected |
|---|---|---|---|
| t1 | Track One | 1 | 0.67 |
| t2 | Track Two | 1 | 0.67 |
| t3 | Track Three | 0 | 0.67 |

---

#### Snapshot: snap4
- **Captured at**: 2026-06-20T12:00:00+02:00
- **Categories (k)**: 4
- **Total plays (N)**: 12
- **Filters**: shuffle only, skipped plays included
- **Track test**: χ² = 10.00, df = 3, p = 0.0186, V = 0.5270
- **Artist test**: χ² = 5.00, df = 2, p = 0.0821, V = 0.4564
- **Residual threshold**: |residual| > 3

**Full per-track table**

| Track ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| t1 | Track One | 5 | 2.00 | 4.5000 | +2.12 | No | +150.00% |
| t4 | Track Four | 0 | 2.00 | 2.0000 | -1.41 | No | -100.00% |
| t3 | Track Three | 1 | 2.00 | 0.5000 | -0.71 | No | -50.00% |
| t2 | Track Two | 2 | 2.00 | 0.0000 | +0.00 | No | +0.00% |

**Full per-artist table**

| Artist ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| a1 | Artist One | 7 | 6.00 | 0.1667 | +0.41 | No | +16.67% |
| a2 | Artist Two | 3 | 4.00 | 0.2500 | -0.50 | No | -25.00% |
| a3 | Artist Three | 2 | 2.00 | 0.0000 | +0.00 | No | +0.00% |

---

## Bob (u2)

### Playlist: Discover Weekly (p2)

#### Snapshot: snap3
- **Captured at**: 2026-06-20T10:00:00+02:00
- **Categories (k)**: 2
- **Total plays (N)**: 35
- **Filters**: shuffle only, skipped plays included
- **Track test**: χ² = 0.10, df = 1, p = 0.7518, V = 0.0534
- **Artist test**: χ² = 0.05, df = 1, p = 0.8231, V = 0.0378
- **Residual threshold**: |residual| > 3

**Full per-track table**

| Track ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| t1 | Track A | 18 | 17.50 | 0.0143 | +0.12 | No | +2.86% |
| t2 | Track B | 17 | 17.50 | 0.0143 | -0.12 | No | -2.86% |

**Full per-artist table**

| Artist ID | Name | O | E | χ² contrib | Residual | Flagged | Deviation |
|---|---|---|---|---|---|---|---|
| a1 | Artist One | 18 | 17.50 | 0.0143 | +0.12 | No | +2.86% |
| a2 | Artist Two | 17 | 17.50 | 0.0143 | -0.12 | No | -2.86% |

---

**Notes**

- **Reproducibility:** All raw counts, expected counts, and test statistics are shown above. No database access is required to verify the results.
- **Chi-squared GOF:** χ² = Σ (O−E)²/E, df = k−1, p-value from regularized upper incomplete gamma function Q(df/2, χ²/2).
- **Cramér's V (GOF variant):** V = sqrt(χ² / (M · (k−1))), bounded [0,1]. Interpretation: 0.1 small, 0.3 medium, 0.5 large.
- **Standardized residual:** rᵢ = (Oᵢ−Eᵢ) / √Eᵢ ~ N(0,1) under H₀. Flagged if |rᵢ| > threshold.
- **Cumulative-to-date:** Analysis includes all plays up to the report time per (user, playlist, snapshot). Does not use a rolling window.
- **Per-snapshot granularity:** Each chi-squared test runs per individual (playlist, snapshot) pair for statistical correctness. Frequently-edited playlists with few plays per snapshot may produce many "insufficient data" (M < threshold) entries. No aggregate per-playlist fallback is applied.
- **Residual false-positive rate:** At |r| > 3, expect ~0.27% × k false flags per snapshot by chance under uniform shuffle (no multiple-comparison correction applied).
