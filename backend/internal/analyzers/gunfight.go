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

const (
	contactFOVDegrees      = 20.0
	shotAssignmentFOV      = 35.0
	peekMovementThreshold  = 55.0
	peekSpeedThreshold     = 80.0
	positionHistoryTicks   = 96
	movementLookbackTicks  = 48
	movementLookaheadTicks = 10
	visibilityStaleTicks   = 64
	combatStaleTicks       = 64
)

type positionSample struct {
	Tick int
	Pos  r3.Vector
}

type movementProfile struct {
	Distance       float64
	SpeedAtContact float64
	Samples        int
}

type Gunfight struct {
	EnemyID   int
	EnemyName string

	FirstTargetSeenTick int
	FirstEnemySeenTick  int
	CombatStartTick     int
	ResolutionTick      int
	LastActivityTick    int

	TargetFirstShotTick  int
	EnemyFirstShotTick   int
	TargetFirstDamageTick int
	EnemyFirstDamageTick  int

	TargetDamage int
	EnemyDamage  int
	TargetHits   int
	EnemyHits    int

	TargetWeapon string
	EnemyWeapon  string

	TargetStartHP int
	EnemyStartHP  int

	CrosshairPitchDiff float64
	CrosshairDirection string
	InitialAimOffset   float64
	FirstBulletAcc     float64
	AdjustmentNeeded   float64

	TargetWasPeeking bool
	EnemyWasPeeking  bool
	TargetMoveDist   float64
	EnemyMoveDist    float64
	TargetSpeed      float64
	EnemySpeed       float64
	TargetMoveSamples int
	EnemyMoveSamples  int

	StartSource string
	Outcome     string
}

type GunfightMetadata struct {
	TargetTTDMs    float64 `json:"target_ttd_ms"`
	EnemyTTDMs     float64 `json:"enemy_ttd_ms"`
	TargetShotMs   float64 `json:"target_shot_ms"`
	EnemyShotMs    float64 `json:"enemy_shot_ms"`
	TargetSeenMs   float64 `json:"target_seen_ms"`
	EnemySeenMs    float64 `json:"enemy_seen_ms"`
	CombatStartMs  float64 `json:"combat_start_ms"`
	ResolutionMs   float64 `json:"resolution_ms"`

	CrosshairPitch float64 `json:"crosshair_pitch"`
	CrosshairDir   string  `json:"crosshair_dir"`
	InitialAimOff  float64 `json:"initial_aim_offset"`
	FirstBulletAcc float64 `json:"first_bullet_acc"`
	Adjustment     float64 `json:"adjustment_needed"`

	Winner                   string   `json:"winner"`
	Outcome                  string   `json:"outcome"`
	StartSource              string   `json:"start_source"`
	TimingConfidence         string   `json:"timing_confidence"`
	ClassificationConfidence string   `json:"classification_confidence"`
	WasPeeking               bool     `json:"was_peeking"`
	FightType                string   `json:"fight_type"`
	Tags                     []string `json:"tags"`

	TargetMovementDist float64 `json:"target_movement_dist"`
	EnemyMovementDist  float64 `json:"enemy_movement_dist"`
	TargetSpeed        float64 `json:"target_speed"`
	EnemySpeed         float64 `json:"enemy_speed"`

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
	positionHistory map[int][]positionSample
}

func NewGunfightAnalyzer(targetPlayer string) *GunfightAnalyzer {
	return &GunfightAnalyzer{
		targetPlayer:    targetPlayer,
		activeDuels:     make(map[int]*Gunfight),
		positionHistory: make(map[int][]positionSample),
	}
}

func (a *GunfightAnalyzer) Name() string {
	return "Gunfight Tracker"
}

func (a *GunfightAnalyzer) OnEvent(event interface{}, state *parser.GameState) {
	switch e := event.(type) {
	case events.RoundStart:
		a.activeDuels = make(map[int]*Gunfight)
		a.positionHistory = make(map[int][]positionSample)

	case events.WeaponFire:
		if state.LiveEnemyCount == 0 || e.Shooter == nil || isNonGunWeapon(e.Weapon) {
			return
		}
		a.handleWeaponFire(e, state)

	case events.PlayerHurt:
		if state.LiveEnemyCount == 0 || e.Attacker == nil || e.Player == nil {
			return
		}
		a.handlePlayerHurt(e, state)

	case events.Kill:
		if e.Killer == nil || e.Victim == nil {
			return
		}
		a.handleKill(e, state)
	}
}


