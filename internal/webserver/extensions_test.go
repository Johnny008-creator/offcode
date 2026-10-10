package webserver

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Johnny008-creator/offcode/internal/webengine"
)

func TestArtifactEndpointRejectsUnregisteredPaths(t *testing.T) {
	server := &Server{sess: &webengine.Session{}}
	for _, url := range []string{"/api/artifact?path=/etc/passwd", "/api/artifact?block=-1", "/api/artifact?block=0"} {
		response := httptest.NewRecorder()
		server.handleArtifact(response, httptest.NewRequest("GET", url, nil))
		if response.Code != 400 && response.Code != 404 {
			t.Fatalf("%s returned %d", url, response.Code)
		}
	}
}

func TestVoiceEndpointsValidateBeforeRunningTools(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	server.handleTranscribe(response, httptest.NewRequest("GET", "/api/transcribe", nil))
	if response.Code != 405 {
		t.Fatalf("GET transcribe status %d", response.Code)
	}
	response = httptest.NewRecorder()
	server.handleSpeak(response, httptest.NewRequest("POST", "/api/speak", strings.NewReader("invalid json")))
	if response.Code != 400 {
		t.Fatalf("invalid speech request status %d", response.Code)
	}
}
