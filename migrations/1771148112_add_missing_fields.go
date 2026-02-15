// migrations/1771148112_add_missing_fields.go
// Adds missing fields to all collections based on the updated schema spec.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"

	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		usersCollection, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		// ======================================================
		// 1. brand_identities — add: userId, slogan, missionStatement, keywords
		// ======================================================
		brandIdentities, err := app.FindCollectionByNameOrId("brand_identities")
		if err != nil {
			return err
		}

		worksheets, err := app.FindCollectionByNameOrId("worksheets")
		if err != nil {
			return err
		}

		brandIdentities.Fields.Add(
			&core.RelationField{
				Name:         "userId",
				CollectionId: usersCollection.Id,
				MaxSelect:    1,
			},
			&core.TextField{
				Name: "slogan",
				Max:  500,
			},
			&core.TextField{
				Name: "missionStatement",
				Max:  5000,
			},
			&core.JSONField{
				Name: "keywords",
			},
		)
		if err := app.Save(brandIdentities); err != nil {
			return err
		}

		// ======================================================
		// 2. ideal_customer_profiles — add: userId, summary,
		//    psychographics, goalsAndMotivations, painPointsAndChallenges
		// ======================================================
		idealCustomerProfiles, err := app.FindCollectionByNameOrId("ideal_customer_profiles")
		if err != nil {
			return err
		}

		idealCustomerProfiles.Fields.Add(
			&core.RelationField{
				Name:         "userId",
				CollectionId: usersCollection.Id,
				MaxSelect:    1,
			},
			&core.TextField{
				Name: "summary",
				Max:  5000,
			},
			&core.JSONField{
				Name: "psychographics",
			},
			&core.JSONField{
				Name: "goalsAndMotivations",
			},
			&core.JSONField{
				Name: "painPointsAndChallenges",
			},
		)
		if err := app.Save(idealCustomerProfiles); err != nil {
			return err
		}

		// ======================================================
		// 3. marketing_campaigns — add: userId, positioning,
		//    valueProposition, toneOfVoice
		// ======================================================
		marketingCampaigns, err := app.FindCollectionByNameOrId("marketing_campaigns")
		if err != nil {
			return err
		}

		marketingCampaigns.Fields.Add(
			&core.RelationField{
				Name:         "userId",
				CollectionId: usersCollection.Id,
				MaxSelect:    1,
			},
			&core.TextField{
				Name: "positioning",
				Max:  5000,
			},
			&core.TextField{
				Name: "valueProposition",
				Max:  5000,
			},
			&core.TextField{
				Name: "toneOfVoice",
				Max:  2000,
			},
		)
		if err := app.Save(marketingCampaigns); err != nil {
			return err
		}

		// ======================================================
		// 4. campaign_tasks — add: userId, description
		// ======================================================
		campaignTasks, err := app.FindCollectionByNameOrId("campaign_tasks")
		if err != nil {
			return err
		}

		campaignTasks.Fields.Add(
			&core.RelationField{
				Name:         "userId",
				CollectionId: usersCollection.Id,
				MaxSelect:    1,
			},
			&core.TextField{
				Name: "description",
				Max:  5000,
			},
		)
		if err := app.Save(campaignTasks); err != nil {
			return err
		}

		// ======================================================
		// 5. content_calendar_events — add: worksheetId, userId,
		//    description, contentSuggestion, socialPostIds
		//    + update eventType values
		// ======================================================
		contentCalendarEvents, err := app.FindCollectionByNameOrId("content_calendar_events")
		if err != nil {
			return err
		}

		contentCalendarEvents.Fields.Add(
			&core.RelationField{
				Name:         "worksheetId",
				CollectionId: worksheets.Id,
				MaxSelect:    1,
			},
			&core.RelationField{
				Name:         "userId",
				CollectionId: usersCollection.Id,
				MaxSelect:    1,
			},
			&core.TextField{
				Name: "description",
				Max:  5000,
			},
			&core.TextField{
				Name: "contentSuggestion",
				Max:  10000,
			},
			&core.JSONField{
				Name: "socialPostIds",
			},
			// Override eventType with the correct values from spec
			&core.SelectField{
				Name:   "eventType",
				Values: []string{"Holiday", "Trending Topic", "Cultural Event", "Brand Milestone"},
			},
		)
		if err := app.Save(contentCalendarEvents); err != nil {
			return err
		}

		// ======================================================
		// 6. social_posts — add: userId, seoDescription, imageUrl
		// ======================================================
		socialPosts, err := app.FindCollectionByNameOrId("social_posts")
		if err != nil {
			return err
		}

		socialPosts.Fields.Add(
			&core.RelationField{
				Name:         "userId",
				CollectionId: usersCollection.Id,
				MaxSelect:    1,
			},
			&core.TextField{
				Name: "seoDescription",
				Max:  500,
			},
			&core.URLField{
				Name: "imageUrl",
			},
		)
		if err := app.Save(socialPosts); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// ----------------------------------------------------------
		// Down: remove the fields added by this migration
		// ----------------------------------------------------------
		fieldsToRemove := map[string][]string{
			"brand_identities":        {"userId", "slogan", "missionStatement", "keywords"},
			"ideal_customer_profiles": {"userId", "summary", "psychographics", "goalsAndMotivations", "painPointsAndChallenges"},
			"marketing_campaigns":     {"userId", "positioning", "valueProposition", "toneOfVoice"},
			"campaign_tasks":          {"userId", "description"},
			"content_calendar_events": {"worksheetId", "userId", "description", "contentSuggestion", "socialPostIds"},
			"social_posts":            {"userId", "seoDescription", "imageUrl"},
		}

		for collectionName, fields := range fieldsToRemove {
			collection, err := app.FindCollectionByNameOrId(collectionName)
			if err != nil {
				return err
			}
			for _, fieldName := range fields {
				collection.Fields.RemoveByName(fieldName)
			}
			if err := app.Save(collection); err != nil {
				return err
			}
		}

		// Restore original eventType values for content_calendar_events
		contentCalendarEvents, err := app.FindCollectionByNameOrId("content_calendar_events")
		if err != nil {
			return err
		}
		contentCalendarEvents.Fields.Add(
			&core.SelectField{
				Name:   "eventType",
				Values: []string{"Blog Post", "Social Media", "Email", "Webinar", "Event", "Ad Campaign", "Other"},
			},
		)
		if err := app.Save(contentCalendarEvents); err != nil {
			return err
		}

		return nil
	})
}
