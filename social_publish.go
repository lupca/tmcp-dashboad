package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type facebookPublishRequest struct {
	WorkspaceID string `json:"workspace_id"`
	VariantID   string `json:"variant_id"`
}

func registerSocialPublishRoutes(app *pocketbase.PocketBase, e *core.ServeEvent) {
	e.Router.POST("/api/social/facebook/posts", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewForbiddenError("Authentication required", nil)
		}

		req, err := parseFacebookPublishRequest(re)
		if err != nil {
			return apis.NewBadRequestError(err.Error(), nil)
		}

		if err := ensureWorkspaceMember(app, req.WorkspaceID, re.Auth.Id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apis.NewForbiddenError("You are not a member of this workspace", nil)
			}
			return apis.NewInternalServerError("Failed to validate workspace access", err)
		}

		variant, err := app.FindRecordById("platform_variants", req.VariantID)
		if err != nil {
			return apis.NewBadRequestError("Platform variant not found", err)
		}

		if variant.GetString("workspace_id") != req.WorkspaceID {
			return apis.NewBadRequestError("Variant does not belong to workspace", nil)
		}

		if variant.GetString("platform") != "facebook" {
			return apis.NewBadRequestError("Only facebook variants are supported in this phase", nil)
		}

		message := strings.TrimSpace(variant.GetString("adapted_copy"))
		if message == "" {
			return apis.NewBadRequestError("Variant adapted_copy is required to publish", nil)
		}

		socialAccounts, err := app.FindRecordsByFilter(
			"social_accounts",
			"workspace_id={:workspaceId} && platform={:platform}",
			"",
			2,
			0,
			dbx.Params{"workspaceId": req.WorkspaceID, "platform": "facebook"},
		)
		if err != nil {
			return apis.NewInternalServerError("Failed to load facebook social account", err)
		}

		if len(socialAccounts) == 0 {
			return apis.NewBadRequestError("No facebook social account configured for this workspace", nil)
		}
		if len(socialAccounts) > 1 {
			return apis.NewBadRequestError("Multiple facebook social accounts found. Keep exactly one facebook account configured before publishing", nil)
		}

		account := socialAccounts[0]
		pageID := strings.TrimSpace(account.GetString("account_id"))
		accessToken := strings.TrimSpace(account.GetString("access_token"))
		if pageID == "" {
			return apis.NewBadRequestError("Facebook account_id (page id) is missing", nil)
		}
		if accessToken == "" {
			return apis.NewBadRequestError("Facebook access_token is missing", nil)
		}

		fbResponse, err := publishToFacebookFeed(pageID, accessToken, message)
		if err != nil {
			return apis.NewBadRequestError("Facebook publish failed", err)
		}

		publishTime := time.Now().UTC().Format(time.RFC3339)
		variant.Set("publish_status", "published")
		variant.Set("published_at", publishTime)

		platformPostID := toStringOrEmpty(fbResponse["id"])
		if platformPostID != "" {
			variant.Set("platform_post_id", platformPostID)
		}

		metadata := map[string]any{}
		rawMetadata := variant.Get("metadata")
		switch mt := rawMetadata.(type) {
		case map[string]any:
			metadata = mt
		case string:
			if strings.TrimSpace(mt) != "" {
				_ = json.Unmarshal([]byte(mt), &metadata)
			}
		}
		metadata["facebook_publish"] = map[string]any{
			"published_at": publishTime,
			"response":     fbResponse,
		}
		variant.Set("metadata", metadata)

		if err := app.Save(variant); err != nil {
			return apis.NewInternalServerError("Published to Facebook but failed to update platform variant", err)
		}

		return re.JSON(http.StatusOK, map[string]any{
			"workspace_id":      req.WorkspaceID,
			"variant_id":        variant.Id,
			"platform":          "facebook",
			"publish_status":    variant.GetString("publish_status"),
			"platform_post_id":  platformPostID,
			"facebook_response": fbResponse,
		})
	})
}

func parseFacebookPublishRequest(re *core.RequestEvent) (*facebookPublishRequest, error) {
	info, err := re.RequestInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to parse request")
	}

	workspaceID := strings.TrimSpace(toStringOrEmpty(info.Body["workspace_id"]))
	variantID := strings.TrimSpace(toStringOrEmpty(info.Body["variant_id"]))
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if variantID == "" {
		return nil, fmt.Errorf("variant_id is required")
	}

	return &facebookPublishRequest{WorkspaceID: workspaceID, VariantID: variantID}, nil
}

func ensureWorkspaceMember(app *pocketbase.PocketBase, workspaceID string, userID string) error {
	_, err := app.FindFirstRecordByFilter(
		"workspaces",
		"id={:workspaceId} && members.id ?= {:userId}",
		dbx.Params{"workspaceId": workspaceID, "userId": userID},
	)
	return err
}

func publishToFacebookFeed(pageID, accessToken, message string) (map[string]any, error) {
	values := url.Values{}
	values.Set("message", message)
	values.Set("access_token", accessToken)

	endpoint := fmt.Sprintf("https://graph.facebook.com/v25.0/%s/feed", url.PathEscape(pageID))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create facebook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook request failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read facebook response: %w", err)
	}

	var payload map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("facebook returned invalid JSON")
		}
	} else {
		payload = map[string]any{}
	}

	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", extractFacebookErrorMessage(payload, res.StatusCode))
	}

	if toStringOrEmpty(payload["id"]) == "" {
		return nil, fmt.Errorf("facebook response missing post id")
	}

	return payload, nil
}

func extractFacebookErrorMessage(payload map[string]any, statusCode int) string {
	errVal, ok := payload["error"]
	if !ok {
		return fmt.Sprintf("facebook API returned status %d", statusCode)
	}
	errMap, ok := errVal.(map[string]any)
	if !ok {
		return fmt.Sprintf("facebook API returned status %d", statusCode)
	}

	msg := strings.TrimSpace(toStringOrEmpty(errMap["message"]))
	code := toStringOrEmpty(errMap["code"])
	subcode := toStringOrEmpty(errMap["error_subcode"])

	if msg == "" {
		return fmt.Sprintf("facebook API returned status %d", statusCode)
	}

	if code != "" || subcode != "" {
		return fmt.Sprintf("facebook error: %s (code=%s subcode=%s)", msg, code, subcode)
	}
	return fmt.Sprintf("facebook error: %s", msg)
}

func toStringOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
