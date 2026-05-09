package analyzers

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"cs-insights/internal/parser"
	"github.com/golang/geo/r3"
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
	CrosshairDirection   string

	TargetDamage int
	EnemyDamage  int
	TargetHits   int
	EnemyHits    int

	TargetWeapon string
	EnemyWeapon  string

	TargetStartHP int
	EnemyStartHP  int

	// First Bullet Accuracy
	TargetFirstBulletAccuracy float64
	TargetWasPeeking          bool
	EnemyWasPeeking           bool
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
	WasPeeking     bool    `json:"was_peeking"` // Kept for backwards compatibility
	FightType      string  `json:"fight_type"`
	Tags           []string `json:"tags"`

	TargetDamage  int    `json:"target_damage"`
	EnemyDamage   int    `json:"enemy_damage"`
	TargetHits    int    `json:"target_hits"`
	EnemyHits     int    `json:"enemy_hits"`
	TargetWeapon  string `json:"target_weapon"`
	EnemyWeapon   string `json:"enemy_weapon"`
	TargetStartHP int    `json:"target_start_hp"`
	EnemyStartHP  int    `json:"enemy_start_hp"`

	Rating   int    `json:"rating"`
	Analysis string `json:"analysis"`
}

type GunfightAnalyzer struct {
	targetPlayer    string
	insights        []parser.InsightData
	activeDuels     map[int]*Gunfight
	positionHistory map[int][]r3.Vector
}