func isNonGunWeapon(weapon *common.Equipment) bool {
	if weapon == nil {
		return true
	}
	return weapon.Class() == common.EqClassGrenade || weapon.Class() == common.EqClassEquipment
}

func (a *GunfightAnalyzer) handleWeaponFire(e events.WeaponFire, state *parser.GameState) {
	closestEnemy := a.getClosestEnemy(state, e.Shooter)
	if closestEnemy == nil {
		return
	}

	if e.Shooter.Name == a.targetPlayer {
		duel := a.getOrCreateDuel(state, closestEnemy, "shot")
		a.markCombat(duel, state.CurrentTick, "shot")
		duel.LastActivityTick = state.CurrentTick
		if duel.TargetWeapon == "" {
			duel.TargetWeapon = e.Weapon.String()
		}
		if duel.TargetFirstShotTick == 0 {
			duel.TargetFirstShotTick = state.CurrentTick
			a.captureFirstBulletAccuracy(duel, e.Shooter, closestEnemy)
		}
		a.updateDuelMovement(duel, state, e.Shooter, closestEnemy)
		return
	}

	if closestEnemy.Name == a.targetPlayer {
		duel := a.getOrCreateDuel(state, e.Shooter, "shot")
		a.markCombat(duel, state.CurrentTick, "shot")
		duel.LastActivityTick = state.CurrentTick
		if duel.EnemyWeapon == "" {
			duel.EnemyWeapon = e.Weapon.String()
		}
		if duel.EnemyFirstShotTick == 0 {
			duel.EnemyFirstShotTick = state.CurrentTick
		}
		a.updateDuelMovement(duel, state, closestEnemy, e.Shooter)
	}
}

func (a *GunfightAnalyzer) handlePlayerHurt(e events.PlayerHurt, state *parser.GameState) {
	if e.Attacker.Name == a.targetPlayer {
		duel := a.getOrCreateDuel(state, e.Player, "damage")
		a.markCombat(duel, state.CurrentTick, "damage")
		duel.LastActivityTick = state.CurrentTick
		duel.TargetDamage += e.HealthDamage
		duel.TargetHits++
		if duel.TargetFirstDamageTick == 0 {
			duel.TargetFirstDamageTick = state.CurrentTick
		}
		a.updateDuelMovement(duel, state, e.Attacker, e.Player)
		return
	}

	if e.Player.Name == a.targetPlayer {
		duel := a.getOrCreateDuel(state, e.Attacker, "damage")
		a.markCombat(duel, state.CurrentTick, "damage")
		duel.LastActivityTick = state.CurrentTick
		duel.EnemyDamage += e.HealthDamage
		duel.EnemyHits++
		if duel.EnemyFirstDamageTick == 0 {
			duel.EnemyFirstDamageTick = state.CurrentTick
		}
		a.updateDuelMovement(duel, state, e.Player, e.Attacker)
	}
}

func (a *GunfightAnalyzer) handleKill(e events.Kill, state *parser.GameState) {
	if e.Victim.Name == a.targetPlayer {
		duel := a.getOrCreateDuel(state, e.Killer, "kill")
		a.markCombat(duel, state.CurrentTick, "kill")
		duel.LastActivityTick = state.CurrentTick
		duel.ResolutionTick = state.CurrentTick
		duel.Outcome = "Lost"
		if duel.EnemyDamage == 0 {
			duel.EnemyDamage = maxInt(duel.TargetStartHP, 1)
			duel.EnemyHits = maxInt(duel.EnemyHits, 1)
			if duel.EnemyFirstDamageTick == 0 {
				duel.EnemyFirstDamageTick = state.CurrentTick
			}
		}
		a.updateDuelMovement(duel, state, e.Victim, e.Killer)
		a.resolveDuel(state, duel, e.Killer.Name)
		delete(a.activeDuels, e.Killer.UserID)
		return
	}

	if e.Killer.Name == a.targetPlayer {
		duel := a.getOrCreateDuel(state, e.Victim, "kill")
		a.markCombat(duel, state.CurrentTick, "kill")
		duel.LastActivityTick = state.CurrentTick
		duel.ResolutionTick = state.CurrentTick
		duel.Outcome = "Won"
		if duel.TargetDamage == 0 {
			duel.TargetDamage = maxInt(duel.EnemyStartHP, 1)
			duel.TargetHits = maxInt(duel.TargetHits, 1)
			if duel.TargetFirstDamageTick == 0 {
				duel.TargetFirstDamageTick = state.CurrentTick
			}
		}
		a.updateDuelMovement(duel, state, e.Killer, e.Victim)
		a.resolveDuel(state, duel, a.targetPlayer)
		delete(a.activeDuels, e.Victim.UserID)
	}
}

