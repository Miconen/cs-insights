package analyzers

import (
	"fmt"
	"math"

	"cs-insights/internal/config"
	"cs-insights/internal/parser"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

type SprayAnalyzer struct {
	targetPlayer string
	cfg          config.SprayConfig
	insights     []parser.InsightData

	// State for tracking an active spray
	isSpraying      bool
	sprayStartTick  int
	shotsFired      int
	shotsHit        int
	lastShotTick    int
	lastEnemyHitID  int
}

func NewSprayAnalyzer(targetPlayer string, cfg config.SprayConfig) *SprayAnalyzer {
	return &SprayAnalyzer{
		targetPlayer: targetPlayer,
		cfg:          cfg,
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
		// 16 ticks is ~0.25s at 64 tick rate.
		tickDiff := state.CurrentTick - a.lastShotTick
		
		if a.isSpraying && tickDiff > 16 { // If more than ~0.25s passed since last shot, spray ended
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
		if e.Attacker == nil || e.Attacker.Name != a.targetPlayer || e.Player == nil {
			return
		}
		
		// If we are currently spraying and hit someone, increment hits
		if a.isSpraying {
			a.shotsHit++
			a.lastEnemyHitID = e.Player.UserID
		}
	}
}

func (a *SprayAnalyzer) OnTickDone(state *parser.GameState) {
	// If a spray is active, but we haven't fired a shot recently, end it
	if a.isSpraying {
		tickDiff := state.CurrentTick - a.lastShotTick
		if tickDiff > 16 {
			a.evaluateSpray(state)
			a.isSpraying = false
			a.shotsFired = 0
			a.shotsHit = 0
		}
	}
}

func (a *SprayAnalyzer) evaluateSpray(state *parser.GameState) {
	if a.shotsFired >= 7 { // Only care if they sprayed 7 or more bullets (definitely a spray, not a 3-4 bullet burst)
		efficiency := float64(a.shotsHit) / float64(a.shotsFired)
		
		// Calculate distance if we hit someone
		distance := 0.0
		if a.lastEnemyHitID > 0 {
			targetPlayer := getPlayerByName(state, a.targetPlayer)
			if targetPlayer != nil {
				for _, p := range state.Parser.GameState().Participants().Playing() {
					if p.UserID == a.lastEnemyHitID {
						pos1 := targetPlayer.Position()
						pos2 := p.Position()
						distX := pos2.X - pos1.X
						distY := pos2.Y - pos1.Y
						distZ := pos2.Z - pos1.Z
						distance = math.Sqrt(float64(distX*distX + distY*distY + distZ*distZ))
						break
					}
				}
			}
		}

		if efficiency < 0.2 { // If less than 20% hit rate
			desc := fmt.Sprintf("Inefficient spray: Fired %d bullets, hit %d (%.0f%%)", a.shotsFired, a.shotsHit, efficiency*100)
			severity := "Medium"
			if distance > a.cfg.LongRangeThreshold {
				desc += fmt.Sprintf(" - Distance was %.0f units (Low percentage long-range spray)", distance)
				severity = "High"
			}

			a.insights = append(a.insights, parser.InsightData{
				Round:       state.CurrentRound,
				Tick:        a.sprayStartTick,
				Type:        "PoorSpray",
				Severity:    severity,
				Description: desc,
			})
		} else if distance > a.cfg.LongRangeThreshold && a.shotsFired > 10 {
			// Even if they hit, spraying 10+ bullets at long range is a bad habit
			a.insights = append(a.insights, parser.InsightData{
				Round:       state.CurrentRound,
				Tick:        a.sprayStartTick,
				Type:        "SprayConfidence",
				Severity:    "Medium",
				Description: fmt.Sprintf("Overconfident long-range spray: Fired %d bullets at a target %.0f units away.", a.shotsFired, distance),
			})
		}
	}
}

func (a *SprayAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
