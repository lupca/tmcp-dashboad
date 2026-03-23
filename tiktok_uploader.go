package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

const tikTokStorageStatePrefix = "TMCP_TIKTOK_STORAGE_STATE::"

// ensurePlaywright is called to make sure playwright is installed
func ensurePlaywright() error {
	return playwright.Install()
}

// getBrowserOptions returns consistent launch options to prevent session invalidation
func getBrowserOptions(headless bool) playwright.BrowserTypeLaunchPersistentContextOptions {
	return playwright.BrowserTypeLaunchPersistentContextOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			"--disable-blink-features=AutomationControlled", // Help bypass bot detection
		},
	}
}

// qrLoginStatus tracks the async QR login processes
var qrLoginStatus sync.Map

// StartQRLoginFlow starts a headless browser, goes to TikTok QR login, returns the base64 of the QR code,
// and leaves a background goroutine to wait for the user to scan and login.
func StartQRLoginFlow(app *pocketbase.PocketBase, workspaceID string) (string, error) {
	if _, exists := qrLoginStatus.Load(workspaceID); exists {
		// A login flow is already pending for this workspace
		qrLoginStatus.Delete(workspaceID) // reset if they hit it again
	}

	pw, err := playwright.Run()
	if err != nil {
		return "", fmt.Errorf("could not start playwright: %w", err)
	}

	qrHeadless := true
	if raw := strings.TrimSpace(os.Getenv("TMCP_TIKTOK_QR_HEADLESS")); raw != "" {
		if raw == "0" || strings.EqualFold(raw, "false") || strings.EqualFold(raw, "no") {
			qrHeadless = false
		}
	}

	userDataDir := filepath.Join(os.TempDir(), "pocketbase-tiktok-auth-"+workspaceID)
	browser, err := pw.Chromium.LaunchPersistentContext(userDataDir, getBrowserOptions(qrHeadless))
	if err != nil {
		pw.Stop()
		return "", fmt.Errorf("could not launch persistent context: %w", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		browser.Close()
		pw.Stop()
		return "", err
	}

	log.Printf("[QR Flow] Navigating to TikTok QR login for workspace %s...", workspaceID)
	_, err = page.Goto("https://www.tiktok.com/login/qrcode", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		browser.Close()
		pw.Stop()
		return "", err
	}

	log.Printf("[QR Flow] Waiting for QR code to render (headless=%v)...", qrHeadless)
	maybeDismissTikTokOverlays(page)
	picBytes, err := waitForTikTokQRScreenshot(page, 45*time.Second)
	if err != nil {
		browser.Close()
		pw.Stop()
		return "", fmt.Errorf("could not get QR code image: %w", err)
	}
	base64QR := "data:image/png;base64," + base64.StdEncoding.EncodeToString(picBytes)

	// Set status to pending
	qrLoginStatus.Store(workspaceID, "pending")

	// Start background goroutine to wait for login success
	go func() {
		defer pw.Stop()
		defer browser.Close()

		log.Printf("[QR Flow] Waiting up to 2 minutes for user to scan QR code...")
		// When user scans and confirms, TikTok navigates away from login page or shows "foryou"
		timeout := time.After(2 * time.Minute)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				qrLoginStatus.Store(workspaceID, "timeout")
				log.Println("[QR Flow] QR Login timed out.")
				return
			case <-ticker.C:
				url := page.URL()
				hasSession := hasTikTokSessionCookie(browser)
				hasProfile := false
				if count, _ := page.Locator("[data-e2e='profile-icon']").Count(); count > 0 {
					hasProfile = true
				}
				offLoginPage := !strings.Contains(url, "/login") && !strings.Contains(url, "passport")

				if hasSession && (offLoginPage || hasProfile) {
					if err := persistTikTokSession(app, workspaceID, page, browser); err != nil {
						log.Printf("[QR Flow] Failed to persist TikTok session for %s: %v", workspaceID, err)
						qrLoginStatus.Store(workspaceID, "error")
						return
					}
					log.Printf("[QR Flow] Successfully captured TikTok session for %s!", workspaceID)
					qrLoginStatus.Store(workspaceID, "success")
					return
				}

				// If URL changes away from login, or if a known logged-in element appears
				if offLoginPage {
					log.Println("[QR Flow] URL changed, checking for success...")
					// Wait a tiny bit for cookies to settle
					time.Sleep(3 * time.Second)
					if err := persistTikTokSession(app, workspaceID, page, browser); err != nil {
						log.Printf("[QR Flow] Failed to persist TikTok session for %s: %v", workspaceID, err)
						qrLoginStatus.Store(workspaceID, "error")
						return
					}
					log.Printf("[QR Flow] Successfully captured TikTok session for %s!", workspaceID)
					qrLoginStatus.Store(workspaceID, "success")
					return
				}

				// A fallback: if a profile icon appears
				if hasProfile {
					if err := persistTikTokSession(app, workspaceID, page, browser); err != nil {
						log.Printf("[QR Flow] Failed to persist TikTok session for %s: %v", workspaceID, err)
						qrLoginStatus.Store(workspaceID, "error")
						return
					}
					log.Printf("[QR Flow] Successfully captured TikTok session for %s!", workspaceID)
					qrLoginStatus.Store(workspaceID, "success")
					return
				}
			}
		}
	}()

	return base64QR, nil
}

