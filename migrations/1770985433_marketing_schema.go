package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// 1. BusinessIdea
		bizIdea := core.NewBaseCollection("business_ideas", "biz_ideas_pbc01")
		bizIdea.Fields.Add(
			&core.RelationField{Name: "userId", CollectionId: "_pb_users_auth_", MaxSelect: 1, Required: true},
			&core.TextField{Name: "rawIdea", Required: true},
			&core.JSONField{Name: "productCore"}, // Chứa problem, solution, usp
		)

		// 2. BrandIdentity
		brandIden := core.NewBaseCollection("brand_identities", "brand_iden_pbc1")
		brandIden.Fields.Add(
			&core.TextField{Name: "brandName", Required: true},
			&core.TextField{Name: "slogan"},
			&core.TextField{Name: "missionStatement"},
			&core.URLField{Name: "logoUrl"},
			&core.JSONField{Name: "colorPalette"},
			&core.JSONField{Name: "keywords"},
		)

		// 3. IdealCustomerProfile
		customerProf := core.NewBaseCollection("customer_profiles", "cust_prof_pbc01")
		customerProf.Fields.Add(
			&core.RelationField{Name: "userId", CollectionId: "_pb_users_auth_", MaxSelect: 1},
			&core.TextField{Name: "personaName", Required: true},
			&core.TextField{Name: "summary"},
			&core.JSONField{Name: "demographics"},
			&core.JSONField{Name: "psychographics"},
			&core.JSONField{Name: "goalsAndMotivations"},
			&core.JSONField{Name: "painPointsAndChallenges"},
		)

		// 4. MarketingCampaign
		campaign := core.NewBaseCollection("marketing_campaigns", "mark_camp_pbc01")
		campaign.Fields.Add(
			&core.RelationField{Name: "userId", CollectionId: "_pb_users_auth_", MaxSelect: 1},
			&core.TextField{Name: "name", Required: true},
			&core.TextField{Name: "goal"},
			&core.SelectField{Name: "language", Values: []string{"en", "vi"}},
			&core.TextField{Name: "acquisitionStrategy"},
			&core.TextField{Name: "positioning"},
			&core.TextField{Name: "valueProposition"},
			&core.TextField{Name: "toneOfVoice"},
		)

		// 5. CampaignTask
		task := core.NewBaseCollection("campaign_tasks", "camp_task_pbc01")
		task.Fields.Add(
			&core.RelationField{Name: "campaignId", CollectionId: "mark_camp_pbc01", MaxSelect: 1},
			&core.TextField{Name: "taskName", Required: true},
			&core.TextField{Name: "description"},
			&core.SelectField{Name: "language", Values: []string{"en", "vi"}},
			&core.NumberField{Name: "week"},
			&core.SelectField{Name: "status", Values: []string{"To Do", "In Progress", "Done"}},
		)

		// 6. ContentCalendarEvent
		event := core.NewBaseCollection("content_calendar_events", "cont_cale_pbc01")
		event.Fields.Add(
			&core.RelationField{Name: "campaignId", CollectionId: "mark_camp_pbc01", MaxSelect: 1},
			&core.RelationField{Name: "userId", CollectionId: "_pb_users_auth_", MaxSelect: 1},
			&core.TextField{Name: "title", Required: true},
			&core.TextField{Name: "description"},
			&core.DateField{Name: "eventDate"},
			&core.SelectField{Name: "eventType", Values: []string{"Holiday", "Trending Topic", "Cultural Event", "Brand Milestone"}},
			&core.TextField{Name: "aiAnalysis"},
			&core.TextField{Name: "contentSuggestion"},
			&core.JSONField{Name: "socialPostIds"},
		)

		// 7. SocialPost
		post := core.NewBaseCollection("social_posts", "social_po_pbc01")
		post.Fields.Add(
			&core.RelationField{Name: "campaignId", CollectionId: "mark_camp_pbc01", MaxSelect: 1},
			&core.RelationField{Name: "eventId", CollectionId: "cont_cale_pbc01", MaxSelect: 1},
			&core.RelationField{Name: "userId", CollectionId: "_pb_users_auth_", MaxSelect: 1},
			&core.SelectField{Name: "platform", Values: []string{"Web", "LinkedIn", "Facebook", "Twitter", "Instagram"}},
			&core.TextField{Name: "content", Required: true},
			&core.TextField{Name: "seoTitle"},
			&core.TextField{Name: "seoDescription"},
			&core.URLField{Name: "imageUrl"},
		)

		// Lưu tất cả các collection vào DB
		collections := []*core.Collection{bizIdea, brandIden, customerProf, campaign, task, event, post}
		for _, c := range collections {
			if err := app.Save(c); err != nil {
				return err
			}
		}

		return nil
	}, func(app core.App) error {
		// Hàm Down: Xóa bảng nếu cần Rollback (ngược thứ tự tạo)
		names := []string{"social_posts", "content_calendar_events", "campaign_tasks", "marketing_campaigns", "customer_profiles", "brand_identities", "business_ideas"}
		for _, name := range names {
			collection, _ := app.FindCollectionByNameOrId(name)
			if collection != nil {
				if err := app.Delete(collection); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