func NewGunfightAnalyzer(targetPlayer string) *GunfightAnalyzer {
	return &GunfightAnalyzer{
		targetPlayer:    targetPlayer,
		activeDuels:     make(map[int]*Gunfight),
		positionHistory: make(map[int][]r3.Vector),
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
			if duel.TargetWeapon == "" {
				duel.TargetWeapon = e.Weapon.String()
			}
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
					duel.TargetFirstBulletAccuracy = math.Sqrt(pDiff*pDiff + yDiff*yDiff)
				}

				// Check if holding or peeking based on position history
				var maxDist float64
				currPos := e.Shooter.Position()
				if history, ok := a.positionHistory[e.Shooter.UserID]; ok {
					for _, pos := range history {
						distX := currPos.X - pos.X
						distY := currPos.Y - pos.Y
						dist := math.Sqrt(float64(distX*distX + distY*distY))
						if dist > maxDist {
							maxDist = dist
						}
					}
				}
				// If they moved more than 40 units in the last 1 second (~64 ticks), they are peeking/dancing
				duel.TargetWasPeeking = maxDist > 40.0
			}
		} else if closestEnemy.Name == a.targetPlayer {
			duel := a.getOrCreateDuel(state, e.Shooter)
			if duel.EnemyWeapon == "" {
				duel.EnemyWeapon = e.Weapon.String()
			}
			if duel.EnemyFirstShotTick == 0 {
				duel.EnemyFirstShotTick = state.CurrentTick
				
				var maxDist float64
				currPos := e.Shooter.Position()
				if history, ok := a.positionHistory[e.Shooter.UserID]; ok {
					for _, pos := range history {
						distX := currPos.X - pos.X
						distY := currPos.Y - pos.Y
						dist := math.Sqrt(float64(distX*distX + distY*distY))
						if dist > maxDist {
							maxDist = dist
						}
					}
				}
				duel.EnemyWasPeeking = maxDist > 40.0
			}
		}

	case events.PlayerHurt:
		if e.Attacker == nil || e.Player == nil {
			return
		}
		if e.Attacker.Name == a.targetPlayer {
			duel := a.getOrCreateDuel(state, e.Player)
			duel.TargetDamage += e.HealthDamage
			duel.TargetHits++
			if duel.TargetFirstHitTick == 0 {
				duel.TargetFirstHitTick = state.CurrentTick
			}
		} else if e.Player.Name == a.targetPlayer {
			duel := a.getOrCreateDuel(state, e.Attacker)
			duel.EnemyDamage += e.HealthDamage
			duel.EnemyHits++
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
		// Clear history when dead/round over, but ensure map is initialized
		a.positionHistory = make(map[int][]r3.Vector)
		return
	}

	// Track positions for all active players
	for _, p := range state.Parser.GameState().Participants().Playing() {
		if !p.IsAlive() {
			continue
		}
		uid := p.UserID
		history := a.positionHistory[uid]
		history = append(history, p.Position())
		if len(history) > 64 {
			history = history[1:]
		}
		a.positionHistory[uid] = history
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

	targetPlayer := getPlayerByName(state, a.targetPlayer)
	targetHP := 100
	if targetPlayer != nil {
		targetHP = targetPlayer.Health()
	}

	duel := &Gunfight{
		EnemyID:       enemy.UserID,
		EnemyName:     enemy.Name,
		StartTick:     state.CurrentTick,
		IsActive:      true,
		TargetStartHP: targetHP,
		EnemyStartHP:  enemy.Health(),
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
			return -1
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
		TargetDamage:   duel.TargetDamage,
		EnemyDamage:    duel.EnemyDamage,
		TargetHits:     duel.TargetHits,
		EnemyHits:      duel.EnemyHits,
		TargetWeapon:   duel.TargetWeapon,
		EnemyWeapon:    duel.EnemyWeapon,
		TargetStartHP:  duel.TargetStartHP,
		EnemyStartHP:   duel.EnemyStartHP,
		Tags:           []string{},
	}

	// Only record if it was an actual duel (shots fired or damage dealt)
	if meta.TargetShotMs < 0 && meta.EnemyShotMs < 0 && meta.TargetTTDMs < 0 && meta.EnemyTTDMs < 0 {
		return
	}

	// Determine fundamental fight type
	if meta.EnemyShotMs < 0 && meta.TargetShotMs >= 0 {
		meta.FightType = "Flank / Unaware"
	} else if meta.TargetShotMs < 0 && meta.EnemyShotMs >= 0 {
		meta.FightType = "Flanked / Unaware"
	} else if duel.TargetWasPeeking && duel.EnemyWasPeeking {
		meta.FightType = "Peek vs Peek"
	} else if duel.TargetWasPeeking && !duel.EnemyWasPeeking {
		meta.FightType = "Peek vs Hold"
	} else if !duel.TargetWasPeeking && duel.EnemyWasPeeking {
		meta.FightType = "Hold vs Peek"
	} else {
		meta.FightType = "Hold vs Hold"
	}

	rating, analysis := evaluateDuel(duel, meta, winner == a.targetPlayer)
	meta.Rating = rating
	meta.Analysis = analysis

	metaBytes, _ := json.Marshal(meta)

	severity := "Low"
	desc := fmt.Sprintf("Duel vs %s (Won)", duel.EnemyName)
	if winner != a.targetPlayer {
		if rating <= 3 {
			severity = "High"
		} else if rating <= 6 {
			severity = "Medium"
		} else {
			severity = "Low"
		}
		desc = fmt.Sprintf("Duel vs %s (Lost)", duel.EnemyName)
	} else {
		// If you won, but the rating was poor (e.g. you got lucky)
		if rating <= 4 {
			severity = "Medium"
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

func evaluateDuel(duel *Gunfight, meta GunfightMetadata, won bool) (int, string) {
	rating := 5
	analysis := ""
	var details []string

	if won {
		rating += 2
		details = append(details, "+2 You won the duel.")
		if meta.TargetTTDMs >= 0 && meta.TargetTTDMs < 300 {
			rating += 2 // Fast kill
			details = append(details, fmt.Sprintf("+2 Excellent TTK (%.0fms).", meta.TargetTTDMs))
			meta.Tags = append(meta.Tags, "Insta-kill")
		} else if meta.TargetDamage >= 100 {
			details = append(details, "Solid kill.")
		}

		if meta.TargetStartHP < meta.EnemyStartHP-20 {
			rating += 2 // Won at a disadvantage
			details = append(details, "+2 Great job winning at a health disadvantage!")
			meta.Tags = append(meta.Tags, "Disadvantage")
		}

		if meta.EnemyDamage >= 80 {
			rating -= 2 // Barely survived
			details = append(details, "-2 You barely survived this duel.")
			meta.Tags = append(meta.Tags, "Close Call")
		}
	} else {
		if meta.TargetDamage == 0 {
			if meta.EnemyDamage >= 100 && meta.EnemyHits == 1 && meta.TargetShotMs < 0 {
				details = append(details, "0 You were instantly one-tapped before you could react. Just unlucky.")
				meta.Tags = append(meta.Tags, "Insta-killed")
			} else {
				rating -= 3 // Whiffed or instakilled
				if meta.TargetShotMs >= 0 && (meta.EnemyShotMs < 0 || meta.TargetShotMs < meta.EnemyShotMs) {
					details = append(details, "-3 You shot first but whiffed completely, dealing 0 damage while they killed you.")
					meta.Tags = append(meta.Tags, "Whiffed")
				} else if meta.TargetShotMs < 0 {
					details = append(details, "-3 You were killed before you could even fire a shot.")
				} else {
					details = append(details, "-3 You dealt 0 damage in this fight.")
					meta.Tags = append(meta.Tags, "Whiffed")
				}
			}
		} else if meta.TargetDamage >= 80 {
			rating += 2 // Close fight
			msg := fmt.Sprintf("+2 Very close fight! You dealt heavy damage (%d in %d hits).", meta.TargetDamage, meta.TargetHits)
			if meta.TargetWeapon != "" && meta.EnemyWeapon != "" {
				msg += fmt.Sprintf(" Lost the aim duel against %s with your %s.", meta.EnemyWeapon, meta.TargetWeapon)
			}
			details = append(details, msg)
			meta.Tags = append(meta.Tags, "Close Call", "Aim Duel")
		} else {
			rating -= 1
			details = append(details, fmt.Sprintf("-1 You traded some damage (%d in %d hits).", meta.TargetDamage, meta.TargetHits))
			meta.Tags = append(meta.Tags, "Traded Damage")
		}

		if meta.TargetStartHP <= 20 {
			rating += 2 // Was basically unwinnable
			details = append(details, fmt.Sprintf("+2 You started the duel at critical health (%d HP), making this an extremely hard fight to win.", meta.TargetStartHP))
		} else if meta.TargetStartHP > 80 && meta.EnemyStartHP <= 30 && meta.TargetDamage == 0 {
			rating -= 3 // Choked an easy kill
			details = append(details, "-3 You had a massive health advantage but choked the kill.")
		}

		if meta.TargetShotMs >= 0 && meta.EnemyShotMs >= 0 {
			if meta.TargetShotMs > meta.EnemyShotMs+100 {
				rating -= 1
				details = append(details, fmt.Sprintf("-1 Your slow reaction time (fired %.0fms after enemy) cost you the duel.", meta.TargetShotMs-meta.EnemyShotMs))
			}
		}
	}

	if meta.FirstBulletAcc > 10.0 {
		rating -= 2
		details = append(details, "-2 You shot wildly before aiming (>10° off target).")
	} else if meta.FirstBulletAcc > 5.0 {
		rating -= 1
		details = append(details, "-1 Your first bullet accuracy was poor (>5° off).")
	} else if meta.FirstBulletAcc <= 2.0 && !meta.WasPeeking {
		rating += 1
		details = append(details, "+1 Excellent crosshair placement while holding.")
	}

	if rating < 1 {
		rating = 1
	} else if rating > 10 {
		rating = 10
	}
	
	analysis = strings.Join(details, " ")

	return rating, strings.TrimSpace(analysis)
}

func (a *GunfightAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