func (a *GunfightAnalyzer) markCombat(duel *Gunfight, tick int, source string) {
	if duel.CombatStartTick == 0 {
		duel.CombatStartTick = tick
		if duel.StartSource == "" || duel.StartSource == "vision" {
			duel.StartSource = source
		}
	}
}

func (a *GunfightAnalyzer) OnTickDone(state *parser.GameState) {
	a.recordPositions(state)
	targetPlayer := getPlayerByName(state, a.targetPlayer)
	if targetPlayer == nil || !targetPlayer.IsAlive() || state.LiveEnemyCount == 0 {
		a.flushCombatDuels(state, "Reset")
		return
	}

	targetEyes, ok := targetPlayer.PositionEyes()
	if !ok {
		return
	}

	seenThisTick := make(map[int]bool)
	for _, enemy := range state.Parser.GameState().Participants().Playing() {
		if enemy.Team == targetPlayer.Team || !enemy.IsAlive() {
			continue
		}

		targetPitch, targetYaw, targetTotal, ok := aimOffset(targetPlayer, targetEyes, enemy)
		if !ok {
			continue
		}
		enemyEyes, ok := enemy.PositionEyes()
		if !ok {
			continue
		}
		_, _, enemyTotal, ok := aimOffset(enemy, enemyEyes, targetPlayer)
		if !ok {
			continue
		}

		targetSeesEnemy := targetPitch < contactFOVDegrees && targetYaw < contactFOVDegrees
		enemySeesTarget := enemyTotal < contactFOVDegrees*2
		if targetSeesEnemy || enemySeesTarget {
			duel := a.getOrCreateDuel(state, enemy, "vision")
			duel.LastActivityTick = state.CurrentTick
			seenThisTick[enemy.UserID] = true

			if targetSeesEnemy && duel.FirstTargetSeenTick == 0 {
				duel.FirstTargetSeenTick = state.CurrentTick
				duel.InitialAimOffset = targetTotal
				duel.CrosshairPitchDiff = targetPitch
				if targetPlayer.ViewDirectionX() > angleToHeadPitch(targetEyes, enemy) {
					duel.CrosshairDirection = "too low (at chest/feet)"
				} else {
					duel.CrosshairDirection = "too high"
				}
			}
			if enemySeesTarget && duel.FirstEnemySeenTick == 0 {
				duel.FirstEnemySeenTick = state.CurrentTick
			}
			a.updateDuelMovement(duel, state, targetPlayer, enemy)
		}
	}

	for enemyID, duel := range a.activeDuels {
		if seenThisTick[enemyID] {
			continue
		}
		if !duel.hasCombat() && state.CurrentTick-duel.LastActivityTick > visibilityStaleTicks {
			delete(a.activeDuels, enemyID)
			continue
		}
		if duel.hasCombat() && state.CurrentTick-duel.LastActivityTick > combatStaleTicks {
			duel.ResolutionTick = state.CurrentTick
			duel.Outcome = "Reset"
			a.resolveDuel(state, duel, a.targetPlayer)
			delete(a.activeDuels, enemyID)
		}
	}
}

func (a *GunfightAnalyzer) recordPositions(state *parser.GameState) {
	for _, p := range state.Parser.GameState().Participants().Playing() {
		if !p.IsAlive() {
			continue
		}
		history := append(a.positionHistory[p.UserID], positionSample{Tick: state.CurrentTick, Pos: p.Position()})
		if len(history) > positionHistoryTicks {
			history = history[len(history)-positionHistoryTicks:]
		}
		a.positionHistory[p.UserID] = history
	}
}

