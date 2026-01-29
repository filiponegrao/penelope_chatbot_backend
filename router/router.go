package router

import (
	"log"

	"penelope/config"
	"penelope/controllers"
	"penelope/middleware"

	"github.com/gin-gonic/gin"
)

// Initialize wires all routes and middlewares.
// It mirrors Venditto's approach: public routes + authenticated routes + "validated" routes (Authorizer).
func Initialize(r *gin.Engine, cfg config.Configuration) {
	_ = cfg

	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())

	api := r.Group("/api")

	// Webhook (WhatsApp) - multi-tenant: /webhook/:userId
	// Mantém /webhook funcionando em dev via env WEBHOOK_DEFAULT_USER_ID
	api.GET("/webhook", controllers.WebhookVerify)
	api.POST("/webhook", controllers.WebhookUpdate)
	api.GET("/webhook/:userId", controllers.WebhookVerify)
	api.POST("/webhook/:userId", controllers.WebhookUpdate)

	// Public (no auth)
	api.POST("/users", Logger(), controllers.CreateUser)
	api.POST("/login", Logger(), controllers.Login)
	api.POST("/refresh", Logger(), controllers.Refresh)

	// Authenticated routes (token required)
	auth := api.Group("")
	auth.Use(controllers.AuthRequired())
	auth.POST("/user/resend-code", Logger(), controllers.ResendActivationCode)
	auth.POST("/user/activate/:code", Logger(), controllers.ActivateUserByCode)

	// Validated routes (token + active user)
	validated := auth.Group("")
	validated.Use(Authorizer())

	// Public routes
	validated.GET("/plans", Logger(), controllers.GetPlans)
	validated.GET("/plans/:id", Logger(), controllers.GetPlanByID)

	// List modules by plan (admin)
	validated.GET("/plans/:id/modules", Logger(), controllers.GetModulesByPlanID)

	// Example protected endpoint (useful for smoke tests)
	validated.GET("/me", Logger(), controllers.Me)

	// Plans (user)
	validated.GET("/plans/user", Logger(), controllers.GetUserPlans)
	validated.POST("/plans/purchase", Logger(), controllers.PurchasePlan)
	validated.POST("/plans/cancel", Logger(), controllers.CancelPlan)

	// Modules/Inputs for user
	validated.GET("/modules/user", Logger(), controllers.GetModulesForUser)
	validated.GET("/inputs/user", Logger(), controllers.GetInputsForUser)

	// User Inputs (user)
	validated.GET("/user-inputs", Logger(), controllers.GetUserInputs)
	validated.GET("/user-inputs/:id", Logger(), controllers.GetUserInputByID)
	validated.POST("/user-inputs", Logger(), controllers.CreateUserInput)
	validated.PUT("/user-inputs/:id", Logger(), controllers.UpdateUserInput)
	validated.DELETE("/user-inputs/:id", Logger(), controllers.DeleteUserInput)

	// Events Dashboard (client)
	validated.GET("/events/dashboard/processed-per-day", Logger(), controllers.GetEventsProcessedPerDay)
	validated.GET("/events/dashboard/monthly-usage", Logger(), controllers.GetEventsMonthlyUsage)
	validated.GET("/events/dashboard/list", Logger(), controllers.GetEventsDashboardList)

	// Admin routes
	admin := validated.Group("")
	admin.Use(Adminizer())

	// Plans CRUD (admin)
	admin.POST("/plans", Logger(), controllers.CreatePlan)
	admin.PUT("/plans/:id", Logger(), controllers.UpdatePlan)
	admin.DELETE("/plans/:id", Logger(), controllers.DeletePlan)

	// Modules CRUD (admin)
	admin.GET("/modules", Logger(), controllers.GetModules)
	admin.GET("/modules/:id", Logger(), controllers.GetModuleByID)
	admin.POST("/modules", Logger(), controllers.CreateModule)
	admin.PUT("/modules/:id", Logger(), controllers.UpdateModule)
	admin.DELETE("/modules/:id", Logger(), controllers.DeleteModule)

	// Link plan <-> module (admin)
	admin.POST("/plan-modules", Logger(), controllers.AddModuleToPlan)
	admin.DELETE("/plan-modules", Logger(), controllers.RemoveModuleFromPlan)

	// Inputs CRUD (admin)
	admin.GET("/inputs", Logger(), controllers.GetInputs)
	admin.GET("/inputs/:id", Logger(), controllers.GetInputByID)
	admin.POST("/inputs", Logger(), controllers.CreateInput)
	admin.PUT("/inputs/:id", Logger(), controllers.UpdateInput)
	admin.DELETE("/inputs/:id", Logger(), controllers.DeleteInput)

	// Link module <-> input (admin) - IDs no body (igual plan-modules)
	admin.POST("/module-inputs", Logger(), controllers.AddInputToModule)
	admin.DELETE("/module-inputs", Logger(), controllers.RemoveInputFromModule)

	// Events (admin)
	admin.GET("/events", Logger(), controllers.GetEvents)
	admin.GET("/events/:id", Logger(), controllers.GetEventByID)

	log.Printf("Routes initialized")
}
