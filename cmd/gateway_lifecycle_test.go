package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMountFacebookWebhookRoute_ClaimsWithZeroInstancesOnce(t *testing.T) {
	mux := http.NewServeMux()
	mountFacebookWebhookRoute(mux)

	request := httptest.NewRequest(http.MethodPost, "/v1/channels/facebook/webhook", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("zero-instance callback status = %d, want 200", response.Code)
	}

	// A second lifecycle claim is a no-op and must not panic from duplicate
	// ServeMux registration.
	mountFacebookWebhookRoute(mux)
	second := httptest.NewRecorder()
	mux.ServeHTTP(second, httptest.NewRequest(http.MethodPost,
		"/v1/channels/facebook/webhook", nil))
	if second.Code != http.StatusOK {
		t.Fatalf("callback status after second claim = %d, want 200", second.Code)
	}
}
