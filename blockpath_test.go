package traefik_blockpath

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		desc    string
		regexps []string
		code    *int
		expErr  bool
	}{
		{
			desc:    "should return no error",
			regexps: []string{`^/foo/(.*)`},
			expErr:  false,
		},
		{
			desc:    "should return an error",
			regexps: []string{"*"},
			expErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cfg := &Config{
				Regex: test.regexps,
			}

			if _, err := New(context.Background(), nil, cfg, "name"); test.expErr && err == nil {
				t.Errorf("expected error on bad regexp format")
			}
		})
	}
}

func TestServeHTTP(t *testing.T) {
	customCode := http.StatusNotFound

	tests := []struct {
		desc          string
		regexps       []string
		code          *int
		includeQuery  bool
		reqPath       string
		expNextCall   bool
		expStatusCode int
	}{
		{
			desc:          "should return forbidden status",
			regexps:       []string{"/test"},
			reqPath:       "/test",
			expNextCall:   false,
			expStatusCode: http.StatusForbidden,
		},
		{
			desc:          "should return forbidden status",
			regexps:       []string{"/test", "/toto"},
			reqPath:       "/toto",
			expNextCall:   false,
			expStatusCode: http.StatusForbidden,
		},
		{
			desc:          "should return ok status",
			regexps:       []string{"/test", "/toto"},
			reqPath:       "/plop",
			expNextCall:   true,
			expStatusCode: http.StatusOK,
		},
		{
			desc:          "should return ok status",
			reqPath:       "/test",
			expNextCall:   true,
			expStatusCode: http.StatusOK,
		},
		{
			desc:          "should return forbidden status",
			regexps:       []string{`^/bar(.*)`},
			reqPath:       "/bar/foo",
			expNextCall:   false,
			expStatusCode: http.StatusForbidden,
		},
		{
			desc:          "should return forbidden status",
			regexps:       []string{`^/bar(.*)`},
			reqPath:       "/foo/bar",
			expNextCall:   true,
			expStatusCode: http.StatusOK,
		},
		{
			desc:          "custom status code (unmatched)",
			regexps:       []string{"^/api/woof"},
			code:          &customCode,
			reqPath:       "/foo/bar",
			expNextCall:   true,
			expStatusCode: http.StatusOK,
		},
		{
			desc:          "custom status code (matched)",
			regexps:       []string{"^/api/woof"},
			code:          &customCode,
			reqPath:       "/api/woof",
			expNextCall:   false,
			expStatusCode: customCode,
		},
		{
			desc:          "include query (unmatched)",
			regexps:       []string{"^/api/woof$"},
			includeQuery:  true,
			reqPath:       "/api/woof?rest_route=/batch/v1",
			expNextCall:   true,
			expStatusCode: http.StatusOK,
		},
		{
			desc:          "include query (matched)",
			regexps:       []string{".*?rest_route=/batch/v1"},
			includeQuery:  true,
			reqPath:       "/api/woof?rest_route=/batch/v1",
			expNextCall:   false,
			expStatusCode: http.StatusForbidden,
		},
		{
			desc:          "include query (? not included in test without params)",
			regexps:       []string{"^/api/woof\\?"},
			includeQuery:  true,
			reqPath:       "/api/woof?",
			expNextCall:   true,
			expStatusCode: http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cfg := &Config{
				Regex:        test.regexps,
				Code:         test.code,
				IncludeQuery: test.includeQuery,
			}

			nextCall := false
			next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				nextCall = true
			})

			handler, err := New(context.Background(), next, cfg, "blockpath")
			if err != nil {
				t.Fatal(err)
			}

			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("http://localhost%s", test.reqPath)
			req := httptest.NewRequest(http.MethodGet, url, nil)

			handler.ServeHTTP(recorder, req)

			if nextCall != test.expNextCall {
				t.Errorf("next handler should not be called")
			}

			if recorder.Result().StatusCode != test.expStatusCode {
				t.Errorf("got status code %d, want %d", recorder.Code, test.expStatusCode)
			}
		})
	}
}

