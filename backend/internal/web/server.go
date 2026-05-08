package web

import (
	"cs-insights/internal/analyzers"
	"cs-insights/internal/config"
	"cs-insights/internal/db"
	"cs-insights/internal/fetcher"
	"cs-insights/internal/parser"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
)

type Server struct {
	db  *db.Database
	cfg *config.Config
}

func NewServer(database *db.Database, cfg *config.Config) *Server {
	return &Server{db: database, cfg: cfg}
}

func (s *Server) Start(addr string) error {
	// Enable CORS for development
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next(w, r)
		}
	}

	http.HandleFunc("/api/insights", corsMiddleware(s.handleInsightsAPI))
	http.HandleFunc("/api/fetch/list", corsMiddleware(s.handleFetchListAPI))
	http.HandleFunc("/api/fetch/process", corsMiddleware(s.handleFetchProcessAPI))

	log.Printf("Starting API server on http://%s", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleFetchListAPI(w http.ResponseWriter, r *http.Request) {
	steamID := r.URL.Query().Get("steam_id")
	cookie := r.URL.Query().Get("cookie")

	if steamID == "" || cookie == "" {
		http.Error(w, "Missing steam_id or cookie", http.StatusBadRequest)
		return
	}

	matches, err := fetcher.GetMatchHistory(steamID, cookie, "demos")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(matches)
}

func (s *Server) handleFetchProcessAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Link       string `json:"link"`
		PlayerName string `json:"player_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Link == "" || req.PlayerName == "" {
		http.Error(w, "Missing link or player_name", http.StatusBadRequest)
		return
	}

	// 1. Reuse existing demo if available, otherwise download & decompress.
	demoFile, err := fetcher.DownloadAndDecompressWithStatus(req.Link, "demos")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	demPath := demoFile.Path

	// 2. Parse Demo
	engine := parser.NewEngine(demPath, req.PlayerName)
	engine.AddAnalyzer(analyzers.NewPrematureFireAnalyzer(req.PlayerName, s.cfg.Analyzers.PrematureFire))
	engine.AddAnalyzer(analyzers.NewSpasmAnalyzer(req.PlayerName, s.cfg.Analyzers.Spasm))
	engine.AddAnalyzer(analyzers.NewSprayAnalyzer(req.PlayerName, s.cfg.Analyzers.Spray))
	engine.AddAnalyzer(analyzers.NewCounterStrafeAnalyzer(req.PlayerName, s.cfg.Analyzers.CounterStrafe))
	engine.AddAnalyzer(analyzers.NewGunfightAnalyzer(req.PlayerName))

	insights, err := engine.Parse()
	if err != nil {
		http.Error(w, "Error parsing demo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Save Insights
	for _, i := range insights {
		err := s.db.SaveInsight(db.Insight{
			PlayerName:  req.PlayerName,
			MatchName:   demPath,
			Round:       i.Round,
			Tick:        i.Tick,
			Type:        i.Type,
			Severity:    i.Severity,
			Description: i.Description,
			Metadata:    i.Metadata,
		})
		if err != nil {
			log.Printf("Failed to save insight: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"insights":   len(insights),
		"downloaded": demoFile.Downloaded,
		"demo_path":  demPath,
	})
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
	Games          []GameSummary  `json:"games"`
}

type GameSummary struct {
	MatchName     string `json:"match_name"`
	DisplayName   string `json:"display_name"`
	MapName       string `json:"map_name"`
	IncidentCount int    `json:"incident_count"`
}

type RichInsight struct {
	db.Insight
	Meta         map[string]interface{} `json:"meta"`
	MatchDisplay string                 `json:"match_display"`
	MapName      string                 `json:"map_name"`
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
	gameCounts := make(map[string]int)
	gameMaps := make(map[string]string)

	lostDuels := 0
	totalTTDDiff := 0.0

	for _, i := range insights {
		counts[i.Type]++

		ri := RichInsight{Insight: i, Meta: make(map[string]interface{})}
		if i.Metadata != "" {
			json.Unmarshal([]byte(i.Metadata), &ri.Meta)
		}
		ri.MatchDisplay = displayMatchName(i.MatchName)
		ri.MapName = mapNameFromMeta(ri.Meta)
		gameCounts[i.MatchName]++
		if ri.MapName != "" {
			gameMaps[i.MatchName] = ri.MapName
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
			advice = append(advice, fmt.Sprintf("Gunfights: You lost %d duels. In the fights where you both dealt damage, you were on average %dms slower to deal damage than the enemy.", lostDuels, avgDiff))
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

	games := make([]GameSummary, 0, len(gameCounts))
	for matchName, count := range gameCounts {
		games = append(games, GameSummary{
			MatchName:     matchName,
			DisplayName:   displayMatchName(matchName),
			MapName:       gameMaps[matchName],
			IncidentCount: count,
		})
	}
	sort.Slice(games, func(i, j int) bool {
		return games[i].DisplayName < games[j].DisplayName
	})

	response := APIResponse{
		PlayerName: playerName,
		Insights:   richInsights,
		Advice:     advice,
		Summary: SummaryData{
			TotalIncidents: len(richInsights),
			LostDuels:      lostDuels,
			AvgTTDDiffMs:   avgTTD,
			CountsByType:   counts,
			Games:          games,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func displayMatchName(matchName string) string {
	base := filepath.Base(matchName)
	base = strings.TrimSuffix(base, ".dem")
	base = strings.TrimSuffix(base, ".dem.bz2")
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "Unknown match"
	}
	return base
}

func mapNameFromMeta(meta map[string]interface{}) string {
	if value, ok := meta["map"].(string); ok && value != "" {
		return value
	}
	return "Unknown map"
}
