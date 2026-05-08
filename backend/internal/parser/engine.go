package parser

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	msg "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

type GameState struct {
	CurrentTick    int
	CurrentRound   int
	MapName        string
	LiveEnemyCount int // updated every tick for the target player's perspective
	Parser         demoinfocs.Parser
}

type Analyzer interface {
	Name() string
	OnEvent(event interface{}, state *GameState)
	OnTickDone(state *GameState)
	GetInsights() []InsightData
}

type InsightData struct {
	Round       int
	Tick        int
	Type        string
	Severity    string
	Description string
	Metadata    string // JSON encoded metadata
}

type Engine struct {
	demoPath     string
	targetPlayer string
	analyzers    []Analyzer
	state        *GameState
}

func NewEngine(demoPath string, targetPlayer string) *Engine {
	return &Engine{
		demoPath:     demoPath,
		targetPlayer: targetPlayer,
		analyzers:    []Analyzer{},
		state:        &GameState{},
	}
}

func (e *Engine) AddAnalyzer(a Analyzer) {
	e.analyzers = append(e.analyzers, a)
}

func (e *Engine) Parse() ([]InsightData, error) {
	f, err := os.Open(e.demoPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Error handling config for panic recovery during parsing
	cfg := demoinfocs.DefaultParserConfig
	cfg.IgnoreErrBombsiteIndexNotFound = true
	cfg.IgnorePacketEntitiesPanic = true

	// We must recover gracefully or the parser will OOM leak if it loops errors
	p := demoinfocs.NewParserWithConfig(f, cfg)
	defer p.Close()

	e.state.Parser = p

	// Register event handlers
	p.RegisterNetMessageHandler(func(message *msg.CSVCMsg_ServerInfo) {
		if message.GetMapName() != "" {
			e.state.MapName = message.GetMapName()
		}
	})

	p.RegisterEventHandler(func(event events.RoundStart) {
		e.state.CurrentRound++
		e.notifyEvent(event)
	})

	p.RegisterEventHandler(func(event events.WeaponFire) {
		e.notifyEvent(event)
	})

	p.RegisterEventHandler(func(event events.PlayerHurt) {
		e.notifyEvent(event)
	})

	p.RegisterEventHandler(func(event events.PlayerFlashed) {
		e.notifyEvent(event)
	})

	p.RegisterEventHandler(func(event events.Kill) {
		e.notifyEvent(event)
	})

	p.RegisterEventHandler(func(event events.PlayerSpottersChanged) {
		e.notifyEvent(event)
	})

	// Try extracting all events and letting analyzers filter them
	p.RegisterEventHandler(func(event interface{}) {
		// e.notifyEvent(event) -- Can be spammy, better to selectively route like above, but for now we route the specific ones above + fallthrough
		// We can add a generic event router if we need to.
	})

	// Optional: we can silence the parser's internal logger to prevent spam
	p.RegisterEventHandler(func(event events.ParserWarn) {
		// Do nothing to suppress the spam
	})

	// Tick processing
	for {
		more, err := p.ParseNextFrame()
		if err != nil {
			// Instead of a fatal log, just warn and break. This prevents OOM loops on highly corrupted demos.
			log.Printf("Warning: stopped parsing due to frame error (demo might be corrupted/unsupported): %v", err)
			break
		}
		if !more {
			break
		}

		e.state.CurrentTick = p.CurrentFrame()

		// Update live enemy count for the target player's team perspective.
		liveEnemies := 0
		var targetTeam int
		for _, participant := range p.GameState().Participants().Playing() {
			if participant.Name == e.targetPlayer {
				targetTeam = int(participant.Team)
				break
			}
		}
		if targetTeam != 0 {
			for _, participant := range p.GameState().Participants().Playing() {
				team := int(participant.Team)
				// 2 = T, 3 = CT
				if (team == 2 || team == 3) && team != targetTeam && participant.IsAlive() {
					liveEnemies++
				}
			}
		}
		e.state.LiveEnemyCount = liveEnemies

		for _, a := range e.analyzers {
			a.OnTickDone(e.state)
		}
	}

	var allInsights []InsightData
	for _, a := range e.analyzers {
		allInsights = append(allInsights, a.GetInsights()...)
	}

	for i := range allInsights {
		allInsights[i].Metadata = mergeMatchMetadata(allInsights[i].Metadata, map[string]interface{}{
			"map":       e.state.MapName,
			"demo_file": filepath.Base(e.demoPath),
		})
	}

	return allInsights, nil
}

func mergeMatchMetadata(existing string, additions map[string]interface{}) string {
	metadata := map[string]interface{}{}
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &metadata); err != nil {
			metadata = map[string]interface{}{}
		}
	}

	for key, value := range additions {
		if value != nil && value != "" {
			metadata[key] = value
		}
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		return existing
	}
	return string(encoded)
}

func (e *Engine) notifyEvent(event interface{}) {
	for _, a := range e.analyzers {
		a.OnEvent(event, e.state)
	}
}
