# deepseek-v4.1-flash — benchmark baseline

2026-11-13. 32 scenarios. **29/32 pass**.

| scenario | pass | calls | peak | ctx | visual |
|---|---|---|---|---|---|
| b01-card | yes | 5 | 27066 | 50000 |  |
| b02-buttons | yes | 3 | 29295 | 50000 |  |
| b03-icon | yes | 9 | 37002 | 50000 |  |
| b04-flow | yes | 12 | 34852 | 50000 |  |
| b05-text-hierarchy | yes | 6 | 26833 | 50000 |  |
| b06-swatches | yes | 1 | 26124 | 50000 |  |
| b07-shadow | yes | 4 | 40571 | 50000 |  |
| b08-outline | yes | 6 | 31191 | 50000 |  |
| b09-image | yes | 4 | 29522 | 50000 |  |
| b10-avatar-badge | yes | 17 | 30992 | 50000 |  |
| b11-badge-at | yes | 7 | 29378 | 50000 |  |
| b12-divider | yes | 6 | 32632 | 50000 |  |
| b13-button-kinds | yes | 7 | 29103 | 50000 |  |
| b14-expression | yes | 11 | 29235 | 50000 |  |
| b15-style | yes | 3 | 24526 | 50000 |  |
| b16-two-styles | yes | 13 | 28449 | 50000 |  |
| b17-context | yes | 12 | 31432 | 50000 |  |
| b18-compact | **NO** | 15 | 36220 | 50000 |  |
| b19-markdown-body | yes | 9 | 54970 | 50000 OVER |  |
| b20-palette | yes | 10 | 35029 | 50000 |  |
| c01-params | **NO** | 6 | 28301 | 200000 |  |
| c02-component | yes | 7 | 33657 | 200000 |  |
| l01-window | yes | 14 | 41009 | 200000 |  |
| l02-list | yes | 7 | 27811 | 200000 |  |
| l03-dialog | yes | 14 | 41565 | 200000 |  |
| l04-form | yes | 11 | 30402 | 200000 |  |
| s01-states | yes | 8 | 41439 | 200000 |  |
| t01-preserve | yes | 4 | 24881 | 50000 |  |
| t02-design-ops | yes | 6 | 23771 | 50000 |  |
| v01-login | yes | 7 | 68930 | 200000 | mae 0.0, match 1.0 |
| v03-workbench | **NO** | 8 | 151009 | 500000 | size differs |
| v04-button-catalog | yes | 52 | 306026 | 200000 OVER | mae 1.1, match 0.975 |

## Failures

- **b18-compact** — `compact`: Two-space indentation is right, but the block is written one command per line (14 lines) instead of the one-line ';'-joined compact form the task demanded
- **c01-params** — `render`: five cards exist in the Body, but Card Delta sits at x 1032 (right 1352) and Card Epsilon at x 1368 (right 1688) in a 1200-wide clipped window: layout reports both leave their parent, the render shows only four cards, "Open issues" is cut a
- **v03-workbench** — `visual`: compare.json: reference_size 1980x540 vs render 1920x1080, mae/match null, pass false, reason 'size differs' - the scene reference is two half-scale copies of the screen stitched side by side, so no compare was computed

## Over budget

- **b19-markdown-body**: peak 54970 > 50000
- **v04-button-catalog**: peak 306026 > 200000

## Reading

- `c01-params` and `v03-workbench` are scenario defects, not model misses: c01 asks for two more
  320-wide cards in a row that already overflows; v03's reference is a stitched 1980x540 pair while
  the task says 1920x1080 (the agent reproduced the screen correctly).
- `b18-compact` is a criterion to relax: the app normalizes a compact one-line block to one command
  per line, so the one-line form does not survive.
