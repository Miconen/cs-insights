package web

import (
	"cs-insights/internal/db"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sort"
)

type Server struct {
	db *db.Database
}

func NewServer(database *db.Database) *Server {
	return &Server{db: database}
}

func (s *Server) Start(addr string) error {
	http.HandleFunc("/", s.handleDashboard)

	log.Printf("Starting web server on http://%s", addr)
	return http.ListenAndServe(addr, nil)
}

type DashboardData struct {
	PlayerName     string
	Insights       []RichInsight
	Advice         []template.HTML
	ChartLabelsRaw string
	ChartDataRaw   string
	LostDuels      int
	AvgTTDDiff     int
}

type RichInsight struct {
	db.Insight
	Meta map[string]interface{}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	playerName := r.URL.Query().Get("player")
	if playerName == "" {
		tmpl := template.Must(template.New("index").Parse(indexTmpl))
		tmpl.Execute(w, nil)
		return
	}

	insights, err := s.db.GetInsightsForPlayer(playerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var richInsights []RichInsight
	counts := make(map[string]int)
	
	lostDuels := 0
	totalTTDDiff := 0.0

	for _, i := range insights {
		counts[i.Type]++
		
		ri := RichInsight{Insight: i, Meta: make(map[string]interface{})}
		if i.Metadata != "" {
			json.Unmarshal([]byte(i.Metadata), &ri.Meta)
		}
		
		// Extract gunfight data for advice
		if i.Type == "Gunfight" && ri.Meta["winner"] != nil && ri.Meta["winner"] != playerName {
			lostDuels++
			targetTTD, ok1 := ri.Meta["target_ttd_ms"].(float64)
			enemyTTD, ok2 := ri.Meta["enemy_ttd_ms"].(float64)
			if ok1 && ok2 && targetTTD > 0 && enemyTTD > 0 {
				totalTTDDiff += (targetTTD - enemyTTD)
			}
		}

		richInsights = append(richInsights, ri)
	}

	// Prepare Chart Data
	var labels []string
	var dataVals []int
	for k, v := range counts {
		labels = append(labels, k)
		dataVals = append(dataVals, v)
	}
	
	labelsJSON, _ := json.Marshal(labels)
	dataJSON, _ := json.Marshal(dataVals)

	// Generate Actionable Advice
	var advice []template.HTML
	
	// Gunfight specific advice
	if lostDuels > 0 {
		avgDiff := 0
		if totalTTDDiff > 0 {
			avgDiff = int(totalTTDDiff / float64(lostDuels))
			advice = append(advice, template.HTML("💀 <b>Gunfights:</b> You lost "+string(rune(lostDuels+'0'))+" duels. In the fights where you both dealt damage, you were on average <b>"+string(rune(avgDiff+'0'))+"ms slower</b> to deal damage than the enemy. Work on your reaction time and raw aim speed."))
		}
	}

	if counts["MovementError"] > 5 {
		advice = append(advice, template.HTML("🛑 <b>Counter-Strafing:</b> You are firing while moving too fast in several engagements. Focus on completely releasing your movement keys (W/A/S/D) and tapping the opposite direction right before you click."))
	}
	if counts["PrematureFire"] > 5 {
		advice = append(advice, template.HTML("⏱️ <b>Premature Firing:</b> You are clicking your mouse before your crosshair reaches the target. Try to consciously delay your trigger finger by a fraction of a second when flicking."))
	}
	if counts["Spasm"] > 3 {
		advice = append(advice, template.HTML("🧘 <b>Aim Spasming:</b> High erratic crosshair movement detected before shooting. You might be tensing your arm or panicking when an enemy appears. Focus on keeping your grip relaxed."))
	}
	if counts["PoorSpray"] > 3 {
		advice = append(advice, template.HTML("🔫 <b>Spray Control:</b> Your spray efficiency is dropping below 20%. Spend some time in a recoil control map or switch to bursting/tapping at medium to long ranges."))
	}

	// Collapse duplicate PrematureFire events in the same round (e.g. spamming shots while aiming away)
	var collapsedInsights []RichInsight
	var lastPrematureTick int
	var lastPrematureRound int

	for i := len(richInsights) - 1; i >= 0; i-- { // Process oldest to newest to keep the first instance
		ri := richInsights[i]
		if ri.Type == "PrematureFire" {
			if ri.Round == lastPrematureRound && (ri.Tick - lastPrematureTick) > 0 && (ri.Tick - lastPrematureTick) < 128 {
				// Skip this one since it's within 2 seconds of the last one in the same round
				lastPrematureTick = ri.Tick
				continue
			}
			lastPrematureRound = ri.Round
			lastPrematureTick = ri.Tick
		}
		collapsedInsights = append(collapsedInsights, ri)
	}
	
	richInsights = collapsedInsights

	// Sort insights by Tick/Round descending
	sort.Slice(richInsights, func(i, j int) bool {
		if richInsights[i].Round == richInsights[j].Round {
			return richInsights[i].Tick > richInsights[j].Tick
		}
		return richInsights[i].Round > richInsights[j].Round
	})

	data := DashboardData{
		PlayerName:     playerName,
		Insights:       richInsights,
		Advice:         advice,
		ChartLabelsRaw: string(labelsJSON),
		ChartDataRaw:   string(dataJSON),
	}

	tmpl := template.Must(template.New("dashboard").Parse(dashboardTmpl))
	tmpl.Execute(w, data)
}

const indexTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CS Insights</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-900 h-screen flex items-center justify-center">
    <div class="bg-slate-800 p-8 rounded-xl shadow-2xl w-96 border border-slate-700">
        <h1 class="text-3xl font-bold mb-6 text-white text-center">CS Insights</h1>
        <form action="/" method="GET" class="space-y-4">
            <div>
                <label for="player" class="block text-sm font-medium text-slate-300 mb-2">Enter Player Name</label>
                <input type="text" name="player" id="player" class="block w-full rounded-md border-0 py-2.5 px-3 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6 bg-slate-100" placeholder="e.g. s1mple">
            </div>
            <button type="submit" class="w-full flex justify-center py-2.5 px-4 border border-transparent rounded-md shadow-sm text-sm font-bold text-white bg-indigo-600 hover:bg-indigo-500 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500">
                View Insights
            </button>
        </form>
    </div>
</body>
</html>
`

const dashboardTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Insights: {{.PlayerName}}</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body class="bg-slate-50 text-slate-900 font-sans">
    <div class="max-w-6xl mx-auto py-10 px-4 sm:px-6 lg:px-8">
        <div class="mb-8 flex justify-between items-end border-b pb-4">
            <div>
                <h1 class="text-4xl font-extrabold tracking-tight text-slate-900">Performance Dashboard</h1>
                <p class="text-lg text-slate-500 mt-2">Analysis for <span class="font-bold text-indigo-600">{{.PlayerName}}</span></p>
            </div>
            <a href="/" class="text-sm font-medium text-indigo-600 hover:text-indigo-500">← Back to Search</a>
        </div>

        {{if not .Insights}}
            <div class="bg-white shadow rounded-lg p-10 text-center text-slate-500 border border-slate-200">
                <svg class="mx-auto h-12 w-12 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                    <path vector-effect="non-scaling-stroke" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 13h6m-3-3v6m-9 1V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" />
                </svg>
                <h3 class="mt-2 text-sm font-medium text-slate-900">No data found</h3>
                <p class="mt-1 text-sm text-slate-500">Run the CLI tool to parse a demo for this player first.</p>
            </div>
        {{else}}

        <div class="grid grid-cols-1 md:grid-cols-3 gap-8 mb-10">
            <!-- Advice Column -->
            <div class="md:col-span-2 space-y-6">
                <div class="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
                    <div class="bg-indigo-600 px-6 py-4">
                        <h2 class="text-xl font-bold text-white">Coach's Advice</h2>
                    </div>
                    <div class="p-6">
                        {{if .Advice}}
                            <ul class="space-y-4">
                                {{range .Advice}}
                                    <li class="flex gap-4 items-start bg-indigo-50/50 p-4 rounded-lg border border-indigo-100">
                                        <div class="text-slate-800 leading-relaxed text-sm">{{.}}</div>
                                    </li>
                                {{end}}
                            </ul>
                        {{else}}
                            <p class="text-slate-500 text-center py-4">No major habits detected yet. Keep playing!</p>
                        {{end}}
                    </div>
                </div>
            </div>

            <!-- Chart Column -->
            <div class="bg-white rounded-xl shadow-sm border border-slate-200 p-6 flex flex-col items-center justify-center">
                <h3 class="text-lg font-bold text-slate-800 w-full text-center mb-4">Habit Profile</h3>
                <div class="w-full relative" style="height: 250px;">
                    <canvas id="habitChart"></canvas>
                </div>
            </div>
        </div>

        <h2 class="text-2xl font-bold mb-6 text-slate-800">Raw Incident Log</h2>
        <div class="space-y-4">
            {{range .Insights}}
                <div class="bg-white shadow-sm rounded-lg p-5 border-l-4 border border-y-slate-200 border-r-slate-200
                    {{if eq .Severity "High"}}border-l-red-500{{else if eq .Severity "Medium"}}border-l-amber-500{{else}}border-l-blue-500{{end}} hover:shadow-md transition-shadow">
                    <div class="flex justify-between items-start">
                        <div>
                            <span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold uppercase tracking-wider
                                {{if eq .Severity "High"}}bg-red-100 text-red-800{{else if eq .Severity "Medium"}}bg-amber-100 text-amber-800{{else}}bg-blue-100 text-blue-800{{end}}">
                                {{.Type}}
                            </span>
                            <h3 class="mt-3 text-lg font-medium text-slate-900">{{.Description}}</h3>
                            
                            {{if eq .Type "Gunfight"}}
                                <div class="mt-4 bg-slate-50 p-3 rounded border border-slate-200">
                                    <div class="text-xs font-bold text-slate-500 mb-2 uppercase tracking-wide">Duel Timeline</div>
                                    <div class="flex flex-col space-y-1 text-sm font-mono text-slate-700">
                                        <div class="flex"><span class="w-16 text-slate-400">0ms:</span> Spotted</div>
                                        {{if gt .Meta.target_shot_ms 0.0}}<div class="flex"><span class="w-16 text-indigo-500">{{printf "%.0f" .Meta.target_shot_ms}}ms:</span> You fired</div>{{end}}
                                        {{if gt .Meta.enemy_shot_ms 0.0}}<div class="flex"><span class="w-16 text-rose-500">{{printf "%.0f" .Meta.enemy_shot_ms}}ms:</span> Enemy fired</div>{{end}}
                                        {{if gt .Meta.target_ttd_ms 0.0}}<div class="flex"><span class="w-16 text-indigo-500 font-bold">{{printf "%.0f" .Meta.target_ttd_ms}}ms:</span> You dealt damage</div>{{end}}
                                        {{if gt .Meta.enemy_ttd_ms 0.0}}<div class="flex"><span class="w-16 text-rose-500 font-bold">{{printf "%.0f" .Meta.enemy_ttd_ms}}ms:</span> Enemy dealt damage</div>{{end}}
                                    </div>
                                    {{if gt .Meta.crosshair_pitch 0.0}}
                                    <div class="mt-3 pt-3 border-t border-slate-200 text-sm text-slate-600">
                                        <span class="font-bold">Crosshair Placement:</span> At the start of the duel, your crosshair was {{printf "%.1f" .Meta.crosshair_pitch}}° {{.Meta.crosshair_dir}}.
                                    </div>
                                    {{end}}
                                </div>
                            {{end}}
                        </div>
                        <div class="text-sm text-slate-500 text-right font-medium flex flex-col items-end">
                            <div class="bg-slate-100 px-3 py-1 rounded-md mb-1 border border-slate-200">Round {{.Round}}</div>
                            <div class="text-xs text-slate-400 mb-2">Tick {{.Tick}}</div>
                            <button onclick="navigator.clipboard.writeText('demo_gototick {{.Tick}}'); this.innerText='Copied!'; setTimeout(() => this.innerText='demo_gototick {{.Tick}}', 2000)" 
                                    class="text-xs font-mono bg-slate-800 text-slate-300 px-2 py-1.5 rounded hover:bg-slate-700 hover:text-white transition-colors border border-slate-700 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                                    title="Click to copy console command">
                                demo_gototick {{.Tick}}
                            </button>
                        </div>
                    </div>
                </div>
            {{end}}
        </div>
        {{end}}
    </div>

    {{if .Insights}}
    <script>
        const ctx = document.getElementById('habitChart').getContext('2d');
        const labels = {{.ChartLabelsRaw}};
        const data = {{.ChartDataRaw}};

        new Chart(ctx, {
            type: 'polarArea',
            data: {
                labels: labels,
                datasets: [{
                    data: data,
                    backgroundColor: [
                        'rgba(239, 68, 68, 0.7)',
                        'rgba(245, 158, 11, 0.7)',
                        'rgba(59, 130, 246, 0.7)',
                        'rgba(16, 185, 129, 0.7)',
                        'rgba(139, 92, 246, 0.7)'
                    ],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right',
                        labels: { boxWidth: 12 }
                    }
                }
            }
        });
    </script>
    {{end}}
</body>
</html>
`