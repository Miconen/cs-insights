package analyzers

import (
	"fmt"
	"math"

	"cs-insights/internal/parser"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/events"
)

type SpasmAnalyzer struct {
	targetPlayer string
	insights     []parser.InsightData

	// Track pitch/yaw history for the target player
	// We keep a small rolling buffer (e.g., 32 ticks = 0.5s at 64 tick)
	history [32]viewAngles
	head    int
}

type viewAngles struct {
	tick  int
	pitch float32
	yaw   float32
}

func NewSpasmAnalyzer(targetPlayer string) *SpasmAnalyzer {
	return &SpasmAnalyzer{
		targetPlayer: targetPlayer,
		head:         0,
	}
}

func (a *SpasmAnalyzer) Name() string {
	return "Aim Spasm & Tensing"
}

func (a *SpasmAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
	switch e := event.(type) {
	case events.WeaponFire:
		if e.Shooter == nil || e.Shooter.Name != a.targetPlayer {
			return
		}

		// When they shoot, analyze the last 0.5 seconds of mouse movement
		a.analyzeHistoryForSpasms(state)
	}
}

func (a *SpasmAnalyzer) OnTickDone(state *parser.GameState) {
	targetPlayer := getPlayerByName(state, a.targetPlayer)
	if targetPlayer == nil || !targetPlayer.IsAlive() {
		return
	}

	a.history[a.head] = viewAngles{
		tick:  state.CurrentTick,
		pitch: targetPlayer.ViewDirectionX(),
		yaw:   targetPlayer.ViewDirectionY(),
	}
	a.head = (a.head + 1) % len(a.history)
}

func (a *SpasmAnalyzer) analyzeHistoryForSpasms(state *parser.GameState) {
	// Analyze the rolling buffer for high variance and rapid direction reversals (zig-zagging).
	var prevYawDiff float32
	var zigzags int
	var totalVariance float32

	// We iterate through the history buffer in order
	for i := 1; i < len(a.history); i++ {
		idx := (a.head + i) % len(a.history)
		prevIdx := (a.head + i - 1) % len(a.history)

		curr := a.history[idx]
		prev := a.history[prevIdx]

		if curr.tick == 0 || prev.tick == 0 {
			continue // Skip uninitialized slots
		}

		pitchDiff := curr.pitch - prev.pitch
		yawDiff := curr.yaw - prev.yaw

		// Normalize yaw
		if yawDiff > 180 {
			yawDiff -= 360
		} else if yawDiff < -180 {
			yawDiff += 360
		}

		totalVariance += float32(math.Abs(float64(pitchDiff))) + float32(math.Abs(float64(yawDiff)))

		// Check for zig-zag (sign flip in velocity)
		// If we were moving left and are now moving right
		if (prevYawDiff > 0 && yawDiff < 0) || (prevYawDiff < 0 && yawDiff > 0) {
			if math.Abs(float64(yawDiff)) > 0.5 { // Only count significant zig-zags
				zigzags++
			}
		}

		prevYawDiff = yawDiff
	}

	// Thresholds for spasming/tensing
	if zigzags > 4 && totalVariance > 15.0 {
		a.insights = append(a.insights, parser.InsightData{
			Round:       state.CurrentRound,
			Tick:        state.CurrentTick,
			Type:        "Spasm",
			Severity:    "High",
			Description: fmt.Sprintf("High erratic crosshair movement detected before shot (Variance: %.1f, Direction Reversals: %d)", totalVariance, zigzags),
		})
	}
}

func (a *SpasmAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