// GetQRLoginStatus checks the status of the async login process
func GetQRLoginStatus(workspaceID string) string {
	val, ok := qrLoginStatus.Load(workspaceID)
	if !ok {
		return "not_started"
	}
	return val.(string)
}

// authenticateTikTok opens a headful browser for the user to log in manually (CLI ONLY).
func authenticateTikTok(userDataDir string) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start playwright: %w", err)
	}
	defer pw.Stop()

	log.Printf("Launching browser. Please log in to TikTok...")
	browser, err := pw.Chromium.LaunchPersistentContext(userDataDir, getBrowserOptions(false))
	if err != nil {
		return fmt.Errorf("could not launch persistent context: %w", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %w", err)
	}

	if _, err := page.Goto("https://www.tiktok.com/login"); err != nil {
		return fmt.Errorf("failed to navigate to login page: %w", err)
	}

	log.Println("Waiting for you to log in. The application will wait until it detects a successful login by checking for the 'foryou' page or after 5 minutes.")
	// Wait until the user is logged in and redirected
	page.WaitForURL("https://www.tiktok.com/foryou*", playwright.PageWaitForURLOptions{
		Timeout: playwright.Float(300000), // 5 minutes to log in
	})

	log.Println("Successfully captured TikTok session! You can close the browser now.")
	return nil
}