func (a *GunfightAnalyzer) getOrCreateDuel(state *parser.GameState, enemy *common.Player, source string) *Gunfight {
	if duel, exists := a.activeDuels[enemy.UserID]; exists {
		return duel
	}

	targetPlayer := getPlayerByName(state, a.targetPlayer)
	targetHP := 100
	if targetPlayer != nil {
		targetHP = targetPlayer.Health()
	}

	duel := &Gunfight{
		EnemyID:          enemy.UserID,
		EnemyName:        enemy.Name,
		LastActivityTick: state.CurrentTick,
		StartSource:      source,
		TargetStartHP:    targetHP,
		EnemyStartHP:     enemy.Health(),
	}
	if source == "vision" {
		duel.LastActivityTick = state.CurrentTick
	} else {
		duel.CombatStartTick = state.CurrentTick
	}
	a.updateDuelMovement(duel, state, targetPlayer, enemy)
	a.activeDuels[enemy.UserID] = duel
	return duel
}

func (duel *Gunfight) hasCombat() bool {
	return duel.CombatStartTick > 0 || duel.TargetFirstShotTick > 0 || duel.EnemyFirstShotTick > 0 || duel.TargetDamage > 0 || duel.EnemyDamage > 0
}

func (a *GunfightAnalyzer) flushCombatDuels(state *parser.GameState, outcome string) {
	for enemyID, duel := range a.activeDuels {
		if duel.hasCombat() {
			duel.ResolutionTick = state.CurrentTick
			duel.Outcome = outcome
			a.resolveDuel(state, duel, a.targetPlayer)
		}
		delete(a.activeDuels, enemyID)
	}
}

func (a *GunfightAnalyzer) updateDuelMovement(duel *Gunfight, state *parser.GameState, targetPlayer *common.Player, enemy *common.Player) {
	anchorTick := duel.contactTick()
	if anchorTick == 0 {
		anchorTick = state.CurrentTick
	}
	targetMove := a.movementProfile(targetPlayer, anchorTick)
	enemyMove := a.movementProfile(enemy, anchorTick)

	if targetMove.Distance > duel.TargetMoveDist {
		duel.TargetMoveDist = targetMove.Distance
	}
	if enemyMove.Distance > duel.EnemyMoveDist {
		duel.EnemyMoveDist = enemyMove.Distance
	}
	if targetMove.SpeedAtContact > duel.TargetSpeed {
		duel.TargetSpeed = targetMove.SpeedAtContact
	}
	if enemyMove.SpeedAtContact > duel.EnemySpeed {
		duel.EnemySpeed = enemyMove.SpeedAtContact
	}
	if targetMove.Samples > duel.TargetMoveSamples {
		duel.TargetMoveSamples = targetMove.Samples
	}
	if enemyMove.Samples > duel.EnemyMoveSamples {
		duel.EnemyMoveSamples = enemyMove.Samples
	}

	duel.TargetWasPeeking = duel.TargetMoveDist >= peekMovementThreshold || duel.TargetSpeed >= peekSpeedThreshold
	duel.EnemyWasPeeking = duel.EnemyMoveDist >= peekMovementThreshold || duel.EnemySpeed >= peekSpeedThreshold
}

func (duel *Gunfight) contactTick() int {
	return firstPositive(duel.FirstTargetSeenTick, duel.FirstEnemySeenTick, duel.CombatStartTick)
}

func (a *GunfightAnalyzer) movementProfile(player *common.Player, anchorTick int) movementProfile {
	if player == nil || anchorTick == 0 {
		return movementProfile{}
	}
	history := a.positionHistory[player.UserID]
	if len(history) == 0 {
		return movementProfile{}
	}

	startTick := anchorTick - movementLookbackTicks
	endTick := anchorTick + movementLookaheadTicks
	current := player.Position()
	var samples []positionSample
	for _, sample := range history {
		if sample.Tick >= startTick && sample.Tick <= endTick {
			samples = append(samples, sample)
		}
	}
	if len(samples) == 0 {
		return movementProfile{}
	}

	var maxDist float64
	for _, sample := range samples {
		dist := distance2D(current, sample.Pos)
		if dist > maxDist {
			maxDist = dist
		}
	}

	var before *positionSample
	var after *positionSample
	for i := range samples {
		sample := &samples[i]
		if sample.Tick <= anchorTick {
			before = sample
		}
		if sample.Tick >= anchorTick {
			after = sample
			break
		}
	}
	if before == nil {
		before = &samples[0]
	}
	if after == nil {
		after = &samples[len(samples)-1]
	}

	speed := 0.0
	if after.Tick != before.Tick {
		tickRate := 64.0
		speed = distance2D(after.Pos, before.Pos) * tickRate / math.Abs(float64(after.Tick-before.Tick))
	}

	return movementProfile{Distance: maxDist, SpeedAtContact: speed, Samples: len(samples)}
}

