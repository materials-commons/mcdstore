package testdb

import (
	r "github.com/dancannon/gorethink"
	"github.com/materials-commons/mcstore/pkg/db"
)

type sessionCreater struct{}

var Sessions db.SessionCreater = &sessionCreater{}

func (_ *sessionCreater) RSession() (*r.Session, error) {
	return RSession()
}

func (_ *sessionCreater) RSessionMust() *r.Session {
	return RSessionMust()
}

func RSessionMust() *r.Session {
	return db.RSessionUsingMust("localhost:30815", "mctestdb")
}

// RSessionErr always returns a nil err. It will panic if it cannot
// get a db session. This function is meant to be used with the
// databaseSessionFilter for unit testing.
func RSession() (*r.Session, error) {
	return RSessionMust(), nil
}
