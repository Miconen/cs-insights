package analyzers

import (
	"encoding/json"
	"fmt"
	"math"

	"cs-insights/internal/parser"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

type Gunfight struct {
	EnemyID             int
	EnemyName           string
	StartTick           int
	TargetFirstShotTick int
	EnemyFirstShotTick  int
	TargetFirstHitTick  int
	EnemyFirstHitTick   int
	IsActive            bool
	CrosshairPitchDiff  float64
	CrosshairDirection  string

	// First Bullet Accuracy
	TargetFirstBulletAccuracy float64
	TargetWasPeeking          bool
}

type GunfightMetadata struct {
	TargetTTDMs    float64 `json:"target_ttd_ms"`
	EnemyTTDMs     float64 `json:"enemy_ttd_ms"`
	TargetShotMs   float64 `json:"target_shot_ms"`
	EnemyShotMs    float64 `json:"enemy_shot_ms"`
	CrosshairPitch float64 `json:"crosshair_pitch"`
	CrosshairDir   string  `json:"crosshair_dir"`
	Winner         string  `json:"winner"`
	FirstBulletAcc float64 `json:"first_bullet_acc"`
	WasPeeking     bool    `json:"was_peeking"`
}

type GunfightAnalyzer struct {
	targetPlayer string
	insights     []parser.InsightData
	activeDuels  map[int]*Gunfight
}

func NewGunfightAnalyzer(targetPlayer string) *GunfightAnalyzer {
	return &GunfightAnalyzer{
		targetPlayer: targetPlayer,
		activeDuels:  make(map[int]*Gunfight),
	}
}

func (a *GunfightAnalyzer) Name() string {
	return "Gunfight Tracker"
}

func (a *GunfightAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
	if state.LiveEnemyCount == 0 {
		return
	}

	switch e := event.(type) {
	case events.WeaponFire:
		if e.Shooter == nil {
			return
		}

		if e.Weapon.Class() == common.EqClassGrenade || e.Weapon.Class() == common.EqClassEquipment {
			return
		}

		// Find the closest enemy to the shooter to assign the shot to a duel
		closestEnemy := a.getClosestEnemy(state, e.Shooter)
		if closestEnemy == nil {
			return
		}

		if e.Shooter.Name == a.targetPlayer {
			duel := a.getOrCreateDuel(state, closestEnemy)
			if duel.TargetFirstShotTick == 0 {
				duel.TargetFirstShotTick = state.CurrentTick

				// Calculate First Bullet Accuracy
				targetEyes, ok1 := e.Shooter.PositionEyes()
				enemyEyes, ok2 := closestEnemy.PositionEyes()
				if ok1 && ok2 {
					pitchToHead, yawToHead := calculateAngles(targetEyes, enemyEyes)
					pDiff := math.Abs(float64(e.Shooter.ViewDirectionX() - pitchToHead))
					yDiff := math.Abs(float64(e.Shooter.ViewDirectionY() - yawToHead))
					if yDiff > 180 {
						yDiff = 360 - yDiff
					}
					duel.TargetFirstBulletAccuracy = pDiff + yDiff
				}

				// Check if holding or peeking (based on 2D velocity at shot time)
				// We don't have historical positions easily accessible here without state tracking,
				// so for simplicity in V1 of this feature, we will estimate based on view movement
				// or just tag it based on general movement.
				// For now, let's just log the accuracy without the stance since Velocity() is not available in v5.
				duel.TargetWasPeeking = false
			}
		} else if closestEnemy.Name == a.targetPlayer {
			duel := a.getOrCreateDuel(state, e.Shooter)
			if duel.EnemyFirstShotTick == 0 {
				duel.EnemyFirstShotTick = state.CurrentTick
			}
		}

	case events.PlayerHurt:
		if e.Attacker == nil || e.Player == nil {
			return
		}
		if e.Attacker.Name == a.targetPlayer {
			duel := a.getOrCreateDuel(state, e.Player)
			if duel.TargetFirstHitTick == 0 {
				duel.TargetFirstHitTick = state.CurrentTick
			}
		} else if e.Player.Name == a.targetPlayer {
			duel := a.getOrCreateDuel(state, e.Attacker)
			if duel.EnemyFirstHitTick == 0 {
				duel.EnemyFirstHitTick = state.CurrentTick
			}
		}

	case events.Kill:
		if e.Killer == nil || e.Victim == nil {
			return
		}

		if e.Victim.Name == a.targetPlayer {
			// Target died
			duel, exists := a.activeDuels[e.Killer.UserID]
			if exists {
				a.resolveDuel(state, duel, e.Killer.Name)
				delete(a.activeDuels, e.Killer.UserID)
			}
		} else if e.Killer.Name == a.targetPlayer {
			// Target killed enemy
			duel, exists := a.activeDuels[e.Victim.UserID]
			if exists {
				a.resolveDuel(state, duel, a.targetPlayer)
				delete(a.activeDuels, e.Victim.UserID)
			}
		}
	}
}

func (a *GunfightAnalyzer) OnTickDone(state *parser.GameState) {
	targetPlayer := getPlayerByName(state, a.targetPlayer)
	if targetPlayer == nil || !targetPlayer.IsAlive() || state.LiveEnemyCount == 0 {
		// If target is dead or no enemies remain, clear active duels.
		for id := range a.activeDuels {
			delete(a.activeDuels, id)
		}
		return
	}

	// Update active duels or start new ones based on FOV
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

		inSight := pDiff < 45 && yDiff < 45

		duel, exists := a.activeDuels[p.UserID]
		if inSight {
			if !exists {
				duel = a.getOrCreateDuel(state, p)

				// Capture Crosshair Placement on first sight
				enemyEyes, ok := p.PositionEyes()
				if ok {
					pitchToHead, _ := calculateAngles(targetEyes, enemyEyes)
					pDiffHead := math.Abs(float64(targetPlayer.ViewDirectionX() - pitchToHead))
					duel.CrosshairPitchDiff = pDiffHead

					if targetPlayer.ViewDirectionX() > pitchToHead {
						duel.CrosshairDirection = "too low (at chest/feet)"
					} else {
						duel.CrosshairDirection = "too high"
					}
				}
			}
		} else {
			// If out of sight and time has passed, we could expire the duel.
			// For simplicity, we just keep it until someone dies or the round ends.
		}
	}
}