func distance2D(a, b r3.Vector) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(float64(dx*dx + dy*dy))
}

func (a *GunfightAnalyzer) captureFirstBulletAccuracy(duel *Gunfight, shooter *common.Player, enemy *common.Player) {
	targetEyes, ok1 := shooter.PositionEyes()
	enemyEyes, ok2 := enemy.PositionEyes()
	if !ok1 || !ok2 {
		return
	}
	pitchToHead, yawToHead := calculateAngles(targetEyes, enemyEyes)
	pDiff := math.Abs(float64(shooter.ViewDirectionX() - pitchToHead))
	yDiff := math.Abs(float64(shooter.ViewDirectionY() - yawToHead))
	if yDiff > 180 {
		yDiff = 360 - yDiff
	}
	duel.FirstBulletAcc = math.Sqrt(pDiff*pDiff + yDiff*yDiff)
	if duel.InitialAimOffset > 0 {
		duel.AdjustmentNeeded = math.Abs(duel.InitialAimOffset - duel.FirstBulletAcc)
	}
}

func aimOffset(player *common.Player, eyes r3.Vector, target *common.Player) (float64, float64, float64, bool) {
	targetEyes, ok := target.PositionEyes()
	if !ok {
		return 0, 0, 0, false
	}
	pitch, yaw := calculateAngles(eyes, targetEyes)
	pDiff := math.Abs(float64(player.ViewDirectionX() - pitch))
	yDiff := math.Abs(float64(player.ViewDirectionY() - yaw))
	if yDiff > 180 {
		yDiff = 360 - yDiff
	}
	return pDiff, yDiff, math.Sqrt(pDiff*pDiff + yDiff*yDiff), true
}

func angleToHeadPitch(eyes r3.Vector, target *common.Player) float32 {
	targetEyes, ok := target.PositionEyes()
	if !ok {
		return 0
	}
	pitch, _ := calculateAngles(eyes, targetEyes)
	return pitch
}

func (a *GunfightAnalyzer) getClosestEnemy(state *parser.GameState, player *common.Player) *common.Player {
	var closestEnemy *common.Player
	minAngle := math.MaxFloat64
	eyes, ok := player.PositionEyes()
	if !ok {
		return nil
	}

	for _, p := range state.Parser.GameState().Participants().Playing() {
		if p.Team == player.Team || !p.IsAlive() {
			continue
		}
		_, _, totalAngleDiff, ok := aimOffset(player, eyes, p)
		if !ok {
			continue
		}
		if totalAngleDiff < minAngle {
			minAngle = totalAngleDiff
			closestEnemy = p
		}
	}

	if minAngle > shotAssignmentFOV {
		return nil
	}
	return closestEnemy
}

