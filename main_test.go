package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOperatorCommandsUpdateSharedWorld(t *testing.T) {
	op := &player{id: 1, name: "Admin", op: true, send: make(chan []byte, 16)}
	member := &player{id: 2, name: "Player", send: make(chan []byte, 16)}
	s := &server{
		mode:         "survival",
		wtime:        0.3,
		pvpKnockback: defaultPvpKnockback,
		players:      map[int]*player{op.id: op, member.id: member},
	}

	s.command(op, "/pvp on")
	if !s.pvp {
		t.Fatal("PvP command did not enable PvP")
	}
	s.command(op, "/gamemode creative")
	if s.mode != "creative" {
		t.Fatalf("mode = %q, want creative", s.mode)
	}
	s.command(op, "/time set 18:30")
	if s.wtime != 18.5/24 {
		t.Fatalf("time = %v, want %v", s.wtime, 18.5/24)
	}
	s.command(op, "/knockback 150")
	if s.pvpKnockback != 1.5 {
		t.Fatalf("pvp knockback = %v, want 1.5", s.pvpKnockback)
	}

	foundPvP, foundMode, foundTime, foundKnockback := false, false, false, false
	for len(member.send) > 0 {
		var message map[string]any
		if err := json.Unmarshal(<-member.send, &message); err != nil {
			t.Fatalf("invalid broadcast JSON: %v", err)
		}
		switch message["t"] {
		case "pvp-mode":
			foundPvP = message["enabled"] == true
		case "gamemode":
			foundMode = message["mode"] == "creative"
		case "tm":
			foundTime = message["time"] == 18.5/24
		case "pvp-knockback":
			foundKnockback = message["value"] == 1.5
		}
	}
	if !foundPvP || !foundMode || !foundTime || !foundKnockback {
		t.Fatalf("missing setting broadcasts: pvp=%v mode=%v time=%v knockback=%v", foundPvP, foundMode, foundTime, foundKnockback)
	}
}

func TestGiveAndOpCommands(t *testing.T) {
	op := &player{id: 1, name: "Admin", op: true, send: make(chan []byte, 8)}
	member := &player{id: 2, name: "Player Two", send: make(chan []byte, 8)}
	s := &server{players: map[int]*player{op.id: op, member.id: member}}

	s.command(op, "/give Player Two iron ingot 3")
	var grant map[string]any
	if err := json.Unmarshal(<-member.send, &grant); err != nil {
		t.Fatalf("invalid grant JSON: %v", err)
	}
	if grant["t"] != "grant" || grant["id"] != float64(110) || grant["count"] != float64(3) {
		t.Fatalf("grant = %#v, want 3 iron ingots", grant)
	}

	s.command(op, "/op Player Two")
	if !member.op {
		t.Fatal("/op did not grant operator status")
	}
	s.command(op, "/deop Player Two")
	if member.op {
		t.Fatal("/deop did not remove operator status")
	}
}

func TestPvpAttackIncludesConfiguredKnockback(t *testing.T) {
	attacker := &player{id: 1, name: "Attacker", x: 0, y: 0, z: 0, send: make(chan []byte, 2)}
	target := &player{id: 2, name: "Target", x: 0, y: 0, z: -2, health: maxHealth, send: make(chan []byte, 2)}
	s := &server{
		pvp:          true,
		pvpKnockback: 1.7,
		players:      map[int]*player{attacker.id: attacker, target.id: target},
	}
	s.pvpAttack(attacker, target.id, &inMsg{Reach: 4, Dmg: 4})
	var message map[string]any
	if err := json.Unmarshal(<-target.send, &message); err != nil {
		t.Fatalf("invalid PvP attack JSON: %v", err)
	}
	if message["t"] != "pvp-health" || message["knockback"] != 1.7 {
		t.Fatalf("PvP attack response = %#v, want knockback 1.7", message)
	}
}

