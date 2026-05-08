package analyzers

import (
	"fmt"
	"math"

	"cs-insights/internal/config"
	"cs-insights/internal/parser"
	"github.com/golang/geo/r3"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

type PrematureFireAnalyzer struct {
	targetPlayer string
	cfg          config.PrematureFireConfig
	insights     []parser.InsightData
}

func NewPrematureFireAnalyzer(targetPlayer string, cfg config.PrematureFireConfig) *PrematureFireAnalyzer {
	return &PrematureFireAnalyzer{
		targetPlayer: targetPlayer,
		cfg:          cfg,
	}
}

func (a *PrematureFireAnalyzer) Name() string {
	return "Premature Firing"
}

func (a *PrematureFireAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
	switch e := event.(type) {
	case events.WeaponFire:
		if e.Shooter == nil || e.Shooter.Name != a.targetPlayer {
			return
		}

		if e.Weapon.Class() == common.EqClassGrenade || e.Weapon.Class() == common.EqClassEquipment {
			return
		}

		// Single-shot weapons (AWP, Scout, Deagle) missing is just a miss, not a premature fire.
		switch e.Weapon.Type {
		case common.EqAWP, common.EqSSG08, common.EqDeagle:
			return
		}

		// Don't count if all enemies are already dead.
		if state.LiveEnemyCount == 0 {
			return
		}

		var closestEnemy *common.Player
		minAngle := math.MaxFloat64

		for _, p := range state.Parser.GameState().Participants().Playing() {
			if p.Team == e.Shooter.Team || !p.IsAlive() {
				continue
			}

			shooterEyes, ok := e.Shooter.PositionEyes()
			if !ok {
				continue
			}
			pitch, yaw := calculateAngles(shooterEyes, p.Position())

			pitchDiff := math.Abs(float64(e.Shooter.ViewDirectionX() - pitch))
			yawDiff := math.Abs(float64(e.Shooter.ViewDirectionY() - yaw))

			if yawDiff > 180 {
				yawDiff = 360 - yawDiff
			}

			totalAngleDiff := pitchDiff + yawDiff

			if totalAngleDiff < minAngle {
				minAngle = totalAngleDiff
				closestEnemy = p
			}
		}

		if closestEnemy != nil {
			// If angle is > MaxEngagementAngle, we assume they are spamming/wallbanging, not targeting this enemy.
			if minAngle > a.cfg.MaxEngagementAngle {
				return
			}

			// Premature Fire threshold: > 3 degrees (clear flick happening) but shot fired
			if minAngle > 3.0 && minAngle <= a.cfg.MaxEngagementAngle {
				a.insights = append(a.insights, parser.InsightData{
					Round:       state.CurrentRound,
					Tick:        state.CurrentTick,
					Type:        "PrematureFire",
					Severity:    "Medium",
					Description: fmt.Sprintf("Fired shot while crosshair was %.1f degrees away from target (%s)", minAngle, closestEnemy.Name),
				})
			}
		}
	}
}

func (a *PrematureFireAnalyzer) OnTickDone(state *parser.GameState) {
}

func (a *PrematureFireAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}

func getPlayerByName(state *parser.GameState, name string) *common.Player {
	for _, p := range state.Parser.GameState().Participants().Playing() {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func calculateAngles(pos1, pos2 r3.Vector) (pitch, yaw float32) {
	delta := r3.Vector{
		X: pos2.X - pos1.X,
		Y: pos2.Y - pos1.Y,
		Z: pos2.Z - pos1.Z,
	}

	hyp := math.Sqrt(float64(delta.X*delta.X + delta.Y*delta.Y))

	pitch = float32(math.Atan2(-float64(delta.Z), hyp) * 180.0 / math.Pi)
	yaw = float32(math.Atan2(float64(delta.Y), float64(delta.X)) * 180.0 / math.Pi)

	return pitch, yaw
}