func (a *GunfightAnalyzer) resolveDuel(state *parser.GameState, duel *Gunfight, winner string) {
	if !duel.hasCombat() {
		return
	}
	tickRate := state.Parser.TickRate()
	if tickRate == 0 {
		tickRate = 64
	}
	anchorTick := duel.anchorTick()
	tickToMs := func(t int) float64 {
		if t == 0 || anchorTick == 0 {
			return -1
		}
		return float64(t-anchorTick) * (1000.0 / tickRate)
	}

	if duel.Outcome == "" {
		if winner == a.targetPlayer {
			duel.Outcome = "Won"
		} else if winner != "" {
			duel.Outcome = "Lost"
		} else {
			duel.Outcome = "Reset"
		}
	}

	meta := GunfightMetadata{
		TargetShotMs:             tickToMs(duel.TargetFirstShotTick),
		EnemyShotMs:              tickToMs(duel.EnemyFirstShotTick),
		TargetTTDMs:              tickToMs(duel.TargetFirstDamageTick),
		EnemyTTDMs:               tickToMs(duel.EnemyFirstDamageTick),
		TargetSeenMs:             tickToMs(duel.FirstTargetSeenTick),
		EnemySeenMs:              tickToMs(duel.FirstEnemySeenTick),
		CombatStartMs:            tickToMs(duel.CombatStartTick),
		ResolutionMs:             tickToMs(duel.ResolutionTick),
		CrosshairPitch:           duel.CrosshairPitchDiff,
		CrosshairDir:             duel.CrosshairDirection,
		InitialAimOff:            duel.InitialAimOffset,
		FirstBulletAcc:           duel.FirstBulletAcc,
		Adjustment:               duel.AdjustmentNeeded,
		Winner:                   winner,
		Outcome:                  duel.Outcome,
		StartSource:              duel.StartSource,
		TimingConfidence:         duel.timingConfidence(),
		ClassificationConfidence: duel.classificationConfidence(),
		WasPeeking:               duel.TargetWasPeeking,
		FightType:                duel.fightType(),
		TargetMovementDist:       duel.TargetMoveDist,
		EnemyMovementDist:        duel.EnemyMoveDist,
		TargetSpeed:              duel.TargetSpeed,
		EnemySpeed:               duel.EnemySpeed,
		TargetDamage:             duel.TargetDamage,
		EnemyDamage:              duel.EnemyDamage,
		TargetHits:               duel.TargetHits,
		EnemyHits:                duel.EnemyHits,
		TargetWeapon:             duel.TargetWeapon,
		EnemyWeapon:              duel.EnemyWeapon,
		TargetStartHP:            duel.TargetStartHP,
		EnemyStartHP:             duel.EnemyStartHP,
		Tags:                     []string{},
	}

	rating, analysis := evaluateDuel(duel, &meta, winner == a.targetPlayer)
	meta.Rating = rating
	meta.Analysis = analysis

	metaBytes, _ := json.Marshal(meta)
	severity := duel.severity(rating)
	desc := fmt.Sprintf("Duel vs %s (%s)", duel.EnemyName, duel.Outcome)
	if duel.Outcome == "Reset" {
		desc = fmt.Sprintf("Duel vs %s (Reset)", duel.EnemyName)
	}

	a.insights = append(a.insights, parser.InsightData{
		Round:       state.CurrentRound,
		Tick:        anchorTick,
		Type:        "Gunfight",
		Severity:    severity,
		Description: desc,
		Metadata:    string(metaBytes),
	})
}

func (duel *Gunfight) anchorTick() int {
	return firstPositive(duel.FirstTargetSeenTick, duel.FirstEnemySeenTick, duel.CombatStartTick, duel.ResolutionTick)
}

func (duel *Gunfight) timingConfidence() string {
	if duel.FirstTargetSeenTick > 0 || duel.FirstEnemySeenTick > 0 {
		return "high"
	}
	if duel.TargetFirstShotTick > 0 || duel.EnemyFirstShotTick > 0 {
		return "medium"
	}
	return "low"
}

func (duel *Gunfight) classificationConfidence() string {
	if duel.TargetMoveSamples >= 8 && duel.EnemyMoveSamples >= 8 {
		return "high"
	}
	if duel.TargetMoveSamples >= 3 || duel.EnemyMoveSamples >= 3 {
		return "medium"
	}
	return "low"
}

func (duel *Gunfight) fightType() string {
	if duel.classificationConfidence() == "low" {
		return "Unknown"
	}
	targetState := "Hold"
	if duel.TargetWasPeeking {
		targetState = "Peek"
	}
	enemyState := "Hold"
	if duel.EnemyWasPeeking {
		enemyState = "Peek"
	}
	return targetState + " vs " + enemyState
}

func (duel *Gunfight) severity(rating int) string {
	if duel.Outcome == "Won" {
		if rating <= 4 {
			return "Medium"
		}
		return "Low"
	}
	if duel.Outcome == "Reset" {
		return "Low"
	}
	if rating <= 3 {
		return "High"
	}
	if rating <= 6 {
		return "Medium"
	}
	return "Low"
}

