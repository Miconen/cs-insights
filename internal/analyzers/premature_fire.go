package analyzers

import (
	"fmt"
	"math"

	"cs-insights/internal/config"
	"cs-insights/internal/parser"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/golang/geo/r3"
)

type PrematureFireAnalyzer struct {
	targetPlayer string
	cfg          config.PrematureFireConfig
	insights     []parser.InsightData
	
	// Key: Enemy UserID, Value: Tick when spotted
	spottedEnemies map[int]int

	// Track last tick angles to calculate velocity for reaction time
	lastPitch float32
	lastYaw   float32
}

func NewPrematureFireAnalyzer(targetPlayer string, cfg config.PrematureFireConfig) *PrematureFireAnalyzer {
	return &PrematureFireAnalyzer{
		targetPlayer:   targetPlayer,
		cfg:            cfg,
		spottedEnemies: make(map[int]int),
	}
}

func (a *PrematureFireAnalyzer) Name() string {
	return "Premature Firing & Reaction"
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
	targetPlayer := getPlayerByName(state, a.targetPlayer)
	if targetPlayer == nil || !targetPlayer.IsAlive() {
		return
	}

	currPitch := targetPlayer.ViewDirectionX()
	currYaw := targetPlayer.ViewDirectionY()

	pitchDiff := math.Abs(float64(currPitch - a.lastPitch))
	yawDiff := math.Abs(float64(currYaw - a.lastYaw))
	if yawDiff > 180 {
		yawDiff = 360 - yawDiff
	}
	velocity := pitchDiff + yawDiff

	// If there's a significant mouse movement, check if it's a reaction to any spotted enemy
	if velocity > 1.0 { // 1.0 degree per tick is a solid flick start
		for enemyID, spottedTick := range a.spottedEnemies {
			tickDiff := state.CurrentTick - spottedTick
			if tickDiff > 0 {
				timeDiffMs := float64(tickDiff) * (1000.0 / state.Parser.TickRate())
				
				if timeDiffMs > a.cfg.ReactionTimeMaxMs {
					a.insights = append(a.insights, parser.InsightData{
						Round:       state.CurrentRound,
						Tick:        state.CurrentTick,
						Type:        "SlowReaction",
						Severity:    "Low",
						Description: fmt.Sprintf("Slow reaction time: %.0fms before initiating aim movement", timeDiffMs),
					})
				}
				// We handled the reaction for this enemy, remove from map
				delete(a.spottedEnemies, enemyID)
			}
		}
	}

	a.lastPitch = currPitch
	a.lastYaw = currYaw

	for _, p := range state.Parser.GameState().Participants().Playing() {
		if p.Team == targetPlayer.Team || !p.IsAlive() {
			continue
		}

		targetEyes, ok := targetPlayer.PositionEyes()
		if !ok {
			continue
		}

		pitch, yaw := calculateAngles(targetEyes, p.Position())
		
		pDiff := math.Abs(float64(targetPlayer.ViewDirectionX() - pitch))
		yDiff := math.Abs(float64(targetPlayer.ViewDirectionY() - yaw))
		
		if yDiff > 180 {
			yDiff = 360 - yDiff
		}

		// Simple FOV check: if enemy is within 45 degrees of center view
		if pDiff < 45 && yDiff < 45 {
			if _, exists := a.spottedEnemies[p.UserID]; !exists {
				a.spottedEnemies[p.UserID] = state.CurrentTick
			}
		} else {
			// If they leave FOV, reset
			delete(a.spottedEnemies, p.UserID)
		}
	}
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
