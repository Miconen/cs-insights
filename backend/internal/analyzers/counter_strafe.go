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

type CounterStrafeAnalyzer struct {
	targetPlayer string
	cfg          config.CounterStrafeConfig
	insights     []parser.InsightData

	lastPos r3.Vector
	speed2D float64
}

func NewCounterStrafeAnalyzer(targetPlayer string, cfg config.CounterStrafeConfig) *CounterStrafeAnalyzer {
	return &CounterStrafeAnalyzer{
		targetPlayer: targetPlayer,
		cfg:          cfg,
	}
}

func (a *CounterStrafeAnalyzer) Name() string {
	return "Counter-Strafing"
}

func (a *CounterStrafeAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
	switch e := event.(type) {
	case events.WeaponFire:
		if e.Shooter == nil || e.Shooter.Name != a.targetPlayer {
			return
		}

		if e.Weapon.Class() == common.EqClassGrenade || e.Weapon.Class() == common.EqClassEquipment {
			return
		}

		// Pistols and SMGs (except Desert Eagle) have natural movement accuracy penalties
		// built into the game, so counter-strafing is less meaningful for them.
		if e.Weapon.Type != common.EqDeagle &&
			(e.Weapon.Class() == common.EqClassPistols || e.Weapon.Class() == common.EqClassSMG) {
			return
		}

		if state.LiveEnemyCount == 0 {
			return
		}

		// Use the speed calculated in OnTickDone
		if a.speed2D > a.cfg.MaxVelocityThreshold {
			// Check if there is an enemy roughly in front of them to ensure it's an actual engagement
			var enemyInSight bool
			for _, p := range state.Parser.GameState().Participants().Playing() {
				if p.Team == e.Shooter.Team || !p.IsAlive() {
					continue
				}

				shooterEyes, ok := e.Shooter.PositionEyes()
				if !ok {
					continue
				}

				pitch, yaw := calculateAngles(shooterEyes, p.Position())

				pDiff := math.Abs(float64(e.Shooter.ViewDirectionX() - pitch))
				yDiff := math.Abs(float64(e.Shooter.ViewDirectionY() - yaw))

				if yDiff > 180 {
					yDiff = 360 - yDiff
				}

				if pDiff < 45 && yDiff < 45 {
					enemyInSight = true
					break
				}
			}

			if enemyInSight {
				a.insights = append(a.insights, parser.InsightData{
					Round:       state.CurrentRound,
					Tick:        state.CurrentTick,
					Type:        "MovementError",
					Severity:    "High",
					Description: fmt.Sprintf("Fired weapon while moving too fast (Speed: %.1f units/sec)", a.speed2D),
				})
			}
		}
	}
}

func (a *CounterStrafeAnalyzer) OnTickDone(state *parser.GameState) {
	targetPlayer := getPlayerByName(state, a.targetPlayer)
	if targetPlayer == nil || !targetPlayer.IsAlive() {
		return
	}

	currPos := targetPlayer.Position()
	if a.lastPos.X != 0 && a.lastPos.Y != 0 {
		distX := currPos.X - a.lastPos.X
		distY := currPos.Y - a.lastPos.Y
		dist := math.Sqrt(float64(distX*distX + distY*distY))

		// Speed in units/sec = distance * tickRate
		a.speed2D = dist * state.Parser.TickRate()
	}
	a.lastPos = currPos
}

func (a *CounterStrafeAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
