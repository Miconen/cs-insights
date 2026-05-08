package parser

import (
	"log"
	"os"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	events "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/events"
)

type GameState struct {
	CurrentTick   int
	CurrentRound  int
	Parser        demoinfocs.Parser
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
}

type Engine struct {
	demoPath    string
	targetPlayer string
	analyzers    []Analyzer
	state       *GameState
}

func NewEngine(demoPath string, targetPlayer string) *Engine {
	return &Engine{
		demoPath:    demoPath,
		targetPlayer: targetPlayer,
		analyzers:   []Analyzer{},
		state:       &GameState{},
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
		
		for _, a := range e.analyzers {
			a.OnTickDone(e.state)
		}
	}

	var allInsights []InsightData
	for _, a := range e.analyzers {
		allInsights = append(allInsights, a.GetInsights()...)
	}

	return allInsights, nil
}

func (e *Engine) notifyEvent(event interface{}) {
	for _, a := range e.analyzers {
		a.OnEvent(event, e.state)
	}
}