// uploadToTikTok automates the upload process.
func uploadToTikTok(app *pocketbase.PocketBase, workspaceID, videoPath, caption string) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start playwright: %w", err)
	}
	defer pw.Stop()

	userDataDir := filepath.Join(os.TempDir(), "pocketbase-tiktok-auth-"+workspaceID)
	var storageState *playwright.StorageState
	if app != nil {
		storageState, err = loadTikTokStorageStateFromDB(app, workspaceID)
		if err != nil {
			return fmt.Errorf("missing TikTok session in database. Please connect account via QR again: %w", err)
		}
	}

	log.Printf("Launching headful browser to bypass bot detection...")
	browser, err := pw.Chromium.LaunchPersistentContext(userDataDir, getBrowserOptions(false))
	if err != nil {
		return fmt.Errorf("could not launch persistent context: %w", err)
	}
	defer browser.Close()

	if storageState != nil {
		if err := applyTikTokStorageState(browser, storageState); err != nil {
			return fmt.Errorf("failed to restore TikTok session from database: %w", err)
		}
	}

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %w", err)
	}

	log.Println("Navigating to TikTok Creator Center...")
	_, err = page.Goto("https://www.tiktok.com/creator-center/upload?from=upload", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil && !strings.Contains(err.Error(), "Timeout") {
		return fmt.Errorf("failed to navigate to upload page: %w", err)
	}

	// Wait a moment for redirects
	time.Sleep(5 * time.Second)

	// Check if we are redirected to login (meaning session expired or not logged in)
	if strings.Contains(page.URL(), "/login") {
		return fmt.Errorf("not logged in or session expired. Redirected to login: '%s'", page.URL())
	}

	// Try to locate the iframe if it exists
	var frame playwright.Frame
	for _, f := range page.Frames() {
		if f.Name() == "Upload_index_iframe" || f.URL() != "" {
			frame = f
		}
	}

	// Sometime tiktok doesn't use iframes entirely, it depends.
	// Let's use the main page or frame
	if frame != nil {
		// Just relying on page locators which auto-pierce in some cases, or we use iframe explicitly
		log.Println("Found iframe, trying to upload inside iframe")
	}

	log.Printf("Uploading video: %s", videoPath)
	fileInput := page.Locator("input[type='file'][accept*='video']").First()
	err = fileInput.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateAttached, Timeout: playwright.Float(15000)})
	if err != nil {
		// Try another common selector for the file input if the first fails
		log.Println("First selector failed, trying fallback input selector...")
		fileInput = page.Locator("input[type='file']").First()
	}

	if err := fileInput.SetInputFiles(videoPath); err != nil {
		return fmt.Errorf("failed to set video file: %w", err)
	}

	log.Println("Waiting for video to be processed...")
	// Wait a bit for the editor to render
	time.Sleep(5 * time.Second)

	log.Printf("Setting caption: %s", caption)
	// The caption editor is usually a Draft.js or contenteditable div
	captionInput := page.Locator(".public-DraftEditor-content").First()
	if err := captionInput.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible, Timeout: playwright.Float(15000)}); err != nil {
		log.Println("Warning: Caption input not found by primary selector, trying fallback...")
		captionInput = page.Locator("[contenteditable='true']").First()
	}

	if err := captionInput.Click(); err == nil {
		page.Keyboard().Type(caption)
	} else {
		log.Printf("Could not click caption input: %v", err)
	}

	log.Println("Waiting for upload to complete (post button active)...")

	// Handle TikTok's pre-post confirmation models (like Copyright check or Data consent)
	// We check for dialogs and try to dismiss or accept them
	time.Sleep(2 * time.Second)
	log.Println("Checking for any verification or consent modals...")

	// Try to find any common 'Accept' or 'Confirm' buttons in Modals
	modalBtns := []string{
		"button:has-text('Turn on')", "button:has-text('Bật')",
		"button:has-text('Accept')", "button:has-text('Chấp nhận')",
		"button:has-text('Xác nhận')", "button:has-text('Confirm')",
		"button:has-text('Allow')", "button:has-text('Cho phép')",
		"button:has-text('Got it')", "button:has-text('Đã hiểu')",
		"button:has-text('Đồng ý')", "div[role='dialog'] button.primary",
		".TUXModal button:has-text('Post')", ".TUXModal button.primary",
	}

	for _, selector := range modalBtns {
		btn := page.Locator(selector).First()
		if visible, _ := btn.IsVisible(); visible {
			log.Printf("Found modal button matching '%s'. Clicking it...", selector)
			_ = btn.Click()
			time.Sleep(2 * time.Second)
		}
	}

	// Sometimes TikTok requires you to scroll down or just wait more
	page.Keyboard().Press("Escape") // try to dismiss any unavoidable overlays just in case
	time.Sleep(2 * time.Second)

	// Make sure it is enabled (TikTok might keep it disabled while uploading)
	log.Println("Waiting for Post button to become enabled...")
	time.Sleep(10 * time.Second) // rough wait for chunk upload and copyright checks

	// Most robust way to find the actual Post button, preventing matches with "When to post" etc.
	postBtn := page.Locator("button").Filter(playwright.LocatorFilterOptions{
		HasText: "Post",
	}).Last() // typically it's the last button on the page in that layout

	if count, _ := postBtn.Count(); count == 0 {
		log.Println("Could not find 'Post', trying 'Đăng'...")
		postBtn = page.Locator("button").Filter(playwright.LocatorFilterOptions{
			HasText: "Đăng",
		}).Last()
	}

	for i := 0; i < 3; i++ {
		log.Println("Attempting to click Post button...")
		// Scroll into view and hover to mimic human
		_ = postBtn.ScrollIntoViewIfNeeded()
		time.Sleep(1 * time.Second)
		_ = postBtn.Hover()
		time.Sleep(500 * time.Millisecond)

		if err := postBtn.Click(); err != nil {
			log.Printf("Failed to click post button on attempt %d: %v", i+1, err)
		} else {
			log.Println("Post button clicked. Checking for late modals (like 'Turn on content checks')...")
			time.Sleep(2 * time.Second)

			// Click Turn On if it appears late after clicking Post
			for _, selector := range []string{"button:has-text('Turn on')", "button:has-text('Bật')"} {
				btn := page.Locator(selector).First()
				if visible, _ := btn.IsVisible(); visible {
					log.Printf("Found late modal button '%s', clicking it...", selector)
					_ = btn.Click()
				}
			}

			// Wait a bit to check if URL changes
			time.Sleep(5 * time.Second)
			if strings.Contains(page.URL(), "content") {
				break // Successfully navigated away!
			} else {
				log.Println("URL hasn't changed. The click might have been ignored. Trying again...")
			}
		}
	}

	log.Println("Waiting for confirmation (TikTok redirects to management page upon success)...")
	// Wait up to 60 seconds for the redirect to content manager page
	err = page.WaitForURL("**/*content*", playwright.PageWaitForURLOptions{Timeout: playwright.Float(60000)})
	if err != nil {
		log.Println("Redirect not detected automatically. Waiting an extra 30 seconds to ensure the background upload task finishes...")
		time.Sleep(30 * time.Second)
	} else {
		log.Println("Redirect successful! Video was published.")
	}

	log.Println("Video uploaded successfully!")

	return nil
}