func (a *GunfightAnalyzer) getOrCreateDuel(state *parser.GameState, enemy *common.Player) *Gunfight {
	if duel, exists := a.activeDuels[enemy.UserID]; exists {
		return duel
	}
	duel := &Gunfight{
		EnemyID:   enemy.UserID,
		EnemyName: enemy.Name,
		StartTick: state.CurrentTick,
		IsActive:  true,
	}
	a.activeDuels[enemy.UserID] = duel
	return duel
}

func (a *GunfightAnalyzer) getClosestEnemy(state *parser.GameState, player *common.Player) *common.Player {
	var closestEnemy *common.Player
	minAngle := math.MaxFloat64

	for _, p := range state.Parser.GameState().Participants().Playing() {
		if p.Team == player.Team || !p.IsAlive() {
			continue
		}

		eyes, ok := player.PositionEyes()
		if !ok {
			continue
		}
		pitch, yaw := calculateAngles(eyes, p.Position())

		pitchDiff := math.Abs(float64(player.ViewDirectionX() - pitch))
		yawDiff := math.Abs(float64(player.ViewDirectionY() - yaw))
		if yawDiff > 180 {
			yawDiff = 360 - yawDiff
		}

		totalAngleDiff := pitchDiff + yawDiff
		if totalAngleDiff < minAngle {
			minAngle = totalAngleDiff
			closestEnemy = p
		}
	}

	// Make sure they are relatively close in aim
	if minAngle > 45.0 {
		return nil
	}
	return closestEnemy
}

func (a *GunfightAnalyzer) resolveDuel(state *parser.GameState, duel *Gunfight, winner string) {
	tickRate := state.Parser.TickRate()
	if tickRate == 0 {
		tickRate = 64
	}
	tickToMs := func(t int) float64 {
		if t == 0 {
			return 0
		}
		return float64(t-duel.StartTick) * (1000.0 / tickRate)
	}

	meta := GunfightMetadata{
		TargetShotMs:   tickToMs(duel.TargetFirstShotTick),
		EnemyShotMs:    tickToMs(duel.EnemyFirstShotTick),
		TargetTTDMs:    tickToMs(duel.TargetFirstHitTick),
		EnemyTTDMs:     tickToMs(duel.EnemyFirstHitTick),
		CrosshairPitch: duel.CrosshairPitchDiff,
		CrosshairDir:   duel.CrosshairDirection,
		Winner:         winner,
		FirstBulletAcc: duel.TargetFirstBulletAccuracy,
		WasPeeking:     duel.TargetWasPeeking,
	}

	// Only record if it was an actual duel (shots fired or damage dealt)
	if meta.TargetShotMs == 0 && meta.EnemyShotMs == 0 && meta.TargetTTDMs == 0 && meta.EnemyTTDMs == 0 {
		return
	}

	metaBytes, _ := json.Marshal(meta)

	severity := "Low"
	desc := fmt.Sprintf("Duel vs %s (Won)", duel.EnemyName)
	if winner != a.targetPlayer {
		severity = "High"
		desc = fmt.Sprintf("Duel lost vs %s", duel.EnemyName)

		// Add some contextual text based on the math
		if meta.TargetTTDMs == 0 && meta.EnemyTTDMs > 0 {
			desc += " - You dealt no damage."
		} else if meta.TargetTTDMs > meta.EnemyTTDMs {
			diff := meta.TargetTTDMs - meta.EnemyTTDMs
			desc += fmt.Sprintf(" - You were %.0fms slower to deal damage.", diff)
		} else if meta.TargetShotMs > 0 && meta.TargetShotMs < meta.EnemyShotMs && meta.TargetTTDMs == 0 {
			desc += " - You shot first but missed."
		}
	}

	a.insights = append(a.insights, parser.InsightData{
		Round:       state.CurrentRound,
		Tick:        duel.StartTick,
		Type:        "Gunfight",
		Severity:    severity,
		Description: desc,
		Metadata:    string(metaBytes),
	})
}

func (a *GunfightAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
