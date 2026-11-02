package fakeapi

import (
	"net/http"
	"sync"
)

// Revoker records the tokens posted to Google's revocation endpoint.
type Revoker struct {
	mu     sync.Mutex
	tokens []string
}

func (rv *Revoker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.ParseForm() != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rv.mu.Lock()
	defer rv.mu.Unlock()
	rv.tokens = append(rv.tokens, r.PostForm.Get("token"))
}

// Tokens returns a copy of the tokens revoked so far, in order.
func (rv *Revoker) Tokens() []string {
	rv.mu.Lock()
	defer rv.mu.Unlock()
	return append([]string(nil), rv.tokens...)
}
