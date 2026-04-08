package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"github.com/alert666/api-server/base/middleware"
	"github.com/alert666/api-server/controller"
	_ "github.com/alert666/api-server/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type RouterInterface interface {
	RegisterRouter(engine *gin.Engine)
}

type Router struct {
	userRouter    controller.UserController
	roleRouter    controller.RoleController
	apiRouter     controller.ApiController
	middleware    middleware.MiddlewareInterface
	alert         controller.AlertManagerController
	alertTemplate controller.AlertTemplateController
	alertChannel  controller.AlertChannelController
	alertHistory  controller.AlertHistoryController
	alertSilence  controller.AlertSilenceController
}

func NewRouter(
	userRouter controller.UserController,
	roleRouter controller.RoleController,
	apiRouter controller.ApiController,
	alertmanager controller.AlertManagerController,
	middleware middleware.MiddlewareInterface,
	alertTemplate controller.AlertTemplateController,
	alertChannel controller.AlertChannelController,
	alertHistory controller.AlertHistoryController,
	alertSilence controller.AlertSilenceController,
) *Router {
	return &Router{
		userRouter:    userRouter,
		roleRouter:    roleRouter,
		apiRouter:     apiRouter,
		middleware:    middleware,
		alert:         alertmanager,
		alertTemplate: alertTemplate,
		alertChannel:  alertChannel,
		alertHistory:  alertHistory,
		alertSilence:  alertSilence,
	}
}

func (r *Router) RegisterRouter(engine *gin.Engine) {
	engine.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	engine.Use(ginzap.GinzapWithConfig(zap.L(), &ginzap.Config{
		Context: ginzap.Fn(func(c *gin.Context) []zapcore.Field {
			fields := []zapcore.Field{}
			if requestID := requestid.Get(c); requestID != "" {
				fields = append(fields, zap.String("request-id", requestID))
			}
			return fields
		}),
	}))

	engine.Use(ginzap.RecoveryWithZap(zap.L(), true))
	engine.Use(requestid.New())

	apiGroup := engine.Group("/api/v1")
	apiGroup.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.registerOAuthRouter(apiGroup)
	r.registerUserRouter(apiGroup)
	r.registerRoleRouter(apiGroup)
	r.registerApiRouter(apiGroup)
	r.registerAlertmanagerRouter(apiGroup)
	r.registerAlertTemplateRouter(apiGroup)
	r.registerAlertChannelRouter(apiGroup)
	r.registerHistoryRouter(apiGroup)
	r.registerAlertSilenceRouter(apiGroup)
}

func (r *Router) registerUserRouter(apiGroup *gin.RouterGroup) {
	userGroup := apiGroup.Group("/user")
	{
		userGroup.POST("/login", r.userRouter.UserLoginController)
		userGroup.Use(r.middleware.Auth())
		userGroup.POST("/logout", r.userRouter.UserLogoutController)
		userGroup.GET("/info", r.userRouter.UserInfoController)
		userGroup.PUT("/self", r.userRouter.UserUpdateBySelfController)
		userGroup.Use(r.middleware.AuthZ())
		userGroup.POST("/register", r.userRouter.UserCreateController)
		userGroup.PUT("/:id", r.userRouter.UserUpdateByAdminController)
		userGroup.GET("/:id", r.userRouter.UserQueryController)
		userGroup.GET("", r.userRouter.UserListController)
		userGroup.DELETE("/:id", r.userRouter.UserDeleteController)
	}
}

func (r *Router) registerRoleRouter(apiGroup *gin.RouterGroup) {
	roleGroup := apiGroup.Group("/role")
	{
		roleGroup.Use(r.middleware.Auth(), r.middleware.AuthZ())
		roleGroup.POST("", r.roleRouter.CreateRole)
		roleGroup.PUT("/:id", r.roleRouter.UpdateRole)
		roleGroup.DELETE("/:id", r.roleRouter.DeleteRole)
		roleGroup.GET("/:id", r.roleRouter.QueryRole)
		roleGroup.GET("", r.roleRouter.ListRole)
	}
}

