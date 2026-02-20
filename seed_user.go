package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	app := pocketbase.New()

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// 1. Create Admin
		superusers, err := app.FindCollectionByNameOrId("_superusers")
		if err != nil {
			return err
		}

		adminEmail := "admin@tmcp.com"
		adminPass := "1234567890"

		existingAdmin, _ := app.FindAuthRecordByEmail("_superusers", adminEmail)
		if existingAdmin == nil {
			record := core.NewRecord(superusers)
			record.Set("email", adminEmail)
			record.Set("password", adminPass)
			if err := app.Save(record); err != nil {
				log.Printf("Failed to create admin: %v", err)
			} else {
				log.Println("Created admin user: " + adminEmail)
			}
		} else {
			log.Println("Admin already exists")
		}

		// 2. Create App User
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		userEmail := "user@tmcp.com"
		userPass := "1234567890"

		existingUser, _ := app.FindAuthRecordByEmail("users", userEmail)
		if existingUser == nil {
			record := core.NewRecord(users)
			record.Set("email", userEmail)
			record.Set("password", userPass)
			record.Set("passwordConfirm", userPass)
			record.Set("name", "Test User")
			// Set any other required fields for your schema
			record.Set("role", "admin") // Assuming role field exists from previous steps

			if err := app.Save(record); err != nil {
				log.Printf("Failed to create user: %v", err)
			} else {
				log.Println("Created app user: " + userEmail)
			}
			existingUser, _ = app.FindAuthRecordByEmail("users", userEmail)
		} else {
			log.Println("User already exists")
		}

		// 3. Create Workspace
		workspaces, err := app.FindCollectionByNameOrId("workspaces")
		if err != nil {
			return err
		}

		// Check if user has workspaces
		// We can't easily check relation here without advanced query, so just try to create one if "Demo Workspace" doesn't exist
		// Actually, just create one with a unique name to be safe? Or check by name.

		workspaceName := "Demo Workspace"
		// Check if exists
		// records, _ := app.FindRecordsByFilter("workspaces", "name = {:name}", "-created", 1, 0, map[string]any{"name": workspaceName})
		// Simple check:
		// But Name is unique in schema.

		existingWS, _ := app.FindFirstRecordByFilter("workspaces", "name='"+workspaceName+"'")
		if existingWS == nil {
			ws := core.NewRecord(workspaces)
			ws.Set("name", workspaceName)
			ws.Set("owner_id", existingUser.Id)
			ws.Set("members", []string{existingUser.Id})

			if err := app.Save(ws); err != nil {
				log.Printf("Failed to create workspace: %v", err)
			} else {
				log.Println("Created workspace: " + workspaceName)
			}
		} else {
			log.Println("Workspace already exists")
		}

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
