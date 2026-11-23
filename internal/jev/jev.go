package jev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
)

const Concurrency = 4
const Timeout = 10 * time.Second
const RetryDelay = 100 * time.Millisecond

type Client struct {
	key      string
	endpoint string
	http     *http.Client
	delay    time.Duration
}

func New(env cli.Env) *Client {
	return NewClient(env.Getenv("TYPESAFE_API_KEY"), env.Getenv("TYPESAFE_API_URL"), &http.Client{Timeout: Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, RetryDelay)
}
func NewClient(key, endpoint string, client *http.Client, delay time.Duration) *Client {
	if endpoint == "" {
		endpoint = "https://api.typesafe.ai/v1/systemone"
	}
	return &Client{key: key, endpoint: endpoint, http: client, delay: delay}
}
func (c *Client) HasKey() bool { return c.key != "" }
func (c *Client) Redact(s string) string {
	if c.key == "" {
		return s
	}
	return strings.ReplaceAll(s, c.key, "[redacted]")
}
func (c *Client) Run(bodies [][]byte) []Reply {
	replies := make([]Reply, len(bodies))
	var wg sync.WaitGroup
	work := make(chan int)
	for worker := 0; worker < min(Concurrency, len(bodies)); worker++ {
		wg.Go(func() {
			for i := range work {
				if !c.HasKey() {
					replies[i].Reason = "TYPESAFE_API_KEY is not set"
					continue
				}
				replies[i] = request(c.http, c.endpoint, c.key, bodies[i], c.delay)
				replies[i].Answer.Model = c.Redact(replies[i].Answer.Model)
			}
		})
	}
	for i := range bodies {
		work <- i
	}
	close(work)
	wg.Wait()
	return replies
}

type Answer struct {
	Model   string `json:"model"`
	Answers map[string]struct {
		Type string   `json:"type"`
		Noul *float64 `json:"noul"`
	} `json:"answers"`
}
type Reply struct {
	Answer Answer
	Reason string
}

func request(client *http.Client, endpoint, key string, body []byte, delay time.Duration) Reply {
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return Reply{Reason: "Invalid TypeSafe API URL"}
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		retry := false
		reply := Reply{}
		if err != nil {
			var timeout net.Error
			retry = errors.As(err, &timeout) && timeout.Timeout()
			reply.Reason = "TypeSafe request failed"
			if retry {
				reply.Reason = "TypeSafe request timed out"
			}
		} else {
			data, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				reply.Reason = fmt.Sprintf("TypeSafe HTTP %d", response.StatusCode)
				retry = response.StatusCode == 429 || response.StatusCode >= 500 && response.StatusCode < 600
			} else if readErr != nil {
				var timeout net.Error
				retry = errors.As(readErr, &timeout) && timeout.Timeout()
				reply.Reason = "TypeSafe response could not be read"
				if retry {
					reply.Reason = "TypeSafe request timed out"
				}
			} else if json.Unmarshal(data, &reply.Answer) != nil || strings.TrimSpace(reply.Answer.Model) == "" {
				reply.Reason = "Malformed TypeSafe answer"
			}
		}
		if !retry || attempt == 1 {
			return reply
		}
		time.Sleep(delay)
	}
	return Reply{Reason: "TypeSafe request failed"}
}
