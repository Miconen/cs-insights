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
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SteamID string `json:"steam_id"`
		Cookie  string `json:"cookie"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	steamID := strings.TrimSpace(req.SteamID)
	cookie := strings.TrimSpace(req.Cookie)

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
	if err := s.db.DeleteInsightsForMatch(req.PlayerName, demPath); err != nil {
		http.Error(w, "Error replacing previous insights: "+err.Error(), http.StatusInternalServerError)
		return
	}
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
	for _, i := range insights {
		ri := RichInsight{Insight: i, Meta: make(map[string]interface{})}
		if i.Metadata != "" {
			json.Unmarshal([]byte(i.Metadata), &ri.Meta)
		}
		ri.MatchDisplay = displayMatchName(i.MatchName)
		ri.MapName = mapNameFromMeta(ri.Meta)
		richInsights = append(richInsights, ri)
	}

	// Collapse duplicate PrematureFire events in the same round
	var collapsedInsights []RichInsight
	var lastPrematureTick int
	var lastPrematureRound int
	var lastPrematureMatch string

	for i := len(richInsights) - 1; i >= 0; i-- {
		ri := richInsights[i]
		if ri.Type == "PrematureFire" {
			if ri.MatchName == lastPrematureMatch && ri.Round == lastPrematureRound && (ri.Tick-lastPrematureTick) > 0 && (ri.Tick-lastPrematureTick) < 128 {
				lastPrematureTick = ri.Tick
				continue
			}
			lastPrematureMatch = ri.MatchName
			lastPrematureRound = ri.Round
			lastPrematureTick = ri.Tick
		}
		collapsedInsights = append(collapsedInsights, ri)
	}

	richInsights = collapsedInsights

	// Sort insights by MatchName descending, then Round ascending, then Tick ascending
	sort.Slice(richInsights, func(i, j int) bool {
		if richInsights[i].MatchName == richInsights[j].MatchName {
			if richInsights[i].Round == richInsights[j].Round {
				return richInsights[i].Tick < richInsights[j].Tick
			}
			return richInsights[i].Round < richInsights[j].Round
		}
		return richInsights[i].MatchName > richInsights[j].MatchName
	})

	counts := make(map[string]int)
	gameCounts := make(map[string]int)
	gameMaps := make(map[string]string)
	lostDuels := 0
	totalTTDDiff := 0.0

	for _, ri := range richInsights {
		counts[ri.Type]++
		gameCounts[ri.MatchName]++
		if ri.MapName != "" {
			gameMaps[ri.MatchName] = ri.MapName
		}

		if ri.Type == "Gunfight" && ri.Meta["winner"] != nil && ri.Meta["winner"] != playerName {
			lostDuels++
			targetTTD, ok1 := ri.Meta["target_ttd_ms"].(float64)
			enemyTTD, ok2 := ri.Meta["enemy_ttd_ms"].(float64)
			if ok1 && ok2 && targetTTD > 0 && enemyTTD > 0 {
				totalTTDDiff += (targetTTD - enemyTTD)
			}
		}
	}

	// Generate Actionable Advice (now plain text, frontend handles HTML)
	var advice []string
	totalGunfights := counts["Gunfight"]
	if lostDuels > 0 && totalGunfights > 0 {
		lossRate := int(float64(lostDuels) / float64(totalGunfights) * 100)
		if totalTTDDiff > 0 {
			avgDiff := int(totalTTDDiff / float64(lostDuels))
			advice = append(advice, fmt.Sprintf("Gunfights: You lost %d of %d tracked duels (%d%%). In fights where you both dealt damage, you were on average %dms slower to deal damage than the enemy.", lostDuels, totalGunfights, lossRate, avgDiff))
		} else {
			advice = append(advice, fmt.Sprintf("Gunfights: You lost %d of %d tracked duels (%d%%).", lostDuels, totalGunfights, lossRate))
		}
	}

	if n := counts["MovementError"]; n > 0 {
		advice = append(advice, fmt.Sprintf("Counter-Strafing: You fired while moving too fast in %d engagements. Release movement keys and tap the opposite direction before clicking.", n))
	}
	if n := counts["PrematureFire"]; n > 0 {
		advice = append(advice, fmt.Sprintf("Premature Firing: You clicked before your crosshair reached the target %d times. Delay your trigger finger a fraction of a second when flicking.", n))
	}
	if n := counts["Spasm"]; n > 0 {
		advice = append(advice, fmt.Sprintf("Aim Spasming: Erratic crosshair movement before shooting detected %d times. Keep your grip relaxed and avoid tensing when you see an enemy.", n))
	}
	if n := counts["PoorSpray"]; n > 0 {
		alt := counts["SprayConfidence"]
		if alt > 0 {
			advice = append(advice, fmt.Sprintf("Spray Control: %d inefficient sprays and %d overconfident long-range sprays detected. Spend time on a recoil control map, or switch to bursting and tapping at longer ranges.", n, alt))
		} else {
			advice = append(advice, fmt.Sprintf("Spray Control: %d inefficient sprays detected (under 20%% hit rate). Practise recoil control or switch to burst/tap at medium-to-long range.", n))
		}
	}

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