func (r *Router) registerApiRouter(apiGroup *gin.RouterGroup) {
	baseGroup := apiGroup.Group("/api")
	{
		baseGroup.Use(r.middleware.Auth(), r.middleware.AuthZ())
		baseGroup.GET("/serverApi", r.apiRouter.GetServerApi)
		baseGroup.POST("", r.apiRouter.CreateApi)
		baseGroup.PUT("/:id", r.apiRouter.UpdateApi)
		baseGroup.DELETE("/:id", r.apiRouter.DeleteApi)
		baseGroup.GET("/:id", r.apiRouter.QueryApi)
		baseGroup.GET("", r.apiRouter.ListApi)
	}
}

func (r *Router) registerAlertmanagerRouter(apiGroup *gin.RouterGroup) {
	baseGroup := apiGroup.Group("/alerts")
	{
		baseGroup.POST("", r.alert.ReceiveAlerts)
	}
}

func (r *Router) registerAlertTemplateRouter(apiGroup *gin.RouterGroup) {
	baseGroup := apiGroup.Group("/alertTemplate")
	{
		baseGroup.Use(r.middleware.Auth(), r.middleware.AuthZ())
		baseGroup.POST("", r.alertTemplate.CreateAlertTemplate)
		baseGroup.PUT("/:id", r.alertTemplate.UpdateAlertTemplate)
		baseGroup.DELETE("/:id", r.alertTemplate.DeleteAlertTemplate)
		baseGroup.GET("/:id", r.alertTemplate.QueryAlertTemplate)
		baseGroup.GET("", r.alertTemplate.ListAlertTemplate)
	}
}

func (r *Router) registerAlertChannelRouter(apiGroup *gin.RouterGroup) {
	baseGroup := apiGroup.Group("/alertChannel")
	{
		baseGroup.Use(r.middleware.Auth(), r.middleware.AuthZ())
		baseGroup.POST("", r.alertChannel.CreateAlertChannel)
		baseGroup.PUT("/:id", r.alertChannel.UpdateAlertChannel)
		baseGroup.DELETE("/:id", r.alertChannel.DeleteAlertChannel)
		baseGroup.GET("/:id", r.alertChannel.QueryAlertChannel)
		baseGroup.GET("", r.alertChannel.ListAlertChannel)
	}
}

func (r *Router) registerAlertSilenceRouter(apiGroup *gin.RouterGroup) {
	baseGroup := apiGroup.Group("/alertSilence")
	{
		baseGroup.Use(r.middleware.Auth(), r.middleware.AuthZ())
		baseGroup.POST("", r.alertSilence.CreateAlertSilence)
		baseGroup.DELETE("/:id", r.alertSilence.DeleteAlertSilence)
		baseGroup.GET("/:id", r.alertSilence.QueryAlertSilence)
		baseGroup.GET("", r.alertSilence.ListAlertSilence)
	}
}

func (r *Router) registerHistoryRouter(apiGroup *gin.RouterGroup) {
	baseGroup := apiGroup.Group("/alertHistory")
	{
		baseGroup.Use(r.middleware.Auth(), r.middleware.AuthZ())
		baseGroup.GET("/:id", r.alertHistory.QueryAlertHistory)
		baseGroup.GET("", r.alertHistory.ListAlertHistory)
	}
}

func (r *Router) registerOAuthRouter(apiGroup *gin.RouterGroup) {
	oauthGroup := apiGroup.Group("/oauth2")
	oauthGroup.Use(r.middleware.Session())
	{
		oauthGroup.GET("/provider", r.userRouter.OAuth2ProviderController)
		oauthGroup.GET("/login", r.userRouter.OAuth2LoginController)
		oauthGroup.GET("/callback", r.userRouter.OAuth2CallbackController)
		oauthGroup.POST("/:id", r.userRouter.OAuth2ActivateController)
	}
}
