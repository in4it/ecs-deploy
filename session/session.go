package session

// inspired by EOL code: https://github.com/gin-gonic/contrib/blob/master/sessions/sessions.go

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/juju/loggo"

	"net/http"
)

const (
	DefaultKey = "sessionstore"
)

// logging
var sessionLogger = loggo.GetLogger("session")

type Session interface {
	Get(key interface{}) interface{}
	GetAll() interface{}
	Set(key interface{}, val interface{})
	Delete(key interface{})
	Clear()
	Options(sessions.Options)
	Save() error
}

type session struct {
	name    string
	request *http.Request
	store   *sessions.CookieStore
	session *sessions.Session
	written bool
	writer  http.ResponseWriter
}

func SessionHandler(name, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		store := sessions.NewCookieStore([]byte(secret))
		s := &session{name, c.Request, store, nil, false, c.Writer}
		s.session, err = s.store.Get(s.request, s.name)
		if err != nil {
			sessionLogger.Errorf("Couldn't initialize session handler (previous cookie present?): %v", err)
		}
		c.Set(DefaultKey, s)
		defer context.Clear(c.Request)
		c.Next()
	}
}

// function to retrieve the session in gin-gonic
func RetrieveSession(c *gin.Context) (Session, bool) {
	value, exists := c.Get(DefaultKey)
	return value.(Session), exists
}

func (s *session) Get(key interface{}) interface{} {
	return s.Session().Values[key]
}
func (s *session) GetAll() interface{} {
	return s.Session().Values
}

func (s *session) Set(key interface{}, val interface{}) {
	s.Session().Values[key] = val
	s.written = true
}

func (s *session) Delete(key interface{}) {
	delete(s.Session().Values, key)
	s.written = true
}

func (s *session) Clear() {
	sessionLogger.Debugf("Clear() called")
	for key := range s.Session().Values {
		sessionLogger.Debugf("Clear(): deleted key: %v", key)
		s.Delete(key)
	}
}

func (s *session) Session() *sessions.Session {
	if s.session == nil {
		sessionLogger.Errorf("Store not initialized")
	}
	return s.session
}

func (s *session) Options(options sessions.Options) {
	s.Session().Options = &sessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
	}
}
func (s *session) Save() error {
	if s.Written() {
		e := s.Session().Save(s.request, s.writer)
		if e == nil {
			sessionLogger.Debugf("Wrote session information")
			s.written = false
		}
		return e
	}
	return nil
}

func (s *session) Written() bool {
	return s.written
}
