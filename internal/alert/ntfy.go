package alert

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// NtfySink posts each alert to one ntfy topic URL.
type NtfySink struct {
	URL    string
	Token  string
	Client *http.Client
}

func NewNtfy(url, token string) *NtfySink {
	return &NtfySink{URL: url, Token: token, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *NtfySink) Send(ctx context.Context, a Alert) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, strings.NewReader(a.Message))
	if err != nil {
		return err
	}
	req.Header.Set("Title", a.Title)
	req.Header.Set("Tags", tagsFor(a))
	if a.Rule == "emergency" {
		req.Header.Set("Priority", "high")
	}
	if n.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.Token)
	}
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: HTTP %d", resp.StatusCode)
	}
	return nil
}

func tagsFor(a Alert) string {
	kind := "airplane"
	if a.Kind == "vessel" {
		kind = "ship"
	}
	switch a.Rule {
	case "emergency":
		return "rotating_light," + kind
	case "military":
		return "military_helmet," + kind
	default:
		return "eyes," + kind
	}
}