func TestServeHTTPDrupalBlockPatterns(t *testing.T) {
	tests := []struct {
		desc         string
		regex        string
		includeQuery bool
		url          string
		blocked      bool
	}{
		// cron.php
		{
			desc:    "block cron.php",
			regex:   `^([/]+)cron\.php`,
			url:     "/cron.php",
			blocked: true,
		},
		{
			desc:    "block cron.php with repeated leading slashes",
			regex:   `^([/]+)cron\.php`,
			url:     "///cron.php",
			blocked: true,
		},
		{
			desc:    "do not block cronXphp",
			regex:   `^([/]+)cron\.php`,
			url:     "/cronXphp",
			blocked: false,
		},
		{
			desc:    "do not block nested cron.php",
			regex:   `^([/]+)cron\.php`,
			url:     "/foo/cron.php",
			blocked: false,
		},

		// /user path
		{
			desc:    "block user path",
			regex:   `^([/]+)user`,
			url:     "/user",
			blocked: true,
		},
		{
			desc:    "block user subpath",
			regex:   `^([/]+)user`,
			url:     "/user/login",
			blocked: true,
		},
		{
			desc:    "do not block nested user path",
			regex:   `^([/]+)user`,
			url:     "/foo/user",
			blocked: false,
		},

		// Query-string user
		{
			desc:         "block q=user",
			regex:        `[?&]q=([/]*)user`,
			includeQuery: true,
			url:          "/?q=user",
			blocked:      true,
		},
		{
			desc:         "block q=/user",
			regex:        `[?&]q=([/]*)user`,
			includeQuery: true,
			url:          "/?q=/user",
			blocked:      true,
		},
		{
			desc:         "block q=//user",
			regex:        `[?&]q=([/]*)user`,
			includeQuery: true,
			url:          "/?q=//user",
			blocked:      true,
		},
		{
			desc:         "block q=user with another parameter first",
			regex:        `[?&]q=([/]*)user`,
			includeQuery: true,
			url:          "/?foo=1&q=/user",
			blocked:      true,
		},
		{
			desc:         "do not block query when includeQuery is disabled",
			regex:        `[?&]q=([/]*)user`,
			includeQuery: false,
			url:          "/?q=/user",
			blocked:      false,
		},

		// /node/add path
		{
			desc:    "block node/add",
			regex:   `^([/]+)node([/]+)add`,
			url:     "/node/add",
			blocked: true,
		},
		{
			desc:    "block node//add",
			regex:   `^([/]+)node([/]+)add`,
			url:     "/node//add",
			blocked: true,
		},
		{
			desc:    "block node/add subpath",
			regex:   `^([/]+)node([/]+)add/article`,
			url:     "/node/add/article",
			blocked: true,
		},
		{
			desc:    "do not block nested node/add",
			regex:   `^([/]+)node([/]+)add`,
			url:     "/foo/node/add",
			blocked: false,
		},

		// Query-string node/add
		{
			desc:         "block q=node/add",
			regex:        `[?&]q=([/]*)node([/]+)add`,
			includeQuery: true,
			url:          "/?q=node/add",
			blocked:      true,
		},
		{
			desc:         "block q=/node/add",
			regex:        `[?&]q=([/]*)node([/]+)add`,
			includeQuery: true,
			url:          "/?q=/node/add",
			blocked:      true,
		},
		{
			desc:         "block q=/node//add",
			regex:        `[?&]q=([/]*)node([/]+)add`,
			includeQuery: true,
			url:          "/path/to?q=/node//add",
			blocked:      true,
		},
		{
			desc:         "block node/add with another parameter first",
			regex:        `[?&]q=([/]*)node([/]+)add`,
			includeQuery: true,
			url:          "/?foo=1&q=/node/add",
			blocked:      true,
		},
		{
			desc:         "do not block query when includeQuery is disabled",
			regex:        `[?&]q=([/]*)node([/]+)add`,
			includeQuery: false,
			url:          "/?q=/node/add",
			blocked:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cfg := &Config{
				Regex:        []string{test.regex},
				IncludeQuery: test.includeQuery,
			}

			nextCalled := false
			next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				nextCalled = true
				rw.WriteHeader(http.StatusOK)
			})

			handler, err := New(context.Background(), next, cfg, "blockpath")
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, test.url, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if test.blocked {
				if nextCalled {
					t.Error("next handler was called; request should be blocked")
				}
				if recorder.Code != http.StatusForbidden {
					t.Errorf("status = %d, want %d", recorder.Code, http.StatusForbidden)
				}
			} else {
				if !nextCalled {
					t.Error("next handler was not called; request should pass")
				}
				if recorder.Code != http.StatusOK {
					t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
				}
			}
		})
	}
}
