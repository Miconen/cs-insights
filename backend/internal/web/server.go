package web

import (
	"cs-insights/internal/db"
	"encoding/json"
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
	// Enable CORS for development
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next(w, r)
		}
	}

	http.HandleFunc("/api/insights", corsMiddleware(s.handleInsightsAPI))

	log.Printf("Starting API server on http://%s", addr)
	return http.ListenAndServe(addr, nil)
}

type APIResponse struct {
	PlayerName string        `json:"player_name"`
	Insights   []RichInsight `json:"insights"`
	Advice     []string      `json:"advice"`
	Summary    SummaryData   `json:"summary"`
}

type SummaryData struct {
	TotalIncidents int            `json:"total_incidents"`
	LostDuels      int            `json:"lost_duels"`
	AvgTTDDiffMs   int            `json:"avg_ttd_diff_ms"`
	CountsByType   map[string]int `json:"counts_by_type"`
}

type RichInsight struct {
	db.Insight
	Meta map[string]interface{} `json:"meta"`
}

func (s *Server) handleInsightsAPI(w http.ResponseWriter, r *http.Request) {
	playerName := r.URL.Query().Get("player")
	if playerName == "" {
		http.Error(w, "Missing 'player' query parameter", http.StatusBadRequest)
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

	// Generate Actionable Advice (now plain text, frontend handles HTML)
	var advice []string

	if lostDuels > 0 {
		avgDiff := 0
		if totalTTDDiff > 0 {
			avgDiff = int(totalTTDDiff / float64(lostDuels))
			advice = append(advice, "Gunfights: You lost "+string(rune(lostDuels+'0'))+" duels. In the fights where you both dealt damage, you were on average "+string(rune(avgDiff+'0'))+"ms slower to deal damage than the enemy. Work on your reaction time and raw aim speed.")
		}
	}

	if counts["MovementError"] > 5 {
		advice = append(advice, "Counter-Strafing: You are firing while moving too fast in several engagements. Focus on completely releasing your movement keys (W/A/S/D) and tapping the opposite direction right before you click.")
	}
	if counts["PrematureFire"] > 5 {
		advice = append(advice, "Premature Firing: You are clicking your mouse before your crosshair reaches the target. Try to consciously delay your trigger finger by a fraction of a second when flicking.")
	}
	if counts["Spasm"] > 3 {
		advice = append(advice, "Aim Spasming: High erratic crosshair movement detected before shooting. You might be tensing your arm or panicking when an enemy appears. Focus on keeping your grip relaxed.")
	}
	if counts["PoorSpray"] > 3 {
		advice = append(advice, "Spray Control: Your spray efficiency is dropping below 20%. Spend some time in a recoil control map or switch to bursting/tapping at medium to long ranges.")
	}

	// Collapse duplicate PrematureFire events in the same round
	var collapsedInsights []RichInsight
	var lastPrematureTick int
	var lastPrematureRound int

	for i := len(richInsights) - 1; i >= 0; i-- {
		ri := richInsights[i]
		if ri.Type == "PrematureFire" {
			if ri.Round == lastPrematureRound && (ri.Tick-lastPrematureTick) > 0 && (ri.Tick-lastPrematureTick) < 128 {
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

	avgTTD := 0
	if lostDuels > 0 && totalTTDDiff > 0 {
		avgTTD = int(totalTTDDiff / float64(lostDuels))
	}

	response := APIResponse{
		PlayerName: playerName,
		Insights:   richInsights,
		Advice:     advice,
		Summary: SummaryData{
			TotalIncidents: len(richInsights),
			LostDuels:      lostDuels,
			AvgTTDDiffMs:   avgTTD,
			CountsByType:   counts,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}