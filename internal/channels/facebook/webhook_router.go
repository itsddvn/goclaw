package facebook

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

// webhookRouter routes incoming Facebook webhook events to the correct channel instance by page_id.
// A single HTTP handler is shared across all facebook channel instances on the same server.
type webhookRouter struct {
	mu           sync.RWMutex
	instances    map[string]*Channel // pageID → channel
	routeHandled bool                // true after the fixed route is claimed
}

var globalRouter = &webhookRouter{
	instances: make(map[string]*Channel),
}

func (r *webhookRouter) register(ch *Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.instances[ch.pageID]; exists {
		return fmt.Errorf("facebook: page_id %q is already registered", ch.pageID)
	}
	r.instances[ch.pageID] = ch
	return nil
}

func (r *webhookRouter) unregister(ch *Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.instances[ch.pageID] == ch {
		delete(r.instances, ch.pageID)
	}
}

// webhookRoute returns the fixed path and shared handler to its first claimant.
func (r *webhookRouter) webhookRoute() (string, http.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.routeHandled {
		r.routeHandled = true
		return webhookPath, r
	}
	return "", nil
}

// ClaimWebhookRoute claims the process-wide Facebook callback route. The gateway
// calls this even when no Facebook channel instance exists yet.
func ClaimWebhookRoute() (string, http.Handler) {
	return globalRouter.webhookRoute()
}

// ServeHTTP is the shared handler for all Facebook page webhooks.
func (r *webhookRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.handleVerification(w, req)
	case http.MethodPost:
		r.handleEvent(w, req)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *webhookRouter) handleVerification(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	if q.Get("hub.mode") != "subscribe" {
		http.Error(w, "invalid hub.mode", http.StatusForbidden)
		return
	}

	requestToken := q.Get("hub.verify_token")
	matched := false
	r.mu.RLock()
	for _, ch := range r.instances {
		if requestToken == ch.webhookH.verifyToken {
			matched = true
			break
		}
	}
	r.mu.RUnlock()
	if !matched {
		slog.Warn("security.facebook_webhook_verify_token_mismatch",
			"remote_addr", req.RemoteAddr)
		http.Error(w, "invalid verify token", http.StatusForbidden)
		return
	}

	challenge := q.Get("hub.challenge")
	if !hubChallengePattern.MatchString(challenge) {
		http.Error(w, "invalid challenge", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
}

func (r *webhookRouter) handleEvent(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(io.LimitReader(req.Body, maxWebhookBodyBytes+1))
	if err != nil {
		slog.Warn("facebook: webhook read body error", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	if len(body) > maxWebhookBodyBytes {
		slog.Warn("facebook: webhook body exceeded limit, event dropped", "bytes", len(body))
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Warn("facebook: webhook parse error", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	if payload.Object != "page" {
		w.WriteHeader(http.StatusOK)
		return
	}

	signature := req.Header.Get("X-Hub-Signature-256")
	signatureValid := make(map[*WebhookHandler]bool, len(payload.Entry))
	for _, entry := range payload.Entry {
		r.mu.RLock()
		target := r.instances[entry.ID]
		r.mu.RUnlock()
		if target == nil {
			slog.Warn("security.facebook_webhook_page_unknown",
				"page_id", entry.ID, "remote_addr", req.RemoteAddr)
			continue
		}
		valid, checked := signatureValid[target.webhookH]
		if !checked {
			valid = verifySignature(body, signature, target.webhookH.appSecret)
			signatureValid[target.webhookH] = valid
		}
		if !valid {
			slog.Warn("security.facebook_webhook_signature_invalid",
				"page_id", entry.ID, "remote_addr", req.RemoteAddr)
			continue
		}
		target.webhookH.dispatchEntry(req.Context(), entry)
	}

	w.WriteHeader(http.StatusOK)
}
