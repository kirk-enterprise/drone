package server

import (
	"encoding/base32"
	"errors"
	"net/http"
	"strconv"

	jose "github.com/square/go-jose"

	"github.com/drone/drone/model"
	"github.com/drone/drone/remote"
	"github.com/drone/drone/router/middleware/session"
	"github.com/drone/drone/shared/httputil"
	"github.com/drone/drone/shared/token"
	"github.com/drone/drone/store"
	"github.com/drone/drone/yaml"
	"github.com/gorilla/securecookie"

	log "github.com/Sirupsen/logrus"
	"github.com/drone/mq/stomp"
	"github.com/gin-gonic/gin"
)

//------------------------------------------------------------
//                 这个文件是kci添加使用
//------------------------------------------------------------

//------------------------------------------------------------
// kciIndex login and get user response
func GetKLogin(c *gin.Context) {

	// when dealing with redirects we may need to adjust the content type. I
	// cannot, however, remember why, so need to revisit this line.
	c.Writer.Header().Del("Content-Type")

	tmpuser, err := remote.Login(c, c.Writer, c.Request)
	if err != nil {
		log.Errorf("cannot authenticate user. %s", err)
		c.AbortWithError(400, err)
		return
	}
	// this will happen when the user is redirected by the remote provider as
	// part of the authorization workflow.
	if tmpuser == nil {
		return
	}

	// get the user from the database
	u, err := store.GetUserLogin(c, tmpuser.Login)
	if err != nil {

		// create the user account
		u = &model.User{
			Login:  tmpuser.Login,
			Token:  tmpuser.Token,
			Secret: tmpuser.Secret,
			Email:  tmpuser.Email,
			Avatar: tmpuser.Avatar,
			Hash: base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(32),
			),
		}

		// insert the user into the database
		if err := store.CreateUser(c, u); err != nil {
			log.Errorf("cannot insert %s. %s", u.Login, err)
			c.AbortWithError(400, err)
			return
		}
	}

	// update the user meta data and authorization data.
	u.Token = tmpuser.Token
	u.Secret = tmpuser.Secret
	u.Email = tmpuser.Email
	u.Avatar = tmpuser.Avatar

	// if self-registration is enabled for whitelisted organizations we need to
	// check the user's organization membership.
	if err := store.UpdateUser(c, u); err != nil {
		log.Errorf("cannot update %s. %s", u.Login, err)
		c.AbortWithError(400, err)
		return
	}

	token := token.New(token.UserToken, u.Login)
	tokenstr, err := token.Sign(u.Hash)
	if err != nil {
		log.Errorf("cannot create token for %s. %s", u.Login, err)
		c.AbortWithError(400, err)
		return
	}
	c.JSON(200, gin.H{"user": u, "token": tokenstr})
}

