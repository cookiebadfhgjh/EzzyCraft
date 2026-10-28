package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type activeMod struct {
	File string `json:"file"`
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"-"`
}

func normalizeModFile(name string) (string, bool) {
	name = strings.TrimSpace(name)
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || name == "." || name == ".." {
		return "", false
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		name += ".json"
	}
	if !safeModFileName(name) {
		return "", false
	}
	return name, true
}

func safeModFileName(name string) bool {
	if len(name) < 6 || len(name) > 132 || !strings.HasSuffix(strings.ToLower(name), ".json") {
		return false
	}
	body := name[:len(name)-5]
	if body == "" {
		return false
	}
	for i, r := range body {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		if i > 0 && (r == '.' || r == '_' || r == '-') {
			continue
		}
		return false
	}
	return true
}

func inspectMod(data []byte) (activeMod, bool) {
	var meta struct {
		Schema   string `json:"schema"`
		Kind     string `json:"kind"`
		ID       string `json:"id"`
		Name     string `json:"name"`
		Registry struct {
			ID int `json:"id"`
		} `json:"registry"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return activeMod{}, false
	}
	switch meta.Schema {
	case "coop1.10.item-block-pack.v2":
		return activeMod{Kind: "pack", ID: meta.ID, Name: meta.Name}, true
	case "coop1.10.item-block-definition.v1", "coop1.10.item-block-definition.v2":
		return activeMod{Kind: "definition", ID: strconv.Itoa(meta.Registry.ID), Name: meta.Name}, true
	default:
		return activeMod{}, false
	}
}

func (s *server) searchModDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for _, dir := range []string{s.modsDir, s.packsDir, s.definitionsDir} {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

func (s *server) findModFile(name string) (string, []byte, activeMod, error) {
	safe, ok := normalizeModFile(name)
	if !ok {
		return "", nil, activeMod{}, errors.New("invalid filename")
	}
	for _, dir := range s.searchModDirs() {
		path := filepath.Join(dir, safe)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		info, ok := inspectMod(data)
		if !ok {
			continue
		}
		info.File = safe
		info.Path = path
		return path, data, info, nil
	}
	want := strings.ToLower(safe)
	for _, dir := range s.searchModDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.ToLower(entry.Name()) != want {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			info, ok := inspectMod(data)
			if !ok {
				continue
			}
			info.File = entry.Name()
			info.Path = path
			return path, data, info, nil
		}
	}
	return "", nil, activeMod{}, os.ErrNotExist
}

func (s *server) listAvailableMods() []activeMod {
	seen := map[string]bool{}
	var out []activeMod
	for _, dir := range s.searchModDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") ||
				!strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
				continue
			}
			key := strings.ToLower(entry.Name())
			if seen[key] {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			info, ok := inspectMod(data)
			if !ok {
				continue
			}
			info.File = entry.Name()
			info.Path = filepath.Join(dir, entry.Name())
			seen[key] = true
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].File) < strings.ToLower(out[j].File)
	})
	return out
}

func (s *server) loadActiveMods() {
	s.activeMods = map[string]activeMod{}
	if s.activePath == "" {
		return
	}
	data, err := os.ReadFile(s.activePath)
	if err != nil {
		return
	}
	var names []string
	if json.Unmarshal(data, &names) != nil {
		return
	}
	for _, name := range names {
		_, _, info, err := s.findModFile(name)
		if err != nil {
			continue
		}
		s.activeMods[info.File] = info
	}
}

