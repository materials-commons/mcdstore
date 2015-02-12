package dai

import (
	r "github.com/dancannon/gorethink"
	"github.com/materials-commons/mcstore/pkg/db/model"
	"github.com/materials-commons/mcstore/pkg/db/schema"
)

// rProjects implements the Projects interface for RethinkDB
type rProjects struct {
	session *r.Session
}

// NewRProjects creates a new instance of rProjects.
func NewRProjects(session *r.Session) rProjects {
	return rProjects{
		session: session,
	}
}

// ByID looks up a project by the given id.
func (p rProjects) ByID(id string) (*schema.Project, error) {
	var project schema.Project
	if err := model.Projects.Qs(p.session).ByID(id, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// HasDirectory checks if the given directoryID is in the given project.
func (p rProjects) HasDirectory(projectID, dirID string) bool {
	rql := model.ProjectDirs.T().GetAllByIndex("datadir_id", dirID)
	var proj2dir []schema.Project2DataDir
	if err := model.ProjectDirs.Qs(p.session).Rows(rql, &proj2dir); err != nil {
		return false
	}

	// Look for matching projectID
	for _, entry := range proj2dir {
		if entry.ProjectID == projectID {
			return true
		}
	}

	return false
}

// Files returns the files for a project.
func (p rProjects) Files(projectID string) ([]schema.Directory, error) {
	rql := r.Table("project2datadir").GetAllByIndex("project_id", projectID).
		EqJoin("datadir_id", r.Table("datadirs")).
		Zip().
		Merge(map[string]interface{}{
		"datafiles": r.Table("datadir2datafile").
			GetAllByIndex("datadir_id", r.Row.Field("id")).
			EqJoin("datafile_id", r.Table("datafiles")).
			Zip().CoerceTo("ARRAY"),
	})
	var dirs []schema.Directory
	if err := model.Dirs.Qs(p.session).Rows(rql, &dirs); err != nil {
		return nil, err
	}
	return dirs, nil
}