//------------------------------------------------------------
// 使用 message 传送[owner/name] custom message
func KPostBuild(c *gin.Context) {
	remote_ := remote.FromContext(c)
	repo := session.Repo(c)
	user := session.User(c)

	build := &model.Build{}
	if err := c.Bind(build); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if repo == nil {
		err := errors.New("failure to find repo")
		log.Errorf("%s", err)
		c.AbortWithError(404, err)
		return
	}

	if repo.UserID == 0 {
		log.Warnf("ignoring hook. repo %s has no owner.", repo.FullName)
		c.Writer.WriteHeader(204)
		return
	}

	// if there is no email address associated with the pull request,
	// we lookup the email address based on the authors github login.
	//
	// my initial hesitation with this code is that it has the ability
	// to expose your email address. At the same time, your email address
	// is already exposed in the public .git log. So while some people will
	// a small number of people will probably be upset by this, I'm not sure
	// it is actually that big of a deal.
	if len(build.Email) == 0 {
		author, err := store.GetUserLogin(c, build.Author)
		if err == nil {
			build.Email = author.Email
		}
	}

	// if the remote has a refresh token, the current access token
	// may be stale. Therefore, we should refresh prior to dispatching
	// the job.
	if refresher, ok := remote_.(remote.Refresher); ok {
		ok, _ := refresher.Refresh(user)
		if ok {
			store.UpdateUser(c, user)
		}
	}

	// fetch the build file from the database
	config := ToConfig(c)
	raw, err := remote_.File(user, repo, build, config.Yaml)
	if err != nil {
		err = errors.New("failure to get build config for " + repo.FullName + err.Error())
		log.Error(err.Error())
		c.AbortWithError(404, err)
		return
	}
	sec, err := remote_.File(user, repo, build, config.Shasum)
	if err != nil {
		log.Debugf("cannot find build secrets for %s. %s", repo.FullName, err)
		// NOTE we don't exit on failure. The sec file is optional
	}

	axes, err := yaml.ParseMatrix(raw)
	if err != nil {
		c.String(500, "Failed to parse yaml file or calculate matrix. %s", err)
		return
	}
	if len(axes) == 0 {
		axes = append(axes, yaml.Axis{})
	}

	netrc, err := remote_.Netrc(user, repo)
	if err != nil {
		c.String(500, "Failed to generate netrc file. %s", err)
		return
	}

	// verify the branches can be built vs skipped
	branches := yaml.ParseBranch(raw)
	if !branches.Match(build.Branch) && build.Event != model.EventManual {
		c.String(200, "Branch does not match restrictions defined in yaml")
		return
	}

	signature, err := jose.ParseSigned(string(sec))
	if err != nil {
		log.Debugf("cannot parse .drone.yml.sig file. %s", err)
	} else if len(sec) == 0 {
		log.Debugf("cannot parse .drone.yml.sig file. empty file")
	} else {
		build.Signed = true
		output, err := signature.Verify([]byte(repo.Hash))
		if err != nil {
			log.Debugf("cannot verify .drone.yml.sig file. %s", err)
		} else if string(output) != string(raw) {
			log.Debugf("cannot verify .drone.yml.sig file. no match")
		} else {
			build.Verified = true
		}
	}

	// update some build fields
	build.Status = model.StatusPending
	build.RepoID = repo.ID

	// and use a transaction
	var jobs []*model.Job
	log.Debug("raw yml", string(raw))
	log.Debug("axes", axes)
	for num, axis := range axes {
		jobs = append(jobs, &model.Job{
			BuildID:     build.ID,
			Number:      num + 1,
			Status:      model.StatusPending,
			Environment: axis,
		})
	}
	err = store.CreateBuild(c, build, jobs...)
	if err != nil {
		log.Errorf("failure to save commit for %s. %s", repo.FullName, err)
		c.AbortWithError(500, err)
		return
	}

	c.JSON(200, build)

	// get the previous build so that we can send
	// on status change notifications
	last, _ := store.GetBuildLastBefore(c, repo, build.Branch, build.ID)
	secs, err := store.GetMergedSecretList(c, repo)
	if err != nil {
		log.Debugf("Error getting secrets for %s#%d. %s", repo.FullName, build.Number, err)
	}
	client := stomp.MustFromContext(c)
	client.SendJSON("/topic/events", model.Event{
		Type:  model.Enqueued,
		Repo:  *repo,
		Build: *build,
	},
		stomp.WithHeader("repo", repo.FullName),
		stomp.WithHeader("private", strconv.FormatBool(repo.IsPrivate)),
	)
	for _, job := range jobs {
		broker, _ := stomp.FromContext(c)
		broker.SendJSON("/queue/pending", &model.Work{
			Signed:    build.Signed,
			Verified:  build.Verified,
			User:      user,
			Repo:      repo,
			Build:     build,
			BuildLast: last,
			Job:       job,
			Netrc:     netrc,
			Yaml:      string(raw),
			Secrets:   secs,
			System:    &model.System{Link: httputil.GetURL(c.Request)},
		},
			stomp.WithHeader(
				"platform",
				yaml.ParsePlatformDefault(raw, "linux/amd64"),
			),
			stomp.WithHeaders(
				yaml.ParseLabel(raw),
			),
		)
	}
}
