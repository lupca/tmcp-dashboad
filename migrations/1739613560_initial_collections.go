// migrations/1739613560_initial_collections.go
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// --- 1. Worksheets ---
		usersCollection, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		worksheets := core.NewBaseCollection("worksheets")
		worksheets.ListRule = types.Pointer("@request.auth.id != ''")
		worksheets.ViewRule = types.Pointer("@request.auth.id != ''")
		worksheets.CreateRule = types.Pointer("@request.auth.id != ''")
		worksheets.UpdateRule = types.Pointer("@request.auth.id != '' && ownerId = @request.auth.id")
		worksheets.DeleteRule = types.Pointer("@request.auth.id != '' && ownerId = @request.auth.id")
		worksheets.Fields.Add(
			&core.RelationField{
				Name:          "ownerId",
				Required:      true,
				CollectionId:  usersCollection.Id,
				CascadeDelete: true,
				MaxSelect:     1,
			},
			&core.RelationField{
				Name:         "members",
				CollectionId: usersCollection.Id,
				MaxSelect:    0, // unlimited
			},
			&core.TextField{
				Name:     "title",
				Required: true,
				Max:      500,
			},
			&core.JSONField{
				Name: "content",
			},
			&core.AutodateField{
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)
		if err := app.Save(worksheets); err != nil {
			return err
		}

		// --- 2. Brand Identities ---
		brandIdentities := core.NewBaseCollection("brand_identities")
		brandIdentities.ListRule = types.Pointer("@request.auth.id != ''")
		brandIdentities.ViewRule = types.Pointer("@request.auth.id != ''")
		brandIdentities.CreateRule = types.Pointer("@request.auth.id != ''")
		brandIdentities.UpdateRule = types.Pointer("@request.auth.id != ''")
		brandIdentities.DeleteRule = types.Pointer("@request.auth.id != ''")
		brandIdentities.Fields.Add(
			&core.RelationField{
				Name:          "worksheetId",
				Required:      true,
				CollectionId:  worksheets.Id,
				CascadeDelete: true,
				MaxSelect:     1,
			},
			&core.TextField{
				Name:     "brandName",
				Required: true,
				Max:      300,
			},
			&core.URLField{
				Name: "logoUrl",
			},
			&core.JSONField{
				Name: "colorPalette",
			},
			&core.AutodateField{
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)
		if err := app.Save(brandIdentities); err != nil {
			return err
		}

		// --- 3. Ideal Customer Profiles ---
		idealCustomerProfiles := core.NewBaseCollection("ideal_customer_profiles")
		idealCustomerProfiles.ListRule = types.Pointer("@request.auth.id != ''")
		idealCustomerProfiles.ViewRule = types.Pointer("@request.auth.id != ''")
		idealCustomerProfiles.CreateRule = types.Pointer("@request.auth.id != ''")
		idealCustomerProfiles.UpdateRule = types.Pointer("@request.auth.id != ''")
		idealCustomerProfiles.DeleteRule = types.Pointer("@request.auth.id != ''")
		idealCustomerProfiles.Fields.Add(
			&core.RelationField{
				Name:          "worksheetId",
				Required:      true,
				CollectionId:  worksheets.Id,
				CascadeDelete: true,
				MaxSelect:     1,
			},
			&core.TextField{
				Name:     "personaName",
				Required: true,
				Max:      300,
			},
			&core.JSONField{
				Name: "demographics",
			},
			&core.JSONField{
				Name: "painPoints",
			},
			&core.AutodateField{
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)
		if err := app.Save(idealCustomerProfiles); err != nil {
			return err
		}

		// --- 4. Marketing Campaigns ---
		marketingCampaigns := core.NewBaseCollection("marketing_campaigns")
		marketingCampaigns.ListRule = types.Pointer("@request.auth.id != ''")
		marketingCampaigns.ViewRule = types.Pointer("@request.auth.id != ''")
		marketingCampaigns.CreateRule = types.Pointer("@request.auth.id != ''")
		marketingCampaigns.UpdateRule = types.Pointer("@request.auth.id != ''")
		marketingCampaigns.DeleteRule = types.Pointer("@request.auth.id != ''")
		marketingCampaigns.Fields.Add(
			&core.RelationField{
				Name:          "worksheetId",
				Required:      true,
				CollectionId:  worksheets.Id,
				CascadeDelete: true,
				MaxSelect:     1,
			},
			&core.TextField{
				Name:     "name",
				Required: true,
				Max:      500,
			},
			&core.TextField{
				Name: "goal",
				Max:  2000,
			},
			&core.TextField{
				Name: "acquisitionStrategy",
				Max:  5000,
			},
			&core.AutodateField{
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)
		if err := app.Save(marketingCampaigns); err != nil {
			return err
		}

		// --- 5. Campaign Tasks ---
		campaignTasks := core.NewBaseCollection("campaign_tasks")
		campaignTasks.ListRule = types.Pointer("@request.auth.id != ''")
		campaignTasks.ViewRule = types.Pointer("@request.auth.id != ''")
		campaignTasks.CreateRule = types.Pointer("@request.auth.id != ''")
		campaignTasks.UpdateRule = types.Pointer("@request.auth.id != ''")
		campaignTasks.DeleteRule = types.Pointer("@request.auth.id != ''")
		campaignTasks.Fields.Add(
			&core.RelationField{
				Name:          "campaignId",
				Required:      true,
				CollectionId:  marketingCampaigns.Id,
				CascadeDelete: true,
				MaxSelect:     1,
			},
			&core.TextField{
				Name:     "taskName",
				Required: true,
				Max:      500,
			},
			&core.NumberField{
				Name: "week",
			},
			&core.SelectField{
				Name:   "status",
				Values: []string{"To Do", "In Progress", "Done", "Cancelled"},
			},
			&core.AutodateField{
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)
		if err := app.Save(campaignTasks); err != nil {
			return err
		}

		// --- 6. Content Calendar Events ---
		contentCalendarEvents := core.NewBaseCollection("content_calendar_events")
		contentCalendarEvents.ListRule = types.Pointer("@request.auth.id != ''")
		contentCalendarEvents.ViewRule = types.Pointer("@request.auth.id != ''")
		contentCalendarEvents.CreateRule = types.Pointer("@request.auth.id != ''")
		contentCalendarEvents.UpdateRule = types.Pointer("@request.auth.id != ''")
		contentCalendarEvents.DeleteRule = types.Pointer("@request.auth.id != ''")
		contentCalendarEvents.Fields.Add(
			&core.RelationField{
				Name:          "campaignId",
				Required:      true,
				CollectionId:  marketingCampaigns.Id,
				CascadeDelete: true,
				MaxSelect:     1,
			},
			&core.TextField{
				Name:     "title",
				Required: true,
				Max:      500,
			},
			&core.DateField{
				Name: "eventDate",
			},
			&core.SelectField{
				Name:   "eventType",
				Values: []string{"Blog Post", "Social Media", "Email", "Webinar", "Event", "Ad Campaign", "Other"},
			},
			&core.TextField{
				Name: "aiAnalysis",
				Max:  10000,
			},
			&core.AutodateField{
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)
		if err := app.Save(contentCalendarEvents); err != nil {
			return err
		}

		// --- 7. Social Posts ---
		socialPosts := core.NewBaseCollection("social_posts")
		socialPosts.ListRule = types.Pointer("@request.auth.id != ''")
		socialPosts.ViewRule = types.Pointer("@request.auth.id != ''")
		socialPosts.CreateRule = types.Pointer("@request.auth.id != ''")
		socialPosts.UpdateRule = types.Pointer("@request.auth.id != ''")
		socialPosts.DeleteRule = types.Pointer("@request.auth.id != ''")
		socialPosts.Fields.Add(
			&core.RelationField{
				Name:          "campaignId",
				Required:      true,
				CollectionId:  marketingCampaigns.Id,
				CascadeDelete: true,
				MaxSelect:     1,
			},
			&core.RelationField{
				Name:         "eventId",
				CollectionId: contentCalendarEvents.Id,
				MaxSelect:    1,
			},
			&core.SelectField{
				Name:   "platform",
				Values: []string{"Facebook", "LinkedIn", "Instagram", "Twitter", "TikTok", "YouTube", "Other"},
			},
			&core.TextField{
				Name: "content",
				Max:  10000,
			},
			&core.TextField{
				Name: "seoTitle",
				Max:  300,
			},
			&core.AutodateField{
				Name:     "created",
				OnCreate: true,
			},
			&core.AutodateField{
				Name:     "updated",
				OnCreate: true,
				OnUpdate: true,
			},
		)
		if err := app.Save(socialPosts); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Revert: delete collections in reverse dependency order
		collections := []string{
			"social_posts",
			"content_calendar_events",
			"campaign_tasks",
			"marketing_campaigns",
			"ideal_customer_profiles",
			"brand_identities",
			"worksheets",
		}
		for _, name := range collections {
			col, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			if err := app.Delete(col); err != nil {
				return err
			}
		}
		return nil
	})
}
