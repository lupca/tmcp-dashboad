package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerVideoRoutes(app *pocketbase.PocketBase, e *core.ServeEvent) {
	// GET /api/videos - List all videos
	e.Router.GET("/api/videos", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewUnauthorizedError("Authentication required", nil)
		}

		workspaceID := re.Request.URL.Query().Get("workspace_id")
		filter := "file_type='video'"
		if workspaceID != "" {
			filter += fmt.Sprintf(" && workspace_id='%s'", strings.ReplaceAll(workspaceID, "'", ""))
		}

		records, err := app.FindRecordsByFilter("media_assets", filter, "-created", 100, 0)
		if err != nil {
			return apis.NewBadRequestError("Failed to fetch videos", err)
		}

		// Return models as JSON
		return re.JSON(http.StatusOK, map[string]any{
			"items": records,
		})
	})

	// POST /api/videos - Create a new video
	e.Router.POST("/api/videos", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewUnauthorizedError("Authentication required", nil)
		}

		collection, err := app.FindCollectionByNameOrId("media_assets")
		if err != nil {
			return apis.NewBadRequestError("Collection media_assets not found", err)
		}

		record := core.NewRecord(collection)
		var data map[string]any
		if err := re.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Failed to parse body", err)
		}

		for k, v := range data {
			record.Set(k, v)
		}
		record.Set("file_type", "video")

		if err := app.Save(record); err != nil {
			return apis.NewBadRequestError("Failed to create video", err)
		}

		return re.JSON(http.StatusOK, record)
	})

	// PUT /api/videos/:id - Update an existing video
	e.Router.PUT("/api/videos/{id}", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewUnauthorizedError("Authentication required", nil)
		}

		id := re.Request.PathValue("id")
		record, err := app.FindRecordById("media_assets", id)
		if err != nil {
			return apis.NewNotFoundError("Video not found", err)
		}

		var data map[string]any
		if err := re.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Failed to parse body", err)
		}

		for k, v := range data {
			record.Set(k, v)
		}

		if err := app.Save(record); err != nil {
			return apis.NewBadRequestError("Failed to update video", err)
		}

		return re.JSON(http.StatusOK, record)
	})

	// DELETE /api/videos/:id - Delete a video
	e.Router.DELETE("/api/videos/{id}", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewUnauthorizedError("Authentication required", nil)
		}

		id := re.Request.PathValue("id")
		record, err := app.FindRecordById("media_assets", id)
		if err != nil {
			return apis.NewNotFoundError("Video not found", err)
		}

		if err := app.Delete(record); err != nil {
			return apis.NewBadRequestError("Failed to delete video", err)
		}

		return re.NoContent(http.StatusNoContent)
	})

	// POST /api/videos/:id/publish-tiktok - Publish to TikTok
	e.Router.POST("/api/videos/{id}/publish-tiktok", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewUnauthorizedError("Authentication required", nil)
		}

		id := re.Request.PathValue("id")
		record, err := app.FindRecordById("media_assets", id)
		if err != nil {
			return apis.NewNotFoundError("Video not found", err)
		}

		if record.GetString("file_type") != "video" {
			return apis.NewBadRequestError("Asset is not a video", nil)
		}

		// Look for 'caption' in JSON body
		var payload struct {
			Caption string `json:"caption"`
		}
		_ = re.BindBody(&payload)

		fileName := record.GetString("file")
		if fileName == "" {
			return apis.NewBadRequestError("Video has no file uploaded", nil)
		}

		workspaceID := record.GetString("workspace_id")
		if workspaceID == "" {
			return apis.NewBadRequestError("Video is missing workspace_id", nil)
		}

		if err := ensureWorkspaceMember(app, workspaceID, re.Auth.Id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apis.NewForbiddenError("You are not a member of this workspace", nil)
			}
			return apis.NewInternalServerError("Failed to validate workspace access", err)
		}

		// Resolve absolute file path from PocketBase pb_data
		// Typically files are in pb_data/storage/media_assets/{id}/{filename}
		baseDir := app.DataDir()
		videoPath := filepath.Join(baseDir, "storage", record.Collection().Id, record.Id, fileName)

		// Run in a goroutine if we don't want to block, but for simplicity we block to return success/fail
		if err := uploadToTikTok(app, workspaceID, videoPath, payload.Caption); err != nil {
			return apis.NewBadRequestError("TikTok upload failed. Note: You may need to scan the QR code first.", err)
		}

		// Update record to indicate it was published (using tags or simple log)
		// (Assume it's an array of strings or simple text, we won't strictly enforce schema here)
		record.Set("tags", append(record.GetStringSlice("tags"), "tiktok-published"))
		_ = app.Save(record)

		return re.JSON(http.StatusOK, map[string]any{
			"status":  "success",
			"message": "Uploaded to TikTok successfully!",
		})
	})

	// GET /api/tiktok/login-qr - Get QR code for TikTok login
	e.Router.GET("/api/tiktok/login-qr", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewUnauthorizedError("Authentication required", nil)
		}
		workspaceID := re.Request.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			return apis.NewBadRequestError("workspace_id is required", nil)
		}
		if err := ensureWorkspaceMember(app, workspaceID, re.Auth.Id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apis.NewForbiddenError("You are not a member of this workspace", nil)
			}
			return apis.NewInternalServerError("Failed to validate workspace access", err)
		}

		base64QR, err := StartQRLoginFlow(app, workspaceID)
		if err != nil {
			return apis.NewBadRequestError("Failed to start QR login flow", err)
		}

		return re.JSON(http.StatusOK, map[string]any{
			"qr_code": base64QR,
		})
	})

	// GET /api/tiktok/login-status - Check QR login status
	e.Router.GET("/api/tiktok/login-status", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return apis.NewUnauthorizedError("Authentication required", nil)
		}
		workspaceID := re.Request.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			return apis.NewBadRequestError("workspace_id is required", nil)
		}
		if err := ensureWorkspaceMember(app, workspaceID, re.Auth.Id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apis.NewForbiddenError("You are not a member of this workspace", nil)
			}
			return apis.NewInternalServerError("Failed to validate workspace access", err)
		}

		status := GetQRLoginStatus(workspaceID)
		return re.JSON(http.StatusOK, map[string]any{
			"status": status,
		})
	})
}
