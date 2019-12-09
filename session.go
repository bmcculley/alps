package koushin

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	imapclient "github.com/emersion/go-imap/client"
)

func generateToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

var ErrSessionExpired = errors.New("session expired")

type AuthError struct {
	cause error
}

func (err AuthError) Error() string {
	return fmt.Sprintf("authentication failed: %v", err.cause)
}

type Session struct {
	locker             sync.Mutex
	imapConn           *imapclient.Client
	username, password string
}

func (s *Session) Do(f func(*imapclient.Client) error) error {
	s.locker.Lock()
	defer s.locker.Unlock()

	return f(s.imapConn)
}

// TODO: expiration timer
type SessionManager struct {
	locker        sync.Mutex
	sessions      map[string]*Session
	newIMAPClient func() (*imapclient.Client, error)
}

func NewSessionManager(newIMAPClient func() (*imapclient.Client, error)) *SessionManager {
	return &SessionManager{
		sessions:      make(map[string]*Session),
		newIMAPClient: newIMAPClient,
	}
}

func (sm *SessionManager) connect(username, password string) (*imapclient.Client, error) {
	c, err := sm.newIMAPClient()
	if err != nil {
		return nil, err
	}

	if err := c.Login(username, password); err != nil {
		c.Logout()
		return nil, AuthError{err}
	}

	return c, nil
}

func (sm *SessionManager) Get(token string) (*Session, error) {
	sm.locker.Lock()
	defer sm.locker.Unlock()

	session, ok := sm.sessions[token]
	if !ok {
		return nil, ErrSessionExpired
	}
	return session, nil
}

func (sm *SessionManager) Put(username, password string) (token string, err error) {
	c, err := sm.connect(username, password)
	if err != nil {
		return "", err
	}

	sm.locker.Lock()
	defer sm.locker.Unlock()

	for {
		var err error
		token, err = generateToken()
		if err != nil {
			c.Logout()
			return "", err
		}

		if _, ok := sm.sessions[token]; !ok {
			break
		}
	}

	sm.sessions[token] = &Session{
		imapConn: c,
		username: username,
		password: password,
	}

	go func() {
		<-c.LoggedOut()

		sm.locker.Lock()
		delete(sm.sessions, token)
		sm.locker.Unlock()
	}()

	return token, nil
}
