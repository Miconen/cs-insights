# CS Insights

CS Insights is a Go-based tool that parses Counter-Strike 2 (CS2) demo files to detect specific physiological player habits and provide actionable feedback. Rather than basic statistics, it focuses on identifying bad habits during gunfights, such as premature firing, aim spasming, and poor spray control.

## Analyzers

1. **Premature Firing & Reaction Time**: Detects if you shoot before fully aiming at an enemy (using angular distance). Also measures reaction time from the moment an enemy enters your FOV to your first shot.
2. **Aim Spasm & Tensing**: Tracks view angles in a rolling buffer. Alerts you if it detects rapid, erratic direction reversals (zig-zags) and high variance right before you shoot.
3. **Spray Accuracy**: Groups consecutive shots and calculates efficiency. Alerts if a spray falls below a 20% hit rate.

## Installation

You need [Go](https://golang.org/doc/install) installed.

```bash
git clone https://github.com/Miconen/cs-insights.git
cd cs-insights
cd backend
go build -o cs-insights ./cmd/cs-insights
```

## Usage

Using the tool is a two-step process: first, parse a demo file to generate insights, then run the web server to view them.

### 1. Parse a Demo File

Run the tool against a CS2 `.dem` file, providing your exact in-game name. This will parse the demo and save the insights to a local SQLite database (`insights.db`).

```bash
cd backend
./cs-insights --demo=/path/to/your/match.dem --player="YourExactIngameName"
```

*(Note: Parsing can take a minute depending on the demo size and your CPU).*

### 2. View the Dashboard

Start the web dashboard to view your generated insights:

```bash
cd backend
./cs-insights --serve
```

Or from the repository root:

```bash
make
```

If you want the Steam match-history token flow, enter your Steam Web API key in the Fetch Demos page. You can create a Steam Web API key at `https://steamcommunity.com/dev/apikey`.

Then start the Svelte frontend:

```bash
cd frontend
npm install
npm run dev
```

Open your browser at [http://localhost:5173](http://localhost:5173).

Enter your player name in the form to see your specific habits and alerts!