func evaluateDuel(duel *Gunfight, meta *GunfightMetadata, won bool) (int, string) {
	rating := 5
	var details []string

	if meta.Outcome == "Reset" {
		if meta.TargetDamage > 0 || meta.EnemyDamage > 0 {
			meta.Tags = append(meta.Tags, "Tag / Reset")
			details = append(details, "0 Fight reset after damage was exchanged.")
		} else if meta.TargetShotMs >= 0 || meta.EnemyShotMs >= 0 {
			meta.Tags = append(meta.Tags, "Missed Reset")
			details = append(details, "0 Fight reset after shots without damage.")
		}
		return rating, strings.TrimSpace(strings.Join(details, " "))
	}

	if won {
		rating += 2
		details = append(details, "+2 You won the duel.")
		if meta.TimingConfidence == "high" && meta.TargetTTDMs >= 0 && meta.TargetTTDMs < 300 {
			rating += 2
			details = append(details, fmt.Sprintf("+2 Fast damage after contact (%.0fms).", meta.TargetTTDMs))
			meta.Tags = append(meta.Tags, "Fast TTD")
		} else if meta.TargetDamage >= 100 {
			details = append(details, "Solid kill.")
		}
		if meta.TargetStartHP < meta.EnemyStartHP-20 {
			rating += 2
			details = append(details, "+2 Won from a health disadvantage.")
			meta.Tags = append(meta.Tags, "Disadvantage")
		}
		if meta.EnemyDamage >= 80 {
			rating -= 2
			details = append(details, "-2 You barely survived this duel.")
			meta.Tags = append(meta.Tags, "Close Call")
		}
	} else {
		if meta.TargetDamage == 0 {
			if meta.EnemyDamage >= 100 && meta.EnemyHits == 1 && meta.TargetShotMs < 0 {
				details = append(details, "0 You were instantly one-tapped before you could respond.")
				meta.Tags = append(meta.Tags, "Insta-killed")
			} else {
				rating -= 3
				details = append(details, "-3 You dealt 0 damage in this fight.")
				meta.Tags = append(meta.Tags, "No Damage")
			}
		} else if meta.TargetDamage >= 80 {
			rating += 2
			details = append(details, fmt.Sprintf("+2 Close fight: you dealt %d damage in %d hits.", meta.TargetDamage, meta.TargetHits))
			meta.Tags = append(meta.Tags, "Close Call", "Aim Duel")
		} else {
			rating -= 1
			details = append(details, fmt.Sprintf("-1 You traded limited damage (%d in %d hits).", meta.TargetDamage, meta.TargetHits))
			meta.Tags = append(meta.Tags, "Traded Damage")
		}
		if meta.TargetStartHP <= 20 {
			rating += 2
			details = append(details, fmt.Sprintf("+2 You started at critical health (%d HP).", meta.TargetStartHP))
		}
		if meta.TimingConfidence == "high" && meta.TargetShotMs >= 0 && meta.EnemyShotMs >= 0 && meta.TargetShotMs > meta.EnemyShotMs+100 {
			rating -= 1
			details = append(details, fmt.Sprintf("-1 You fired %.0fms after the enemy.", meta.TargetShotMs-meta.EnemyShotMs))
		}
	}

	if meta.FirstBulletAcc > 10.0 {
		rating -= 2
		details = append(details, "-2 First bullet was far off target (>10°).")
	} else if meta.FirstBulletAcc > 5.0 {
		rating -= 1
		details = append(details, "-1 First bullet accuracy was poor (>5° off).")
	} else if meta.TimingConfidence == "high" && meta.FirstBulletAcc > 0 && meta.FirstBulletAcc <= 2.0 && !meta.WasPeeking {
		rating += 1
		details = append(details, "+1 Strong first bullet accuracy while holding.")
	}

	if meta.TimingConfidence == "low" {
		meta.Tags = append(meta.Tags, "Low Timing Confidence")
	}
	if meta.ClassificationConfidence == "low" {
		meta.Tags = append(meta.Tags, "Low Movement Confidence")
	}

	if rating < 1 {
		rating = 1
	} else if rating > 10 {
		rating = 10
	}

	return rating, strings.TrimSpace(strings.Join(details, " "))
}

func firstPositive(values ...int) int {
	best := 0
	for _, value := range values {
		if value > 0 && (best == 0 || value < best) {
			best = value
		}
	}
	return best
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *GunfightAnalyzer) GetInsights() []parser.InsightData {
	return a.insights
}
