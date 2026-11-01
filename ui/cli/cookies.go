package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// jarCookie is the on-disk representation of one cookie. Path is persisted so
// the /v1/auth-scoped refresh cookie is only sent to auth endpoints.
type jarCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HttpOnly bool      `json:"httpOnly,omitempty"`
}

type jarFile struct {
	Version int         `json:"version"`
	Cookies []jarCookie `json:"cookies"`
}

// CookieJar is a small, stdlib-only http.CookieJar implementation. Unlike
// net/http/cookiejar it can enumerate and persist every cookie, including
// path-scoped ones that are not visible from the base URL.
type CookieJar struct {
	file    string
	cookies []jarCookie
}

// NewCookieJar returns an empty jar backed by file.
func NewCookieJar(file string) *CookieJar {
	return &CookieJar{file: file}
}

// SetCookies implements http.CookieJar.
func (j *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	host := strings.ToLower(u.Hostname())
	now := time.Now()

	for _, ck := range cookies {
		domain := strings.ToLower(strings.TrimPrefix(ck.Domain, "."))
		if domain == "" {
			domain = host
		}
		path := ck.Path
		if path == "" || !strings.HasPrefix(path, "/") {
			path = "/"
		}

		if ck.MaxAge < 0 {
			j.remove(ck.Name, domain, path)
			continue
		}

		expires := ck.Expires
		if ck.MaxAge > 0 {
			expires = now.Add(time.Duration(ck.MaxAge) * time.Second)
		}

		j.upsert(jarCookie{
			Name:     ck.Name,
			Value:    ck.Value,
			Domain:   domain,
			Path:     path,
			Expires:  expires,
			Secure:   ck.Secure,
			HttpOnly: ck.HttpOnly,
		})
	}

	j.dropExpired(now)
}

// Cookies implements http.CookieJar.
func (j *CookieJar) Cookies(u *url.URL) []*http.Cookie {
	host := strings.ToLower(u.Hostname())
	reqPath := u.EscapedPath()
	if reqPath == "" {
		reqPath = "/"
	}
	https := u.Scheme == "https"
	now := time.Now()

	type matched struct {
		cookie jarCookie
	}
	var found []matched
	for _, ck := range j.cookies {
		if !domainMatch(host, ck.Domain) {
			continue
		}
		if !pathMatch(reqPath, ck.Path) {
			continue
		}
		if ck.Secure && !https {
			continue
		}
		if !ck.Expires.IsZero() && !now.Before(ck.Expires) {
			continue
		}
		found = append(found, matched{cookie: ck})
	}

	// RFC 6265: longer paths first, then earlier creation. We do not track
	// creation time; name order is a stable, deterministic tiebreaker.
	sort.SliceStable(found, func(a, b int) bool {
		if len(found[a].cookie.Path) != len(found[b].cookie.Path) {
			return len(found[a].cookie.Path) > len(found[b].cookie.Path)
		}
		return found[a].cookie.Name < found[b].cookie.Name
	})

	out := make([]*http.Cookie, 0, len(found))
	for _, item := range found {
		ck := item.cookie
		out = append(out, &http.Cookie{
			Name:     ck.Name,
			Value:    ck.Value,
			Domain:   ck.Domain,
			Path:     ck.Path,
			Expires:  ck.Expires,
			Secure:   ck.Secure,
			HttpOnly: ck.HttpOnly,
		})
	}
	return out
}

// Load reads persisted cookies. A missing file is not an error.
func (j *CookieJar) Load() error {
	data, err := os.ReadFile(j.file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var payload jarFile
	if err := json.Unmarshal(data, &payload); err != nil {
		// A corrupt session file should not brick the CLI; treat it as empty.
		return nil
	}
	j.cookies = payload.Cookies
	j.dropExpired(time.Now())
	return nil
}

// Save atomically persists all unexpired cookies with mode 0600.
func (j *CookieJar) Save() error {
	j.dropExpired(time.Now())

	if len(j.cookies) == 0 {
		if err := os.Remove(j.file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	payload := jarFile{Version: 1, Cookies: j.cookies}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	dir := filepath.Dir(j.file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0o700)

	tmp, err := os.CreateTemp(dir, ".dietctl-cookies-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, j.file); err != nil {
		return err
	}
	return os.Chmod(j.file, 0o600)
}

// Clear removes all cookies from memory and disk.
func (j *CookieJar) Clear() {
	j.cookies = nil
}

func (j *CookieJar) upsert(ck jarCookie) {
	for i := range j.cookies {
		existing := j.cookies[i]
		if existing.Name == ck.Name && existing.Domain == ck.Domain && existing.Path == ck.Path {
			j.cookies[i] = ck
			return
		}
	}
	j.cookies = append(j.cookies, ck)
}

func (j *CookieJar) remove(name, domain, path string) {
	out := j.cookies[:0]
	for _, ck := range j.cookies {
		if ck.Name == name && ck.Domain == domain && ck.Path == path {
			continue
		}
		out = append(out, ck)
	}
	j.cookies = out
}

func (j *CookieJar) dropExpired(now time.Time) {
	out := j.cookies[:0]
	for _, ck := range j.cookies {
		if !ck.Expires.IsZero() && !now.Before(ck.Expires) {
			continue
		}
		out = append(out, ck)
	}
	j.cookies = out
}

func domainMatch(host, domain string) bool {
	host = strings.ToLower(host)
	domain = strings.ToLower(strings.TrimPrefix(domain, "."))
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

func pathMatch(reqPath, cookiePath string) bool {
	if reqPath == cookiePath {
		return true
	}
	if !strings.HasPrefix(reqPath, cookiePath) {
		return false
	}
	if strings.HasSuffix(cookiePath, "/") {
		return true
	}
	return len(reqPath) > len(cookiePath) && reqPath[len(cookiePath)] == '/'
}