func TestPvpAttackAppliesWeaponKnockbackMultiplier(t *testing.T) {
	attacker := &player{id: 1, name: "Attacker", x: 0, y: 0, z: 0, send: make(chan []byte, 2)}
	target := &player{id: 2, name: "Target", x: 0, y: 0, z: -2, health: maxHealth, send: make(chan []byte, 2)}
	s := &server{
		pvp:          true,
		pvpKnockback: 1.5,
		players:      map[int]*player{attacker.id: attacker, target.id: target},
	}
	weaponKnockback := 2.0
	s.pvpAttack(attacker, target.id, &inMsg{Reach: 4, Dmg: 4, KB: &weaponKnockback})
	var message map[string]any
	if err := json.Unmarshal(<-target.send, &message); err != nil {
		t.Fatalf("invalid PvP attack JSON: %v", err)
	}
	if message["knockback"] != 3.0 {
		t.Fatalf("PvP attack response knockback = %v, want 3", message["knockback"])
	}
}

func TestPvpHealIsRateLimitedAndServerAuthoritative(t *testing.T) {
	member := &player{id: 2, health: 10, send: make(chan []byte, 2)}
	s := &server{pvp: true}
	s.pvpHeal(member, 3)
	if member.health != 13 {
		t.Fatalf("health after heal = %d, want 13", member.health)
	}
	var message map[string]any
	if err := json.Unmarshal(<-member.send, &message); err != nil {
		t.Fatalf("invalid heal JSON: %v", err)
	}
	if message["t"] != "pvp-heal" || message["health"] != float64(13) {
		t.Fatalf("heal response = %#v, want authoritative health 13", message)
	}
	s.pvpHeal(member, 3)
	if member.health != 13 {
		t.Fatalf("rate-limited heal changed health to %d", member.health)
	}
	select {
	case extra := <-member.send:
		t.Fatalf("rate-limited heal sent another message: %s", extra)
	default:
	}
}

func TestPvpDamageIsRateLimitedAndServerAuthoritative(t *testing.T) {
	member := &player{id: 2, health: 10, send: make(chan []byte, 2)}
	s := &server{pvp: true}
	s.pvpDamage(member, 3.4)
	if member.health != 7 || member.dead {
		t.Fatalf("player state after damage = health %d, dead %v; want 7 and alive", member.health, member.dead)
	}
	var message map[string]any
	if err := json.Unmarshal(<-member.send, &message); err != nil {
		t.Fatalf("invalid damage JSON: %v", err)
	}
	if message["t"] != "pvp-health" || message["health"] != float64(7) ||
		message["amount"] != float64(3) || message["knockback"] != float64(0) {
		t.Fatalf("damage response = %#v, want authoritative health 7 and no knockback", message)
	}
	s.pvpDamage(member, 3)
	if member.health != 7 {
		t.Fatalf("rate-limited damage changed health to %d", member.health)
	}
	select {
	case extra := <-member.send:
		t.Fatalf("rate-limited damage sent another message: %s", extra)
	default:
	}
}

func TestOnlyOperatorCanChangeSharedWorldTime(t *testing.T) {
	op := &player{id: 1, op: true, joined: true, send: make(chan []byte, 2)}
	member := &player{id: 2, joined: true, send: make(chan []byte, 2)}
	s := &server{
		wtime:   0.25,
		players: map[int]*player{op.id: op, member.id: member},
	}

	s.handle(member, &inMsg{T: "pvp-time", Time: 18})
	if s.wtime != 0.25 {
		t.Fatalf("non-operator changed world time to %v", s.wtime)
	}
	if len(member.send) == 0 {
		t.Fatal("server did not explain why the time change was rejected")
	}

	s.handle(op, &inMsg{T: "pvp-time", Time: 18})
	if s.wtime != 0.75 {
		t.Fatalf("operator-set world time = %v, want 0.75", s.wtime)
	}
}

