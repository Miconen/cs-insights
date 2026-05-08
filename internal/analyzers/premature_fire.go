package analyzers

import (
	"fmt"
	"math"

	"cs-insights/internal/parser"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/events"
	"github.com/golang/geo/r3"
)

type PrematureFireAnalyzer struct {
	targetPlayer string
	insights     []parser.InsightData
	
	// Map to track when an enemy was first spotted by the target player
	// Key: Enemy UserID, Value: Tick when spotted
	spottedEnemies map[int]int
}

func NewPrematureFireAnalyzer(targetPlayer string) *PrematureFireAnalyzer {
	return &PrematureFireAnalyzer{
		targetPlayer:   targetPlayer,
		spottedEnemies: make(map[int]int),
	}
}

func (a *PrematureFireAnalyzer) Name() string {
	return "Premature Firing & Reaction"
}

func (a *PrematureFireAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
	switch e := event.(type) {
	
	case events.PlayerSpottersChanged:
		// If our target player spotted an enemy
		if e.Spotted != nil && e.Spotted.IsAlive() && !e.Spotted.IsBot {
			// e.Spotted is the player being spotted.
			// Who spotted them? We need to check if our target player is among the spotters.
			// Currently, demoinfocs doesn't easily expose exactly *who* triggered the spot in this event payload cleanly 
			// without checking game state or radar. For a rough approximation, we can track proximity or FOV in OnTickDone instead.
			// Let's implement FOV checking in OnTickDone for better accuracy.
		}

	case events.WeaponFire:
		if e.Shooter == nil || e.Shooter.Name != a.targetPlayer {
			return
		}
		
		// If it's not a bullet weapon (e.g. knife, grenade), ignore
		if e.Weapon.Class() == common.EqClassGrenade || e.Weapon.Class() == common.EqClassEquipment {
			return
		}

		// Find the closest visible enemy to see if they are shooting at them
		var closestEnemy *common.Player
		minAngle := math.MaxFloat64

		for _, p := range state.Parser.GameState().Participants().Playing() {
			if p.Team == e.Shooter.Team || !p.IsAlive() {
				continue
			}

			// Calculate angular distance
			pitch, yaw := calculateAngles(e.Shooter.PositionEyes(), p.Position())
			
			// Difference in angles
			pitchDiff := math.Abs(float64(e.Shooter.ViewDirectionX() - pitch))
			yawDiff := math.Abs(float64(e.Shooter.ViewDirectionY() - yaw))
			
			// Normalize yaw diff (handle 360 wrap around)
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
			// Threshold: If angle diff is > 15 degrees, it's a premature fire
			// Note: We'd want to correlate this with if they eventually hit or moved crosshair there
			if minAngle > 15.0 {
				a.insights = append(a.insights, parser.InsightData{
					Round:       state.CurrentRound,
					Tick:        state.CurrentTick,
					Type:        "PrematureFire",
					Severity:    "Medium",
					Description: fmt.Sprintf("Fired shot while crosshair was %.1f degrees away from target (%s)", minAngle, closestEnemy.Name),
				})
			}
			
			// Check Reaction time if we have spotted them recently
			if spottedTick, ok := a.spottedEnemies[closestEnemy.UserID]; ok {
				tickDiff := state.CurrentTick - spottedTick
				timeDiffMs := float64(tickDiff) * (1000.0 / state.Parser.TickRate())
				
				if timeDiffMs > 500 { // If it took more than 500ms to shoot
					a.insights = append(a.insights, parser.InsightData{
						Round:       state.CurrentRound,
						Tick:        state.CurrentTick,
						Type:        "SlowReaction",
						Severity:    "Low",
						Description: fmt.Sprintf("Reaction time of %.0fms before first shot at %s", timeDiffMs, closestEnemy.Name),
					})
				}
				
				// Clear the spotted timer so we don't trigger reaction time again for subsequent shots in the same burst
				delete(a.spottedEnemies, closestEnemy.UserID)
			}
		}
	}
}

func (a *PrematureFireAnalyzer) OnTickDone(state *parser.GameState) {
	// Let's manually calculate if an enemy entered our target's FOV
	targetPlayer := getPlayerByName(state, a.targetPlayer)
	if targetPlayer == nil || !targetPlayer.IsAlive() {
		return
	}

	for _, p := range state.Parser.GameState().Participants().Playing() {
		if p.Team == targetPlayer.Team || !p.IsAlive() {
			continue
		}

		pitch, yaw := calculateAngles(targetPlayer.PositionEyes(), p.Position())
		
		pitchDiff := math.Abs(float64(targetPlayer.ViewDirectionX() - pitch))
		yawDiff := math.Abs(float64(targetPlayer.ViewDirectionY() - yaw))
		
		if yawDiff > 180 {
			yawDiff = 360 - yawDiff
		}

		// Simple FOV check: if enemy is within 45 degrees of center view
		if pitchDiff < 45 && yawDiff < 45 {
			if _, exists := a.spottedEnemies[p.UserID]; !exists {
				a.spottedEnemies[p.UserID] = state.CurrentTick
			}
		} else {
			// If they leave FOV, reset the timer so we can track reaction time again next time they appear
			delete(a.spottedEnemies, p.UserID)
		}
	}
}

func (a *PrematureFireAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}

// Helper to find player
func getPlayerByName(state *parser.GameState, name string) *common.Player {
	for _, p := range state.Parser.GameState().Participants().Playing() {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// calculateAngles calculates pitch and yaw from pos1 to pos2
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