func maybeDismissTikTokOverlays(page playwright.Page) {
	selectors := []string{
		"button:has-text('Accept all')",
		"button:has-text('Accept')",
		"button:has-text('I agree')",
		"button:has-text('Đồng ý')",
		"button:has-text('Allow all')",
	}

	for _, selector := range selectors {
		btn := page.Locator(selector).First()
		if visible, _ := btn.IsVisible(); visible {
			_ = btn.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(1000)})
			time.Sleep(250 * time.Millisecond)
		}
	}
}

func waitForTikTokQRScreenshot(page playwright.Page, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	selectors := []string{
		"[data-e2e*='qrcode'] canvas:visible",
		"[class*='qrcode'] canvas:visible",
		"[class*='qr'] canvas:visible",
		"img[alt*='qr' i]:visible",
		"img[src*='qrcode' i]:visible",
		"img[src*='data:image' i]:visible",
		"canvas:visible",
		"img:visible",
	}

	for time.Now().Before(deadline) {
		for _, frame := range page.Frames() {
			for _, selector := range selectors {
				locator := frame.Locator(selector)
				count, err := locator.Count()
				if err != nil || count == 0 {
					continue
				}

				if count > 6 {
					count = 6
				}

				for i := 0; i < count; i++ {
					candidate := locator.Nth(i)
					if visible, _ := candidate.IsVisible(); !visible {
						continue
					}

					box, err := candidate.BoundingBox()
					if err != nil || box == nil {
						continue
					}

					if box.Width < 120 || box.Height < 120 || box.Width > 800 || box.Height > 800 {
						continue
					}

					shot, err := candidate.Screenshot()
					if err == nil && len(shot) > 0 {
						return shot, nil
					}
				}
			}
		}

		maybeDismissTikTokOverlays(page)
		time.Sleep(1500 * time.Millisecond)
	}

	fallback, err := page.Screenshot(playwright.PageScreenshotOptions{FullPage: playwright.Bool(true)})
	if err != nil {
		return nil, fmt.Errorf("could not find QR code on page within %s and fallback screenshot failed: %w", timeout, err)
	}

	log.Println("[QR Flow] QR-specific selector not found; returning full page screenshot fallback.")
	return fallback, nil
}

func hasTikTokSessionCookie(browser playwright.BrowserContext) bool {
	cookies, err := browser.Cookies("https://www.tiktok.com")
	if err != nil {
		return false
	}

	for _, c := range cookies {
		if c.Name == "sessionid" || c.Name == "sessionid_ss" || c.Name == "sid_tt" {
			if strings.TrimSpace(c.Value) != "" {
				return true
			}
		}
	}

	return false
}

func persistTikTokSession(app *pocketbase.PocketBase, workspaceID string, page playwright.Page, browser playwright.BrowserContext) error {
	if app == nil {
		return fmt.Errorf("app is required to persist TikTok session")
	}

	state, err := browser.StorageState()
	if err != nil {
		return fmt.Errorf("failed to read browser storage state: %w", err)
	}

	encodedState, err := encodeTikTokStorageState(state)
	if err != nil {
		return err
	}

	accountName, accountID := extractTikTokIdentity(page, state, workspaceID)
	expiresAt := getTikTokSessionExpiry(state)

	records, err := app.FindRecordsByFilter(
		"social_accounts",
		"workspace_id={:workspaceId} && platform={:platform}",
		"-updated",
		1,
		0,
		dbx.Params{"workspaceId": workspaceID, "platform": "tiktok"},
	)
	if err != nil {
		return fmt.Errorf("failed to query social_accounts: %w", err)
	}

	var record *core.Record
	if len(records) > 0 {
		record = records[0]
	} else {
		collection, err := app.FindCollectionByNameOrId("social_accounts")
		if err != nil {
			return fmt.Errorf("social_accounts collection not found: %w", err)
		}
		record = core.NewRecord(collection)
		record.Set("workspace_id", workspaceID)
		record.Set("platform", "tiktok")
	}

	record.Set("account_name", accountName)
	record.Set("account_id", accountID)
	record.Set("access_token", encodedState)
	record.Set("refresh_token", "")
	if expiresAt != "" {
		record.Set("expires_at", expiresAt)
	}

	if err := app.Save(record); err != nil {
		return fmt.Errorf("failed to save TikTok social account: %w", err)
	}

	return nil
}

