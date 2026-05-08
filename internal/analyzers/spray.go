package analyzers

import (
	"fmt"

	"cs-insights/internal/parser"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

type SprayAnalyzer struct {
	targetPlayer string
	insights     []parser.InsightData

	// State for tracking an active spray
	isSpraying      bool
	sprayStartTick  int
	shotsFired      int
	shotsHit        int
	lastShotTick    int
}

func NewSprayAnalyzer(targetPlayer string) *SprayAnalyzer {
	return &SprayAnalyzer{
		targetPlayer: targetPlayer,
	}
}

func (a *SprayAnalyzer) Name() string {
	return "Spray Accuracy"
}

func (a *SprayAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
	switch e := event.(type) {
	case events.WeaponFire:
		if e.Shooter == nil || e.Shooter.Name != a.targetPlayer {
			return
		}

		// Check if this shot is part of a continuous spray.
		// If the time since the last shot is small, we consider it the same spray.
		tickDiff := state.CurrentTick - a.lastShotTick
		
		if a.isSpraying && tickDiff > 32 { // If more than ~0.5s passed since last shot, spray ended
			a.evaluateSpray(state)
			a.isSpraying = false
			a.shotsFired = 0
			a.shotsHit = 0
		}

		if !a.isSpraying {
			a.isSpraying = true
			a.sprayStartTick = state.CurrentTick
		}

		a.shotsFired++
		a.lastShotTick = state.CurrentTick

	case events.PlayerHurt:
		if e.Attacker == nil || e.Attacker.Name != a.targetPlayer {
			return
		}
		
		// If we are currently spraying and hit someone, increment hits
		if a.isSpraying {
			a.shotsHit++
		}
	}
}

func (a *SprayAnalyzer) OnTickDone(state *parser.GameState) {
	// If a spray is active, but we haven't fired a shot recently, end it
	if a.isSpraying {
		tickDiff := state.CurrentTick - a.lastShotTick
		if tickDiff > 32 {
			a.evaluateSpray(state)
			a.isSpraying = false
			a.shotsFired = 0
			a.shotsHit = 0
		}
	}
}

func (a *SprayAnalyzer) evaluateSpray(state *parser.GameState) {
	if a.shotsFired > 5 { // Only care if they sprayed more than 5 bullets
		efficiency := float64(a.shotsHit) / float64(a.shotsFired)
		
		if efficiency < 0.2 { // If less than 20% hit rate
			a.insights = append(a.insights, parser.InsightData{
				Round:       state.CurrentRound,
				Tick:        a.sprayStartTick,
				Type:        "PoorSpray",
				Severity:    "Medium",
				Description: fmt.Sprintf("Inefficient spray: Fired %d bullets, hit %d (%.0f%%)", a.shotsFired, a.shotsHit, efficiency*100),
			})
		}
	}
}

func (a *SprayAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
