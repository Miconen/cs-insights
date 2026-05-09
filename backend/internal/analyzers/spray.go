package analyzers

import (
	"fmt"
	"math"

	"cs-insights/internal/config"
	"cs-insights/internal/parser"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

type SprayAnalyzer struct {
	targetPlayer string
	cfg          config.SprayConfig
	insights     []parser.InsightData

	// State for tracking an active spray
	isSpraying     bool
	sprayStartTick int
	shotsFired     int
	shotsHit       int
	lastShotTick   int
	lastEnemyHitID int
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
	case events.RoundStart:
		a.resetSpray()

	case events.WeaponFire:
		if e.Shooter == nil || e.Shooter.Name != a.targetPlayer {
			return
		}

		// Exclude pistols (all classes) and semi-automatic weapons.
		// Spraying is only meaningful analysis for automatic rifles, SMGs, and heavy weapons.
		if e.Weapon.Class() == common.EqClassPistols {
			return
		}
		if e.Weapon.Class() == common.EqClassEquipment || e.Weapon.Class() == common.EqClassGrenade {
			return
		}
		// Exclude semi-auto snipers (AWP, Scout, Scar20, G3SG1)
		switch e.Weapon.Type {
		case common.EqAWP, common.EqSSG08, common.EqScar20, common.EqG3SG1:
			return
		}

		if state.LiveEnemyCount == 0 {
			return
		}

		// Check if this shot is part of a continuous spray.
		// If the time since the last shot is small, we consider it the same spray.
		// 16 ticks is ~0.25s at 64 tick rate.
		tickDiff := state.CurrentTick - a.lastShotTick

		if a.isSpraying && tickDiff > 16 { // If more than ~0.25s passed since last shot, spray ended
			a.evaluateSpray(state)
			a.resetSpray()
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
			a.resetSpray()
		}
	}
}

func (a *SprayAnalyzer) resetSpray() {
	a.isSpraying = false
	a.sprayStartTick = 0
	a.shotsFired = 0
	a.shotsHit = 0
	a.lastShotTick = 0
	a.lastEnemyHitID = 0
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

		rangeLabel := "close range"
		switch {
		case distance >= a.cfg.LongRangeThreshold:
			rangeLabel = "long range"
		case distance >= a.cfg.MediumRangeThreshold:
			rangeLabel = "medium range"
		}

		if efficiency < 0.2 {
			desc := fmt.Sprintf("Inefficient spray at %s (%.0f units): Fired %d bullets, hit %d (%.0f%%)",
				rangeLabel, distance, a.shotsFired, a.shotsHit, efficiency*100)
			severity := "Low"
			if distance >= a.cfg.LongRangeThreshold {
				severity = "High"
			} else if distance >= a.cfg.MediumRangeThreshold {
				severity = "Medium"
			}

			a.insights = append(a.insights, parser.InsightData{
				Round:       state.CurrentRound,
				Tick:        a.sprayStartTick,
				Type:        "PoorSpray",
				Severity:    severity,
				Description: desc,
			})
		} else if distance >= a.cfg.LongRangeThreshold && a.shotsFired > 10 {
			// Hitting at long range but still spraying a lot is a bad habit
			a.insights = append(a.insights, parser.InsightData{
				Round:       state.CurrentRound,
				Tick:        a.sprayStartTick,
				Type:        "SprayConfidence",
				Severity:    "Medium",
				Description: fmt.Sprintf("Overconfident long-range spray: Fired %d bullets at a target %.0f units away.", a.shotsFired, distance),
			})
		} else if distance >= a.cfg.MediumRangeThreshold && a.shotsFired > 15 {
			// Spraying a very high number of bullets at medium range
			a.insights = append(a.insights, parser.InsightData{
				Round:       state.CurrentRound,
				Tick:        a.sprayStartTick,
				Type:        "SprayConfidence",
				Severity:    "Low",
				Description: fmt.Sprintf("Extended medium-range spray: Fired %d bullets at a target %.0f units away. Consider switching to burst fire.", a.shotsFired, distance),
			})
		}
	}
}

func (a *SprayAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
