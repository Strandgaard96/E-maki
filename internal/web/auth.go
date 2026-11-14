package web

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Auth is the optional password on the settings page. Only a salted
// PBKDF2 hash is kept, in its own file next to the settings, and signed-in
// browsers hold a cookie signed with a key from the same file. Setting a new
// password makes a new key, which signs every browser out.
type Auth struct {
	Path string // "" keeps the password in memory only, until restart

	mu    sync.Mutex
	data  authData
	fails map[string]fail
}

type authData struct {
	Salt []byte `json:"salt,omitempty"`
	Hash []byte `json:"hash,omitempty"`
	Key  []byte `json:"key,omitempty"`
}

type fail struct {
	n    int
	last time.Time
}

const (
	iterations = 600_000 // OWASP's figure for PBKDF2-HMAC-SHA256
	sessionFor = 30 * 24 * time.Hour
	cookieName = "ekiru_session"
)

// LoadAuth reads the password file; a missing file means no password.
func LoadAuth(path string) (*Auth, error) {
	a := &Auth{Path: path}
	if path == "" {
		return a, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &a.data); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return a, nil
}

// Enabled reports whether a password is set.
func (a *Auth) Enabled() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.data.Hash) > 0
}

// Set changes the password; "" removes it.
func (a *Auth) Set(password string) error {
	d := authData{}
	if password != "" {
		d.Salt, d.Key = random(16), random(32)
		h, err := pbkdf2.Key(sha256.New, password, d.Salt, iterations, 32)
		if err != nil {
			return err
		}
		d.Hash = h
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Path != "" {
		if password == "" {
			if err := os.Remove(a.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else {
			b, _ := json.Marshal(d)
			tmp := a.Path + ".tmp"
			if err := os.WriteFile(tmp, b, 0o600); err != nil {
				return err
			}
			if err := os.Rename(tmp, a.Path); err != nil {
				return err
			}
		}
	}
	a.data = d
	return nil
}

func random(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// Check compares a password with the stored hash. Too many wrong tries from
// one address lock it out for a while, so a password cannot be guessed fast.
func (a *Auth) Check(r *http.Request, password string) (ok bool, wait time.Duration) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	a.mu.Lock()
	d, f := a.data, a.fails[ip]
	a.mu.Unlock()
	if f.n >= 5 {
		lock := min(time.Minute<<(f.n-5), 15*time.Minute)
		if left := time.Until(f.last.Add(lock)); left > 0 {
			return false, left
		}
	}
	h, err := pbkdf2.Key(sha256.New, password, d.Salt, iterations, 32)
	ok = err == nil && len(d.Hash) > 0 && subtle.ConstantTimeCompare(h, d.Hash) == 1
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.fails == nil {
		a.fails = map[string]fail{}
	}
	if ok {
		delete(a.fails, ip)
	} else {
		a.fails[ip] = fail{f.n + 1, time.Now()}
	}
	return ok, 0
}

// SignIn gives the browser a session cookie.
func (a *Auth) SignIn(w http.ResponseWriter, path string) {
	until := time.Now().Add(sessionFor).Unix()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: a.sign(until), Path: path,
		MaxAge: int(sessionFor.Seconds()), HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

// SignOut clears the browser's cookie.
func SignOut(w http.ResponseWriter, path string) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: path, MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func (a *Auth) sign(until int64) string {
	a.mu.Lock()
	key := a.data.Key
	a.mu.Unlock()
	m := hmac.New(sha256.New, key)
	fmt.Fprint(m, until)
	return strconv.FormatInt(until, 10) + "." + hex.EncodeToString(m.Sum(nil))
}

// SignedIn reports whether r may use the settings page: no password is set,
// or it carries a valid session cookie.
func (a *Auth) SignedIn(r *http.Request) bool {
	if !a.Enabled() {
		return true
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	ts, _, _ := strings.Cut(c.Value, ".")
	until, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || time.Now().Unix() > until {
		return false
	}
	return hmac.Equal([]byte(c.Value), []byte(a.sign(until)))
}
