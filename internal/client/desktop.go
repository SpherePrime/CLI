package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SpherePrime/CLI/internal/proto"
	"net/http"
	"net/url"
)

func desktopGet[T any](ctx context.Context, client *Client, path string) (T, error) {
	var result T
	response, err := client.get(ctx, path, nil, nil)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("desktop API %s: status %d", path, response.StatusCode)
	}
	err = json.NewDecoder(response.Body).Decode(&result)
	return result, err
}

func (c *Client) Capabilities(ctx context.Context) (proto.DesktopCapabilities, error) {
	return desktopGet[proto.DesktopCapabilities](ctx, c, "/capabilities")
}

func (c *Client) GetRun(ctx context.Context, workspaceID, runID string) (proto.RunRecord, error) {
	return desktopGet[proto.RunRecord](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/runs/"+url.PathEscape(runID))
}

func (c *Client) ListRuns(ctx context.Context, workspaceID string) ([]proto.RunRecord, error) {
	return desktopGet[[]proto.RunRecord](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/runs")
}

func (c *Client) PendingRequests(ctx context.Context, workspaceID string) (proto.PendingRequests, error) {
	return desktopGet[proto.PendingRequests](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/pending")
}

func (c *Client) ModelCapabilities(ctx context.Context, workspaceID string) ([]proto.ModelCapabilities, error) {
	return desktopGet[[]proto.ModelCapabilities](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/models/capabilities")
}

func (c *Client) Plugins(ctx context.Context, workspaceID string) ([]proto.PluginInfo, error) {
	return desktopGet[[]proto.PluginInfo](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/plugins")
}

func (c *Client) PluginTasks(ctx context.Context, workspaceID string) ([]proto.PluginTask, error) {
	return desktopGet[[]proto.PluginTask](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/plugins/tasks")
}

func (c *Client) BackgroundJobs(ctx context.Context, workspaceID string) ([]proto.BackgroundJob, error) {
	return desktopGet[[]proto.BackgroundJob](ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/jobs")
}
