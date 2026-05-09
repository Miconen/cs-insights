package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"cs-insights/internal/analyzers"
	"cs-insights/internal/config"
	"cs-insights/internal/db"
	"cs-insights/internal/fetcher"
	"cs-insights/internal/parser"
	"cs-insights/internal/web"
)

func main() {
	demoPath := flag.String("demo", "", "Path to the CS2 .dem file")
	playerName := flag.String("player", "", "Exact in-game name of the player to analyze")
	serve := flag.Bool("serve", false, "Start the web dashboard")
	dbPath := flag.String("db", "insights.db", "Path to SQLite database")
	clearDb := flag.Bool("clear", false, "Clear all previous insights from the database before running")
	
	// Fetching flags
	steamID := flag.String("steam_id", "", "Steam ID or Custom URL (to fetch demos automatically from Matchmaking)")
	cookie := flag.String("cookie", "", "steamLoginSecure cookie value (required for fetching)")
	fetchLimit := flag.Int("limit", 1, "Number of recent matches to fetch")

	flag.Parse()

	database, err := db.InitDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if *clearDb {
		err := database.ClearInsights()
		if err != nil {
			log.Fatalf("Failed to clear database: %v", err)
		}
		log.Println("Successfully cleared previous insights from the database.")
		// If they only wanted to clear, they might not have provided a demo.
		if *demoPath == "" && *steamID == "" && !*serve {
			return
		}
	}

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Printf("Warning: Failed to load config.json, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	if *serve {
		server := web.NewServer(database, cfg)
		err := server.Start("0.0.0.0:8080")
		if err != nil {
			log.Fatalf("Web server failed: %v", err)
		}
		return
	}

	if (*demoPath == "" && *steamID == "") || *playerName == "" {
		fmt.Println("Usage: cs-insights --demo=<path.dem> --player=\"Name\"")
		fmt.Println("   Or: cs-insights --steam_id=<id> --cookie=\"...\" --player=\"Name\"")
		fmt.Println("   Or: cs-insights --serve")
		os.Exit(1)
	}

	var demosToParse []string
	if *demoPath != "" {
		demosToParse = append(demosToParse, *demoPath)
	} else if *steamID != "" && *cookie != "" {
		log.Printf("Fetching up to %d recent matches for Steam ID %s...", *fetchLimit, *steamID)
		fetched, err := fetcher.FetchRecentMatches(*steamID, *cookie, *fetchLimit, "demos")
		if err != nil {
			log.Fatalf("Failed to fetch matches: %v", err)
		}
		demosToParse = fetched
	} else if *steamID != "" && *cookie == "" {
		log.Fatalf("You must provide --cookie=\"your_steamLoginSecure_cookie\" to fetch matches.")
	}

	for _, dp := range demosToParse {
		log.Printf("Starting analysis on %s for player %s", dp, *playerName)

		engine := parser.NewEngine(dp, *playerName)

		// Register V1 & V2 Analyzers
		engine.AddAnalyzer(analyzers.NewPrematureFireAnalyzer(*playerName, cfg.Analyzers.PrematureFire))
		engine.AddAnalyzer(analyzers.NewSpasmAnalyzer(*playerName, cfg.Analyzers.Spasm))
		engine.AddAnalyzer(analyzers.NewSprayAnalyzer(*playerName, cfg.Analyzers.Spray))
		engine.AddAnalyzer(analyzers.NewCounterStrafeAnalyzer(*playerName, cfg.Analyzers.CounterStrafe))
		engine.AddAnalyzer(analyzers.NewGunfightAnalyzer(*playerName))

		log.Println("Parsing demo (this may take a minute)...")
		insights, err := engine.Parse()
		if err != nil {
			log.Printf("Error parsing demo %s: %v", dp, err)
			continue
		}

		log.Printf("Parsing complete for %s. Found %d insights.", dp, len(insights))

		// Save to DB
		if err := database.DeleteInsightsForMatch(*playerName, dp); err != nil {
			log.Printf("Failed to replace previous insights for %s: %v", dp, err)
			continue
		}
		for _, i := range insights {
			err := database.SaveInsight(db.Insight{
				PlayerName:  *playerName,
				MatchName:   dp,
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
	}

	log.Println("All analyses complete. Insights saved to database. Run with --serve to view them.")
}