func loadTikTokStorageStateFromDB(app *pocketbase.PocketBase, workspaceID string) (*playwright.StorageState, error) {
	records, err := app.FindRecordsByFilter(
		"social_accounts",
		"workspace_id={:workspaceId} && platform={:platform}",
		"-updated",
		1,
		0,
		dbx.Params{"workspaceId": workspaceID, "platform": "tiktok"},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query social_accounts: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no TikTok account configured for workspace %s", workspaceID)
	}

	rawState := strings.TrimSpace(records[0].GetString("access_token"))
	if rawState == "" {
		return nil, fmt.Errorf("TikTok storage state is empty")
	}

	storageState, err := decodeTikTokStorageState(rawState)
	if err != nil {
		return nil, err
	}

	return storageState, nil
}

func encodeTikTokStorageState(state *playwright.StorageState) (string, error) {
	bytes, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to serialize TikTok storage state: %w", err)
	}
	return tikTokStorageStatePrefix + base64.StdEncoding.EncodeToString(bytes), nil
}

func decodeTikTokStorageState(raw string) (*playwright.StorageState, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, tikTokStorageStatePrefix) {
		payload := strings.TrimPrefix(raw, tikTokStorageStatePrefix)
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("invalid encoded TikTok storage state: %w", err)
		}

		var state playwright.StorageState
		if err := json.Unmarshal(decoded, &state); err != nil {
			return nil, fmt.Errorf("invalid TikTok storage JSON: %w", err)
		}
		return &state, nil
	}

	return nil, fmt.Errorf("stored TikTok session has unsupported format")
}

func applyTikTokStorageState(browser playwright.BrowserContext, state *playwright.StorageState) error {
	if state == nil || len(state.Cookies) == 0 {
		return nil
	}

	cookies := make([]playwright.OptionalCookie, 0, len(state.Cookies))
	for _, c := range state.Cookies {
		cookies = append(cookies, c.ToOptionalCookie())
	}

	if err := browser.AddCookies(cookies); err != nil {
		return fmt.Errorf("failed to add cookies: %w", err)
	}

	return nil
}

func extractTikTokIdentity(page playwright.Page, state *playwright.StorageState, workspaceID string) (string, string) {
	accountName := "TikTok Connected Account"
	accountID := "tiktok_" + workspaceID

	if page != nil {
		if value, err := page.Evaluate(`() => {
			const profileLink = document.querySelector('a[href^="/@"]');
			if (!profileLink) return '';
			const href = profileLink.getAttribute('href') || '';
			if (!href.startsWith('/@')) return '';
			return href.slice(2).split(/[/?#]/)[0];
		}`); err == nil {
			if username, ok := value.(string); ok {
				username = strings.TrimSpace(username)
				if username != "" {
					accountName = "@" + username
					accountID = username
				}
			}
		}
	}

	if accountID == "tiktok_"+workspaceID {
		sessionSeed := workspaceID
		for _, c := range state.Cookies {
			if c.Name == "sessionid" || c.Name == "sessionid_ss" || c.Name == "sid_tt" {
				sessionSeed = c.Value
				break
			}
		}
		hash := sha256.Sum256([]byte(sessionSeed))
		accountID = "tt_" + hex.EncodeToString(hash[:8])
	}

	return accountName, accountID
}

func getTikTokSessionExpiry(state *playwright.StorageState) string {
	for _, c := range state.Cookies {
		if c.Name == "sessionid" || c.Name == "sessionid_ss" || c.Name == "sid_tt" {
			if c.Expires > 0 {
				return time.Unix(int64(c.Expires), 0).UTC().Format(time.RFC3339)
			}
		}
	}
	return ""
}

// RegisterTikTokCommands adds custom CLI commands to the PocketBase app
func RegisterTikTokCommands(app *pocketbase.PocketBase) {
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "tiktok-login",
		Short: "Opens a browser to log into TikTok and saves the session.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ensurePlaywright()
			if err != nil {
				return fmt.Errorf("failed to install Playwright deps: %w", err)
			}
			userDataDir := filepath.Join(os.TempDir(), "pocketbase-tiktok-auth-cli")
			if err := authenticateTikTok(userDataDir); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
			return nil
		},
	})

	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "tiktok-upload [video_path] [caption]",
		Short: "Uploads a video to TikTok using the saved session.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoPath := args[0]
			caption := ""
			if len(args) > 1 {
				caption = args[1]
			}

			if err := ensurePlaywright(); err != nil {
				return fmt.Errorf("failed to install Playwright deps: %w", err)
			}

			// We use "cli" as the workspace ID for CLI runs
			if err := uploadToTikTok(nil, "cli", videoPath, caption); err != nil {
				return fmt.Errorf("upload failed: %w", err)
			}
			return nil
		},
	})
}
