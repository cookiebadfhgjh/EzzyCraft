package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
