package jev

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestEnvironmentClientDefaultsAndNoKey(t *testing.T) {
	client := New(cli.Env{LookupEnv: func(string) (string, bool) { return "", false }})
	testutil.Equal(t, client.HasKey(), false)
	testutil.Equal(t, client.endpoint, "https://api.typesafe.ai/v1/systemone")
	testutil.Equal(t, client.http.Timeout, 10*time.Second)
	testutil.Equal(t, client.delay, 100*time.Millisecond)
	replies := client.Run([][]byte{[]byte(`{}`)})
	testutil.Equal(t, replies[0].Reason, "TYPESAFE_API_KEY is not set")
	testutil.Equal(t, client.Redact("unchanged"), "unchanged")
}
func TestSharedClientConcurrencyAndRetry(t *testing.T) {
	var active, peak, calls atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		if current == Concurrency {
			once.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-time.After(time.Second):
			t.Error("requests not concurrent")
		}
		testutil.Equal(t, r.Header.Get("Authorization"), "Bearer secret")
		if n == 1 {
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"model":"secret-model","answers":{}}`)
	}))
	defer server.Close()
	bodies := make([][]byte, 12)
	for i := range bodies {
		bodies[i] = []byte(`{}`)
	}
	client := NewClient("secret", server.URL, server.Client(), 0)
	replies := client.Run(bodies)
	testutil.Equal(t, len(replies), 12)
	testutil.Equal(t, calls.Load(), int32(13))
	testutil.Equal(t, peak.Load(), int32(Concurrency))
	for _, reply := range replies {
		testutil.Equal(t, reply.Reason, "")
		testutil.Equal(t, reply.Answer.Model, "[redacted]-model")
	}
}
func TestSharedClientRejectsRedirectAndSanitizesErrors(t *testing.T) {
	var redirected atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client := New(cli.Env{LookupEnv: func(name string) (string, bool) {
		value, ok := map[string]string{"TYPESAFE_API_KEY": "secret", "TYPESAFE_API_URL": server.URL}[name]
		return value, ok
	}})
	result := client.Run([][]byte{[]byte(`{}`)})
	testutil.Equal(t, result[0].Reason, "TypeSafe HTTP 307")
	testutil.Equal(t, redirected.Load(), int32(0))
	for _, endpoint := range []string{"\x00secret", "http://127.0.0.1:0/secret"} {
		result = NewClient("secret", endpoint, &http.Client{Timeout: time.Second}, 0).Run([][]byte{[]byte(`{}`)})
		if result[0].Reason == "" || strings.Contains(result[0].Reason, "secret") {
			t.Fatalf("unsafe error: %s", result[0].Reason)
		}
	}
}
