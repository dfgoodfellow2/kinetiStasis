# TUI Status

> **FROZEN — 2026-10-23.** The Bubble Tea TUI is a maintained-but-frozen secondary
> client. New features are PWA-only. The TUI stays compiling, vetted, and at its
> current feature parity; it is not being extended or removed.

## History

The TUI (`cmd/tui` + `ui/tui`) was the original client for Diet Tracker v2 and
drove the API surface during early development. When the Svelte 5 PWA matured
around May 2026, the PWA became the primary client and the TUI fell behind. In
October 2026 full PWA/TUI parity was deliberately restored — missing API client
methods were added, the weekly check-in cycle was brought over, the profile
macro-distribution selector landed, and target editing gained the eat-back
toggle. With parity restored, the owner froze the TUI: it remains useful as a
fast terminal client, but all future product work happens in the PWA.

## Current scope

The TUI is intentionally scoped to three jobs:

- **Logging** — meals (manual and AI-parsed), workouts, biometrics, body
  measurements, and daily nutrition/workout history.
- **Check-in** — weekly check-in preview, biometric submission, and recording
  the check-in with its recommended calorie target.
- **Glance** — dashboard summary (calories/macros, TDEE figures, raw weight-trend
  points as text), targets editing (including eat-back), body-fat calculator,
  and data export.

Supporting surfaces — login/register, profile settings (including the
`keto` / `low_carb` / `balanced` / `high_carb` macro-distribution selector), and
logout — exist to make those three jobs usable.

## Policy

- **New features are PWA-only by default.** Do not add screens, menu entries, or
  new workflows to `ui/tui` / `cmd/tui`.
- **The TUI must keep compiling and vetting.** `make test` runs `go vet ./...`
  and `go test ./...`, which covers the TUI packages; keep them green when
  shared types, client signatures, or dependencies change.
- **API client methods stay in parity where cheap.** When a new endpoint is
  added, adding the corresponding `ui/tui/client` method is acceptable and
  encouraged *if* it is low-cost and type-only. Wiring it into a screen is a
  feature and is out of scope.
- **No removal.** The TUI is not being deleted or deprecated; it is frozen at
  current parity. Fixes should be limited to keeping it working (build/toolchain
  drift, server contract changes, correctness bugs), not new behavior.
- **Documentation lives here.** Do not describe the TUI as a co-equal client in
  top-level docs; point to this file instead.

## Build and run

```bash
make tui                  # builds bin/diet-tracker-ui
./bin/diet-tracker-ui     # run against the default API URL

# point at a different server:
DIET_API_URL=http://localhost:8080 ./bin/diet-tracker-ui
```

See the README's "Running the TUI Client" section for the surrounding dev
setup (API server must be running).

## Known gaps (left as-is)

- **No saved-meal or meal-template screens.** The transport layer exists
  (`ListSaved`, `PostSaved`, `DeleteSaved`, `ListTemplates`, `PostTemplate`,
  `DeleteTemplate`), but there is no UI. Use the PWA for these.
- **No auto-distribution screen.** `DistributeMacros` exists as client
  transport; macro targets are edited manually (or via the profile
  distribution selector) in the TUI.
- **PWA-only insight features** — trend-weight insights (EMA trend level,
  kg/week rate, sparkline), the TDEE chart, and the streak ring are PWA-only by
  design. The TUI dashboard shows only plain-text summaries (observed/estimated
  TDEE, a list of weight-trend points); it has no charts.

These gaps are deliberate under the freeze. They are not bugs and are not
scheduled for TUI work.
