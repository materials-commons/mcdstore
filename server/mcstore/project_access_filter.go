package mcstore

import (
	"net/http"

	r "github.com/dancannon/gorethink"
	"github.com/emicklei/go-restful"
	"github.com/materials-commons/mcstore/pkg/app"
	"github.com/materials-commons/mcstore/pkg/db/dai"
	"github.com/materials-commons/mcstore/pkg/db/schema"
	"github.com/materials-commons/mcstore/pkg/domain"
)

type projectAccessFilterDAI struct {
	projects dai.Projects
	access   domain.Access
}

func newProjectAccessFilterDAI(session *r.Session) *projectAccessFilterDAI {
	files := dai.NewRFiles(session)
	users := dai.NewRUsers(session)
	projects := dai.NewRProjects(session)
	access := domain.NewAccess(projects, files, users)
	return &projectAccessFilterDAI{
		projects: projects,
		access:   access,
	}
}

type projectIDAccess struct {
	ProjectID string `json:"project_id"`
}

func projectAccessFilter(request *restful.Request, response *restful.Response, chain *restful.FilterChain) {
	user := request.Attribute("user").(schema.User)
	session := request.Attribute("session").(*r.Session)
	var p projectIDAccess

	if err := request.ReadEntity(&p); err != nil {
		response.WriteErrorString(http.StatusNotAcceptable, "No project_id found")
		return
	}

	f := newProjectAccessFilterDAI(session)
	if project, err := f.getProjectValidatingAccess(p.ProjectID, user.ID); err != nil {
		response.WriteErrorString(http.StatusUnauthorized, "No access to project")
	} else {
		request.SetAttribute("project", *project)
		chain.ProcessFilter(request, response)
	}
}

// getProjectValidatingAccess retrieves the project with the given projectID. It checks that the
// given user has access to that project.
func (f *projectAccessFilterDAI) getProjectValidatingAccess(projectID, user string) (*schema.Project, error) {
	project, err := f.projects.ByID(projectID)
	switch {
	case err != nil:
		return nil, err
	case !f.access.AllowedByOwner(projectID, user):
		return nil, app.ErrNoAccess
	default:
		return project, nil
	}
}
