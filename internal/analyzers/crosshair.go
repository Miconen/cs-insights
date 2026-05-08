package analyzers

import (
	"fmt"
	"math"

	"cs-insights/internal/config"
	"cs-insights/internal/parser"
)

type CrosshairPlacementAnalyzer struct {
	targetPlayer   string
	cfg            config.CrosshairHeightConfig
	insights       []parser.InsightData
	spottedEnemies map[int]int
}

func NewCrosshairPlacementAnalyzer(targetPlayer string, cfg config.CrosshairHeightConfig) *CrosshairPlacementAnalyzer {
	return &CrosshairPlacementAnalyzer{
		targetPlayer:   targetPlayer,
		cfg:            cfg,
		spottedEnemies: make(map[int]int),
	}
}

func (a *CrosshairPlacementAnalyzer) Name() string {
	return "Crosshair Placement"
}

func (a *CrosshairPlacementAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
}

func (a *CrosshairPlacementAnalyzer) OnTickDone(state *parser.GameState) {
	targetPlayer := getPlayerByName(state, a.targetPlayer)
	if targetPlayer == nil || !targetPlayer.IsAlive() {
		return
	}

	for _, p := range state.Parser.GameState().Participants().Playing() {
		if p.Team == targetPlayer.Team || !p.IsAlive() {
			continue
		}

		targetEyes, ok := targetPlayer.PositionEyes()
		if !ok {
			continue
		}

		// Use enemy's head position for ideal crosshair placement
		enemyEyes, ok := p.PositionEyes()
		if !ok {
			continue
		}

		pitchToHead, yawToHead := calculateAngles(targetEyes, enemyEyes)
		
		pDiff := math.Abs(float64(targetPlayer.ViewDirectionX() - pitchToHead))
		yDiff := math.Abs(float64(targetPlayer.ViewDirectionY() - yawToHead))
		
		if yDiff > 180 {
			yDiff = 360 - yDiff
		}

		if pDiff < 45 && yDiff < 45 {
			if _, exists := a.spottedEnemies[p.UserID]; !exists {
				// Enemy just entered FOV
				a.spottedEnemies[p.UserID] = state.CurrentTick

				// Check resting vertical placement (pitch) against enemy head
				// targetPlayer.ViewDirectionX() is pitch. Positive is looking down, negative is looking up (or vice-versa in CS2, check standard).
				// We just use absolute difference for now to catch "too far away vertically"
				if pDiff > a.cfg.MaxVerticalDistance {
					// Also check if they are aiming lower than the head (usually the case)
					// In Source, pitch > 0 is looking down. If target pitch > ideal pitch, they are looking too low.
					direction := "too far vertically"
					if targetPlayer.ViewDirectionX() > pitchToHead {
						direction = "too low (at chest/feet)"
					} else {
						direction = "too high"
					}

					a.insights = append(a.insights, parser.InsightData{
						Round:       state.CurrentRound,
						Tick:        state.CurrentTick,
						Type:        "CrosshairPlacement",
						Severity:    "Medium",
						Description: fmt.Sprintf("Crosshair resting %s when engaging %s (%.1f degrees off head height)", direction, p.Name, pDiff),
					})
				}
			}
		} else {
			delete(a.spottedEnemies, p.UserID)
		}
	}
}

func (a *CrosshairPlacementAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
