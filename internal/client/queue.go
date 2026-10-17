package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/SpherePrime/CLI/internal/proto"
)

func (c *Client) QueueEntries(ctx context.Context, workspaceID, sessionID string) ([]proto.QueueEntry, error) {
	return desktopGet[[]proto.QueueEntry](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/agent/sessions/"+url.PathEscape(sessionID)+"/prompts/entries")
}

func (c *Client) RunQueuedNow(ctx context.Context, workspaceID, sessionID, id string) error {
	encoded, err := json.Marshal(map[string]string{"id": id})
	if err != nil {
		return err
	}
	response, err := c.post(ctx, "/workspaces/"+url.PathEscape(workspaceID)+"/agent/sessions/"+url.PathEscape(sessionID)+"/prompts/run-now", nil, bytes.NewReader(encoded), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("run queued prompt: status %d", response.StatusCode)
	}
	return nil
}
