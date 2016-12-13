package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/drone/drone/router/middleware/header"
	"github.com/drone/drone/router/middleware/session"
	"github.com/drone/drone/router/middleware/token"
	"github.com/drone/drone/server"
	"github.com/drone/drone/server/template"

	"github.com/drone/drone-ui/dist"
)

// Load loads the router
func Load(middleware ...gin.HandlerFunc) http.Handler {

	e := gin.New()
	e.Use(gin.Recovery())

	e.SetHTMLTemplate(template.Load())

	fs := http.FileServer(dist.AssetFS())
	e.GET("/static/*filepath", func(c *gin.Context) {
		fs.ServeHTTP(c.Writer, c.Request)
	})

	e.Use(header.NoCache)
	e.Use(header.Options)
	e.Use(header.Secure)
	e.Use(middleware...)
	e.Use(session.SetUser())
	e.Use(token.Refresh)

	e.GET("/login", server.ShowLogin)
	e.GET("/login/form", server.ShowLoginForm)
	e.GET("/logout", server.GetLogout)
	e.NoRoute(server.ShowIndex)

	// TODO above will Go away with React UI

	user := e.Group("/api/user")
	{
		user.Use(session.MustUser())
		user.GET("", server.GetSelf)
		user.GET("/feed", server.GetFeed)
		user.GET("/repos", server.GetRepos)
		user.GET("/repos/remote", server.GetRemoteRepos)
		user.POST("/token", server.PostToken)
		user.DELETE("/token", server.DeleteToken)
	}

	users := e.Group("/api/users")
	{
		users.Use(session.MustAdmin())
		users.GET("", server.GetUsers)
		users.POST("", server.PostUser)
		users.GET("/:login", server.GetUser)
		users.PATCH("/:login", server.PatchUser)
		users.DELETE("/:login", server.DeleteUser)
	}

	teams := e.Group("/api/teams")
	{
		//teams.Use(session.MustTeamAdmin())

		team := teams.Group("/:team")
		{
			team.GET("/secrets", server.GetTeamSecrets)
			team.POST("/secrets", server.PostTeamSecret)
			team.DELETE("/secrets/:secret", server.DeleteTeamSecret)
		}
	}

	global := e.Group("/api/global")
	{
		global.Use(session.MustAdmin())

		global.GET("/secrets", server.GetGlobalSecrets)
		global.POST("/secrets", server.PostGlobalSecret)
		global.DELETE("/secrets/:secret", server.DeleteGlobalSecret)
	}

	repos := e.Group("/api/repos/:owner/:name")
	{
		repos.POST("", server.PostRepo)

		repo := repos.Group("")
		{
			repo.Use(session.SetRepo())
			repo.Use(session.SetPerm())
			repo.Use(session.MustPull)

			repo.GET("", server.GetRepo)
			repo.GET("/builds", server.GetBuilds)
			repo.GET("/builds/:number", server.GetBuild)
			repo.GET("/logs/:number/:job", server.GetBuildLogs)
			repo.POST("/sign", session.MustPush, server.Sign)

			repo.GET("/secrets", session.MustPush, server.GetSecrets)
			repo.POST("/secrets", session.MustPush, server.PostSecret)
			repo.DELETE("/secrets/:secret", session.MustPush, server.DeleteSecret)

			// requires push permissions
			repo.PATCH("", session.MustPush, server.PatchRepo)
			repo.DELETE("", session.MustRepoAdmin(), server.DeleteRepo)
			repo.POST("/chown", session.MustRepoAdmin(), server.ChownRepo)

			repo.POST("/build", server.KPostBuild)
			repo.POST("/builds/:number", session.MustPush, server.PostBuild)
			repo.DELETE("/builds/:number/:job", session.MustPush, server.DeleteBuild)
		}
	}

	badges := e.Group("/api/badges/:owner/:name")
	{
		badges.GET("/status.svg", server.GetBadge)
		badges.GET("/cc.xml", server.GetCC)
	}

	e.POST("/hook", server.PostHook)
	e.POST("/api/hook", server.PostHook)

	ws := e.Group("/ws")
	{
		ws.GET("/broker", server.Broker)
		ws.GET("/feed", server.EventStream)
		ws.GET("/logs/:owner/:name/:build/:number",
			session.SetRepo(),
			session.SetPerm(),
			session.MustPull,
			server.LogStream,
		)
	}

	auth := e.Group("/authorize")
	{
		auth.GET("", server.GetLogin)
		auth.POST("", server.GetLogin)
		auth.POST("/token", server.GetLoginToken)
	}
	// ---------------------------------------------------------------------------
	// add by kci
	e.GET("/v1/user/:remote/login", server.GetKLogin)
	authk := e.Group("/v1/user/:remote/authorize")
	{
		authk.GET("", server.GetKLogin)
		authk.POST("", server.GetKLogin)
	}
	// ----------------------------------------------------------------------------

	builds := e.Group("/api/builds")
	{
		// builds.Use(session.MustAdmin())
		builds.GET("", server.GetBuildQueue)
	}

	agents := e.Group("/api/agents")
	{
		agents.Use(session.MustAdmin())
		agents.GET("", server.GetAgents)
	}

	// DELETE THESE
	// gitlab := e.Group("/gitlab/:owner/:name")
	// {
	// 	gitlab.Use(session.SetRepo())
	// 	gitlab.GET("/commits/:sha", GetCommit)
	// 	gitlab.GET("/pulls/:number", GetPullRequest)
	//
	// 	redirects := gitlab.Group("/redirect")
	// 	{
	// 		redirects.GET("/commits/:sha", RedirectSha)
	// 		redirects.GET("/pulls/:number", RedirectPullRequest)
	// 	}
	// }

	// bots := e.Group("/bots")
	// {
	// 	bots.Use(session.MustUser())
	// 	bots.POST("/slack", Slack)
	// 	bots.POST("/slack/:command", Slack)
	// }

	return e
}