func TestNonOperatorCannotChangeSettings(t *testing.T) {
	admin := &player{id: 1, name: "Admin", op: true, send: make(chan []byte, 4)}
	member := &player{id: 2, name: "Player", send: make(chan []byte, 4)}
	s := &server{mode: "survival", players: map[int]*player{admin.id: admin, member.id: member}}

	s.command(member, "/gamemode creative")
	if s.mode != "survival" {
		t.Fatalf("non-operator changed mode to %q", s.mode)
	}
	var message map[string]any
	if err := json.Unmarshal(<-member.send, &message); err != nil {
		t.Fatalf("invalid error JSON: %v", err)
	}
	if message["t"] != "c" || message["m"] != "Only an operator can use this command." {
		t.Fatalf("non-operator response = %#v", message)
	}
}

func TestParseWorldTimeAndItemNames(t *testing.T) {
	for _, test := range []struct {
		input string
		want  float64
	}{
		{input: "day", want: 0.25},
		{input: "night", want: 0.75},
		{input: "set 18:30", want: 18.5 / 24},
		{input: "set 6", want: 0.25},
	} {
		got, ok := parseWorldTime(strings.Fields(test.input))
		if !ok || got != test.want {
			t.Errorf("parseWorldTime(%q) = %v, %v; want %v, true", test.input, got, ok, test.want)
		}
	}
	if id, ok := parseItemID("iron ingot"); !ok || id != 110 {
		t.Errorf("parseItemID(iron ingot) = %d, %v; want 110, true", id, ok)
	}
}

func TestPvPKnockbackPersistsIncludingDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.json")
	s := &server{
		savePath:     path,
		seed:         10,
		mode:         "survival",
		pvpKnockback: 0,
		edits:        map[uint32]uint8{},
	}
	s.save(true)
	loaded := &server{savePath: path, edits: map[uint32]uint8{}}
	if !loaded.load() {
		t.Fatal("saved world could not be loaded")
	}
	if loaded.pvpKnockback != 0 {
		t.Fatalf("loaded knockback = %v, want 0", loaded.pvpKnockback)
	}

	if err := os.WriteFile(path, []byte(`{"seed":10,"mode":"survival","time":0.3,"edits":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := &server{savePath: path, edits: map[uint32]uint8{}}
	if !legacy.load() || legacy.pvpKnockback != defaultPvpKnockback {
		t.Fatalf("legacy save loaded knockback = %v, want default %v", legacy.pvpKnockback, defaultPvpKnockback)
	}
}

func TestHandleSettingsRequiresOperatorAndBroadcasts(t *testing.T) {
	member := &player{send: make(chan []byte, 8)}
	s := &server{
		operatorCode: "secret",
		mode:         "survival",
		wtime:        0.25,
		players:      map[int]*player{1: member},
	}

	unauthorized := httptest.NewRequest(http.MethodPost, "/api/settings",
		strings.NewReader(`{"mode":"creative","pvp":true,"time":0.5}`))
	unauthorizedResponse := httptest.NewRecorder()
	s.handleSettings(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorizedResponse.Code, http.StatusUnauthorized)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/settings",
		strings.NewReader(`{"mode":"creative","pvp":true,"time":0.5,"pvpKnockback":1.5}`))
	request.Header.Set("X-Server-Op-Code", "secret")
	response := httptest.NewRecorder()
	s.handleSettings(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d, want %d: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if s.mode != "creative" || !s.pvp || s.wtime != 0.5 || s.pvpKnockback != 1.5 || !s.dirty {
		t.Fatalf("settings not updated: mode=%q pvp=%v time=%v knockback=%v dirty=%v", s.mode, s.pvp, s.wtime, s.pvpKnockback, s.dirty)
	}
	if len(member.send) != 4 {
		t.Fatalf("received %d broadcasts, want mode, PvP, time, and knockback updates", len(member.send))
	}

	getResponse := httptest.NewRecorder()
	s.handleSettings(getResponse, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if getResponse.Code != http.StatusOK ||
		!strings.Contains(getResponse.Body.String(), `"mode":"creative"`) ||
		!strings.Contains(getResponse.Body.String(), `"pvp":true`) ||
		!strings.Contains(getResponse.Body.String(), `"pvpKnockback":1.5`) {
		t.Fatalf("GET settings response = %d %q", getResponse.Code, getResponse.Body.String())
	}
}

func TestHandleOperatorChecksConfiguredCode(t *testing.T) {
	s := &server{operatorCode: "test-code"}
	for _, test := range []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "rejects wrong code", body: `{"code":"wrong"}`, wantStatus: http.StatusUnauthorized},
		{name: "accepts configured code", body: `{"code":"test-code"}`, wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/operator", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			s.handleOperator(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestHandleOperatorLimitsIncorrectCodes(t *testing.T) {
	s := &server{operatorCode: "test-code"}
	for attempt := 0; attempt < 8; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/operator", strings.NewReader(`{"code":"wrong"}`))
		response := httptest.NewRecorder()
		s.handleOperator(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", attempt+1, response.Code, http.StatusUnauthorized)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/operator", strings.NewReader(`{"code":"wrong"}`))
	response := httptest.NewRecorder()
	s.handleOperator(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked attempt status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
}

func TestHandleDefinitionsRequiresCodeAndPersists(t *testing.T) {
	dir := t.TempDir()
	recipient := &player{send: make(chan []byte, 2)}
	s := &server{
		definitionsDir: dir,
		operatorCode:   "test-code",
		players:        map[int]*player{1: recipient},
	}

	definition := `{"schema":"coop1.10.item-block-definition.v1","kind":"block","name":"Shared Block","registry":{"id":30}}`

	unauthorized := httptest.NewRequest(http.MethodPost, "/api/definitions", strings.NewReader(definition))
	unauthorizedResponse := httptest.NewRecorder()
	s.handleDefinitions(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized POST status = %d, want %d", unauthorizedResponse.Code, http.StatusUnauthorized)
	}

	invalidBlock := strings.Replace(definition, `"id":30`, `"id":256`, 1)
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/definitions", strings.NewReader(invalidBlock))
	invalidRequest.Header.Set("X-Server-Op-Code", "test-code")
	invalidResponse := httptest.NewRecorder()
	s.handleDefinitions(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range block ID status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/definitions", strings.NewReader(definition))
	request.Header.Set("X-Server-Op-Code", "test-code")
	response := httptest.NewRecorder()
	s.handleDefinitions(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authorized POST status = %d, want %d: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if message := string(<-recipient.send); !strings.Contains(message, `"t":"definition"`) {
		t.Fatalf("broadcast = %q, want shared definition message", message)
	}
	if _, err := os.Stat(filepath.Join(dir, "30.json")); err != nil {
		t.Fatalf("definition file was not saved: %v", err)
	}

	updated := strings.Replace(definition, "Shared Block", "Updated Block", 1)
	updateRequest := httptest.NewRequest(http.MethodPost, "/api/definitions", strings.NewReader(updated))
	updateRequest.Header.Set("X-Server-Op-Code", "test-code")
	updateResponse := httptest.NewRecorder()
	s.handleDefinitions(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusNoContent {
		t.Fatalf("definition update status = %d, want %d: %s", updateResponse.Code, http.StatusNoContent, updateResponse.Body.String())
	}
	if message := string(<-recipient.send); !strings.Contains(message, `"Updated Block"`) {
		t.Fatalf("updated definition broadcast = %q, want updated name", message)
	}

	getResponse := httptest.NewRecorder()
	s.handleDefinitions(getResponse, httptest.NewRequest(http.MethodGet, "/api/definitions", nil))
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), `"name":"Updated Block"`) {
		t.Fatalf("GET response = %d %q, want updated definition", getResponse.Code, getResponse.Body.String())
	}
}

func TestHandlePacksRequiresCodePersistsAndBroadcasts(t *testing.T) {
	dir := t.TempDir()
	recipient := &player{send: make(chan []byte, 2)}
	s := &server{
		packsDir:     dir,
		operatorCode: "test-code",
		players:      map[int]*player{1: recipient},
	}

	pack := `{"schema":"coop1.10.item-block-pack.v2","id":"test_pack","entries":[{"type":"feature","name":"chat","event":"onChat","handler":"chat"}],"javascript":{"runtime":"coop1.10.pack-javascript.v1","source":"({features:{chat(){}}})"}}`

	unauthorized := httptest.NewRecorder()
	s.handlePacks(unauthorized, httptest.NewRequest(http.MethodPost, "/api/packs", strings.NewReader(pack)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized POST status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/packs", strings.NewReader(pack))
	request.Header.Set("X-Server-Op-Code", "test-code")
	response := httptest.NewRecorder()
	s.handlePacks(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authorized POST status = %d, want %d: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if message := string(<-recipient.send); !strings.Contains(message, `"t":"pack"`) {
		t.Fatalf("broadcast = %q, want shared pack message", message)
	}
	if _, err := os.Stat(filepath.Join(dir, "test_pack.json")); err != nil {
		t.Fatalf("pack file was not saved: %v", err)
	}

	getResponse := httptest.NewRecorder()
	s.handlePacks(getResponse, httptest.NewRequest(http.MethodGet, "/api/packs", nil))
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), `"id":"test_pack"`) {
		t.Fatalf("GET response = %d %q, want persisted pack", getResponse.Code, getResponse.Body.String())
	}
}

func TestSharedDropsAreRelayedAndPickedUpOnce(t *testing.T) {
	first := &player{id: 1, joined: true, x: 10, y: 20, z: 10, send: make(chan []byte, 8)}
	second := &player{id: 2, joined: true, x: 10, y: 20, z: 10, send: make(chan []byte, 8)}
	s := &server{
		players: map[int]*player{first.id: first, second.id: second},
		drops:   map[uint64]*worldDrop{},
	}

	s.handle(first, &inMsg{
		T: "drop-spawn", ClientID: "1:1", Item: 600, Count: 3,
		X: 10.5, Y: 21, Z: 10.5, VY: 2, Age: 0.6,
	})
	if len(s.drops) != 1 {
		t.Fatalf("drop count = %d, want 1", len(s.drops))
	}
	var dropID uint64
	for id := range s.drops {
		dropID = id
	}
	if message := string(<-first.send); !strings.Contains(message, `"t":"drop-add"`) {
		t.Fatalf("drop spawn was not sent to owner: %q", message)
	}
	if message := string(<-second.send); !strings.Contains(message, `"item":600`) {
		t.Fatalf("drop spawn was not shared with other players: %q", message)
	}

	s.handle(second, &inMsg{T: "drop-pick", DropID: dropID, Count: 2})
	if got := s.drops[dropID].count; got != 1 {
		t.Fatalf("remaining drop count = %d, want 1", got)
	}
	update := string(<-first.send)
	if !strings.Contains(update, `"t":"drop-update"`) || !strings.Contains(update, `"count":1`) {
		t.Fatalf("partial pickup update = %q, want remaining shared count 1", update)
	}
	if message := string(<-second.send); !strings.Contains(message, `"t":"drop-update"`) {
		t.Fatalf("picker did not receive remaining drop update: %q", message)
	}
	if message := string(<-second.send); !strings.Contains(message, `"t":"drop-picked"`) ||
		!strings.Contains(message, `"count":2`) {
		t.Fatalf("picker grant = %q, want 2 picked items", message)
	}

	s.handle(second, &inMsg{T: "drop-pick", DropID: dropID, Count: 1})
	if _, exists := s.drops[dropID]; exists {
		t.Fatal("drop should be removed when its final item is picked up")
	}
	if message := string(<-first.send); !strings.Contains(message, `"t":"drop-remove"`) {
		t.Fatalf("final pickup removal was not broadcast: %q", message)
	}
	if message := string(<-second.send); !strings.Contains(message, `"t":"drop-remove"`) {
		t.Fatalf("picker did not receive final drop removal: %q", message)
	}
	if message := string(<-second.send); !strings.Contains(message, `"t":"drop-picked"`) {
		t.Fatalf("final pickup grant = %q, want picked item", message)
	}
}

func TestJoiningPlayerReceivesExistingSharedDrops(t *testing.T) {
	s := &server{
		players: map[int]*player{},
		drops: map[uint64]*worldDrop{
			7: {id: 7, ownerID: 9, clientID: "9:1", item: 600, count: 1, x: 12, y: 20, z: 15},
		},
		projectiles: map[uint64]*worldProjectile{
			8: {id: 8, ownerID: 9, clientID: "9:2", packID: "magic_pack", itemID: 601,
				hookScope: "item", hookKey: "MagicStaff", x: 12, y: 21, z: 15, dz: -1, speed: 16, lifetime: 5},
		},
	}

	joining := &player{send: make(chan []byte, 8)}

	s.join(joining, &inMsg{T: "hello", V: netVersion, Name: "New player"})

	found := false
	foundProjectile := false
	for len(joining.send) > 0 {
		message := string(<-joining.send)
		if strings.Contains(message, `"t":"drop-add"`) && strings.Contains(message, `"item":600`) {
			found = true
		}
		if strings.Contains(message, `"t":"projectile-add"`) && strings.Contains(message, `"packId":"magic_pack"`) {
			foundProjectile = true
		}
	}
	if !found {
		t.Fatal("joining player did not receive existing dropped items")
	}
	if !foundProjectile {
		t.Fatal("joining player did not receive active projectiles")
	}
	if got := s.drops[7].ownerID; got != joining.id {
		t.Fatalf("orphaned drop owner = %d, want joining player's ID %d", got, joining.id)
	}
}

func TestFaviconDoesNotReturnNotFound(t *testing.T) {
	response := httptest.NewRecorder()
	handleFavicon(response, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("favicon status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestSharedProjectilesAreRelayedAndRemoved(t *testing.T) {
	first := &player{id: 1, joined: true, x: 10, y: 20, z: 10, send: make(chan []byte, 8)}
	second := &player{id: 2, joined: true, x: 10, y: 20, z: 10, send: make(chan []byte, 8)}
	s := &server{
		players:     map[int]*player{first.id: first, second.id: second},
		projectiles: map[uint64]*worldProjectile{},
	}

	s.handle(first, &inMsg{
		T: "projectile-spawn", ClientID: "1:1", PackID: "magic_pack",
		ProjectileItem: 600, HookScope: "item", HookKey: "MagicStaff",
		X: 10, Y: 21.5, Z: 10, DZ: -1, Speed: 16, Lifetime: 5, ProjectileDamage: 3,
	})
	if len(s.projectiles) != 1 {
		t.Fatalf("projectile count = %d, want 1", len(s.projectiles))
	}
	var projectileID uint64
	for id := range s.projectiles {
		projectileID = id
	}
	s.projectiles[projectileID].lastUpdate = time.Now().Add(-time.Second)
	if message := string(<-first.send); !strings.Contains(message, `"t":"projectile-add"`) {
		t.Fatalf("projectile spawn was not sent to owner: %q", message)
	}
	if message := string(<-second.send); !strings.Contains(message, `"packId":"magic_pack"`) ||
		!strings.Contains(message, `"hookKey":"MagicStaff"`) {
		t.Fatalf("projectile details were not shared with other players: %q", message)
	}

	s.handle(first, &inMsg{T: "projectile-update", ProjectileID: projectileID, X: 10, Y: 21.5, Z: 8, Age: 0.1})
	if message := string(<-second.send); !strings.Contains(message, `"t":"projectile-update"`) ||
		!strings.Contains(message, `"z":8`) {
		t.Fatalf("projectile movement was not relayed: %q", message)
	}

	s.handle(second, &inMsg{T: "projectile-remove", ProjectileID: projectileID})
	if _, exists := s.projectiles[projectileID]; !exists {
		t.Fatal("a different player must not remove the projectile")
	}
	s.handle(first, &inMsg{T: "projectile-remove", ProjectileID: projectileID})
	if _, exists := s.projectiles[projectileID]; exists {
		t.Fatal("owner removal should delete the shared projectile")
	}
	if message := string(<-first.send); !strings.Contains(message, `"t":"projectile-remove"`) {
		t.Fatalf("projectile removal was not sent to owner: %q", message)
	}
	if message := string(<-second.send); !strings.Contains(message, `"t":"projectile-remove"`) {
		t.Fatalf("projectile removal was not broadcast: %q", message)
	}
}
