# CS Insights

CS Insights parses Counter-Strike 2 demo files and surfaces actionable feedback on your gameplay habits. Rather than raw statistics, it focuses on detecting specific bad habits in gunfights and movement, with a clean web dashboard to review the results.

## Architecture

```
cs-insights/
├── backend/   Go engine (demo parser, analyzers, REST API, SQLite)
└── frontend/  SvelteKit dashboard
```

## Analyzers

All analyzers ignore events that occur after all enemies on the opposing team are dead, so end-of-round cleanups and irrelevant shots do not inflate the stats.

| Analyzer | What it detects |
|---|---|
| **Premature Firing** | Firing a rifle/SMG/heavy weapon while the crosshair is still 3–15° away from the target. Excludes Deagle, AWP, and Scout (misses on those are just misses). |
| **Aim Spasm** | High-variance, zig-zagging crosshair movement in the ~0.5 s window before you shoot. |
| **Spray Accuracy** | Continuous bursts of 7+ bullets with under 20% hit rate, or long-range sprays (>650 units). Excludes pistols and semi-auto weapons. |
| **Movement Error** | Firing while moving faster than the weapon's accuracy threshold. Excludes pistols and SMGs except the Deagle. |
| **Gunfight Tracker** | Tracks the full lifecycle of each duel: time-to-first-shot, time-to-damage, and crosshair placement at the moment an enemy entered your FOV. |

## Requirements

- [Go 1.24+](https://golang.org/doc/install) with a C compiler (for SQLite — install `gcc` via your package manager)
- [Node.js 20+](https://nodejs.org/)

## Getting started

### 1. Clone

```bash
git clone https://github.com/Miconen/cs-insights.git
cd cs-insights
```

### 2. Build the backend

```bash
cd backend
go build -o cs-insights ./cmd/cs-insights
```

### 3. Install frontend dependencies

```bash
cd frontend
npm install
```

## Running

From the repository root, `make` starts the backend API on port 8080:

```bash
make
```

In a second terminal, start the frontend dev server:

```bash
cd frontend
npm run dev
```

Open [http://localhost:5173](http://localhost:5173) in your browser.

## Parsing demos

### From a local file

```bash
cd backend
./cs-insights --demo=/path/to/match.dem --player="YourExactIngameName"
```

Or from the repository root:

```bash
make parse DEMO=/path/to/match.dem PLAYER="YourExactIngameName"
```

Parsing can take a minute depending on demo length.

### From your Steam match history (web UI)

Open the **Fetch Demos** page in the dashboard. Paste your `steamLoginSecure` cookie and Steam ID to list your recent Premier match replays, then click **Download & Analyze** for each one. The demo is downloaded, decompressed, and parsed automatically.

### Clearing previous data

```bash
make clear
```

## Configuration

Edit `backend/config.json` to tune analyzer thresholds without touching Go code:

```json
{
  "analyzers": {
    "premature_fire": {
      "max_engagement_angle": 15.0
    },
    "spasm": {
      "variance_threshold": 15.0,
      "min_zig_zags": 4
    },
    "counter_strafe": {
      "max_velocity_threshold": 34.0
    },
    "crosshair_height": {
      "max_vertical_distance": 10.0
    },
    "spray": {
      "long_range_threshold": 650.0
    }
  }
}
```

## Dashboard

The dashboard shows:

- **Coach's Advice** — aggregated feedback with exact counts per category
- **Habit Profile** — polar area chart of incident type distribution
- **Incident Log** — every flagged event with round, tick, severity, and a `demo_gototick` button to jump straight to it in the CS2 demo viewer. Events within the same round and 100 ticks of each other are grouped into a collapsible cluster.