func (s *server) saveActiveMods() {
	if s.activePath == "" {
		return
	}
	names := make([]string, 0, len(s.activeMods))
	for name := range s.activeMods {
		names = append(names, name)
	}
	sort.Strings(names)
	data, err := json.MarshalIndent(names, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.activePath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(s.activePath, data, 0o644)
}

func (s *server) rememberActiveLocked(info activeMod) {
	if s.activeMods == nil {
		s.activeMods = map[string]activeMod{}
	}
	s.activeMods[info.File] = info
	s.saveActiveMods()
}

func (s *server) activateMod(name string) (activeMod, []byte, error) {
	_, data, info, err := s.findModFile(name)
	if err != nil {
		return activeMod{}, nil, err
	}
	s.mu.Lock()
	s.rememberActiveLocked(info)
	if info.Kind == "pack" {
		s.broadcast(M{"t": "pack", "pack": json.RawMessage(data), "file": info.File}, nil)
	} else {
		s.broadcast(M{"t": "definition", "definition": json.RawMessage(data), "file": info.File}, nil)
	}
	s.mu.Unlock()
	return info, data, nil
}

func (s *server) deactivateMod(name string) (activeMod, error) {
	safe, ok := normalizeModFile(name)
	if !ok {
		return activeMod{}, errors.New("invalid filename")
	}
	s.mu.Lock()
	info, exists := s.activeMods[safe]
	if !exists {
		for file, current := range s.activeMods {
			if strings.EqualFold(file, safe) {
				info = current
				exists = true
				safe = file
				break
			}
		}
	}
	s.mu.Unlock()
	if !exists {
		_, _, found, err := s.findModFile(safe)
		if err != nil {
			return activeMod{}, os.ErrNotExist
		}
		info = found
		safe = found.File
	}
	s.mu.Lock()
	delete(s.activeMods, safe)
	s.saveActiveMods()
	if info.Kind == "pack" {
		s.broadcast(M{"t": "pack-remove", "id": info.ID, "file": info.File}, nil)
	} else {
		id, _ := strconv.Atoi(info.ID)
		s.broadcast(M{"t": "definition-remove", "id": id, "file": info.File}, nil)
	}
	s.mu.Unlock()
	return info, nil
}

func (s *server) activeKindPayloads(kind string) []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]json.RawMessage, 0, len(s.activeMods))
	for file, info := range s.activeMods {
		if info.Kind != kind {
			continue
		}
		path := info.Path
		if path == "" {
			path = file
		}
		data, err := os.ReadFile(path)
		if err != nil {
			_, foundData, found, findErr := s.findModFile(file)
			if findErr != nil {
				continue
			}
			info = found
			s.activeMods[info.File] = info
			data = foundData
		}
		out = append(out, json.RawMessage(data))
	}
	return out
}

func (s *server) isModActive(file string) bool {
	if s.activeMods == nil {
		return false
	}
	if _, ok := s.activeMods[file]; ok {
		return true
	}
	for name := range s.activeMods {
		if strings.EqualFold(name, file) {
			return true
		}
	}
	return false
}

func (s *server) handleMods(w http.ResponseWriter, r *http.Request) {
	setAPICORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch r.Method {
	case http.MethodGet:
		available := s.listAvailableMods()
		s.mu.Lock()
		active := make([]string, 0, len(s.activeMods))
		for name := range s.activeMods {
			active = append(active, name)
		}
		listed := make([]M, 0, len(available))
		for _, info := range available {
			listed = append(listed, M{
				"file":   info.File,
				"kind":   info.Kind,
				"id":     info.ID,
				"name":   info.Name,
				"active": s.isModActive(info.File),
			})
		}
		s.mu.Unlock()
		sort.Strings(active)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(M{"mods": listed, "active": active}); err != nil {
			log.Println("could not return mod catalog:", err)
		}
	case http.MethodPost:
		if s.operatorCode == "" {
			writeAPIError(w, http.StatusServiceUnavailable, "Server operator access is not configured.")
			return
		}
		authorized, blocked := s.checkOperatorCode(r, r.Header.Get("X-Server-Op-Code"))
		if blocked {
			writeAPIError(w, http.StatusTooManyRequests, "Too many incorrect operator-code attempts. Try again later.")
			return
		}
		if !authorized {
			writeAPIError(w, http.StatusUnauthorized, "Server operator access is required.")
			return
		}
		var body struct {
			File   string `json:"file"`
			Action string `json:"action"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil ||
			strings.TrimSpace(body.File) == "" {
			writeAPIError(w, http.StatusBadRequest, "Type the filename of a JSON file in the mods folder.")
			return
		}
		action := strings.ToLower(strings.TrimSpace(body.Action))
		if action == "" || action == "load" {
			info, data, err := s.activateMod(body.File)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					writeAPIError(w, http.StatusNotFound, "No mod file named "+body.File+" was found.")
					return
				}
				writeAPIError(w, http.StatusBadRequest, err.Error())
				return
			}
			resp := M{"ok": true, "file": info.File, "kind": info.Kind, "id": info.ID, "name": info.Name}
			if info.Kind == "pack" {
				resp["pack"] = json.RawMessage(data)
			} else {
				resp["definition"] = json.RawMessage(data)
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				log.Println("could not return loaded mod:", err)
			}
			return
		}
		if action == "unload" || action == "remove" {
			info, err := s.deactivateMod(body.File)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					writeAPIError(w, http.StatusNotFound, "No mod file named "+body.File+" was found.")
					return
				}
				writeAPIError(w, http.StatusBadRequest, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(M{
				"ok": true, "file": info.File, "kind": info.Kind, "id": info.ID, "removed": true,
			}); err != nil {
				log.Println("could not return unloaded mod:", err)
			}
			return
		}
		writeAPIError(w, http.StatusBadRequest, "Unknown mod action.")
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "Use GET or POST for the mod catalog.")
	}
}
