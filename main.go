// main.go
package main

import (
	"log"
	"os"
	"strings"
	_ "tmcp-dashboard/migrations"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

// enable once you have at least one migration
// _ "yourpackage/migrations"

func main() {
	app := pocketbase.New()

	// loosely check if it was executed using "go run"
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Dashboard
		// (the isGoRun check is to enable it only during development)
		Automigrate: isGoRun,
	})

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Auto-create initial admin if provided via environment variables and no admins exist
		adminEmail := os.Getenv("admin@tmcp.com")
		adminPassword := os.Getenv("123qweasdzxc")

		if adminEmail != "" && adminPassword != "" {
			superusers, err := app.FindCollectionByNameOrId("_superusers")
			if err == nil {
				total, err := app.CountRecords(superusers)
				if err == nil && total == 0 {
					record := core.NewRecord(superusers)
					record.Set("email", adminEmail)
					record.Set("password", adminPassword)

					if err := app.Save(record); err != nil {
						log.Printf("Failed to create initial admin: %v", err)
					} else {
						log.Println("Created initial admin user from environment variables")
					}
				}
			}
		}

		e.Router.GET("/api/tmcp/collections", func(e *core.RequestEvent) error {
			if e.Auth == nil {
				return apis.NewForbiddenError("Authentication required", nil)
			}

			// Fetch all collections
			collections, err := app.FindAllCollections()
			if err != nil {
				return apis.NewBadRequestError("Failed to fetch collections", err)
			}

			return e.JSON(200, map[string]any{
				"items": collections,
			})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
