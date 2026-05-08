package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"cs-insights/internal/analyzers"
	"cs-insights/internal/config"
	"cs-insights/internal/db"
	"cs-insights/internal/parser"
	"cs-insights/internal/web"
)

func main() {
	demoPath := flag.String("demo", "", "Path to the CS2 .dem file")
	playerName := flag.String("player", "", "Exact in-game name of the player to analyze")
	serve := flag.Bool("serve", false, "Start the web dashboard")
	dbPath := flag.String("db", "insights.db", "Path to SQLite database")

	flag.Parse()

	database, err := db.InitDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if *serve {
		server := web.NewServer(database)
		err := server.Start("0.0.0.0:8080")
		if err != nil {
			log.Fatalf("Web server failed: %v", err)
		}
		return
	}

	if *demoPath == "" || *playerName == "" {
		fmt.Println("Usage: cs-insights --demo=<path.dem> --player=\"Name\"")
		fmt.Println("   Or: cs-insights --serve")
		os.Exit(1)
	}

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Printf("Warning: Failed to load config.json, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	log.Printf("Starting analysis on %s for player %s", *demoPath, *playerName)

	engine := parser.NewEngine(*demoPath, *playerName)

	// Register V1 & V2 Analyzers
	engine.AddAnalyzer(analyzers.NewPrematureFireAnalyzer(*playerName, cfg.Analyzers.PrematureFire))
	engine.AddAnalyzer(analyzers.NewSpasmAnalyzer(*playerName, cfg.Analyzers.Spasm))
	engine.AddAnalyzer(analyzers.NewSprayAnalyzer(*playerName))
	engine.AddAnalyzer(analyzers.NewCounterStrafeAnalyzer(*playerName, cfg.Analyzers.CounterStrafe))
	engine.AddAnalyzer(analyzers.NewCrosshairPlacementAnalyzer(*playerName, cfg.Analyzers.CrosshairHeight))

	log.Println("Parsing demo (this may take a minute)...")
	insights, err := engine.Parse()
	if err != nil {
		log.Fatalf("Error parsing demo: %v", err)
	}

	log.Printf("Parsing complete. Found %d insights.", len(insights))

	// Save to DB
	for _, i := range insights {
		err := database.SaveInsight(db.Insight{
			PlayerName:  *playerName,
			MatchName:   *demoPath,
			Round:       i.Round,
			Tick:        i.Tick,
			Type:        i.Type,
			Severity:    i.Severity,
			Description: i.Description,
		})
		if err != nil {
			log.Printf("Failed to save insight: %v", err)
		}
	}

	log.Println("Insights saved to database. Run with --serve to view them.")
}
