package api

import "testing"

// TestCheckLANToken exercises the pure token-check at LAN mode's auth gate
// table-style, per the LAN-reachable mode PRD's Testing Decisions: no token
// configured always allows (default/localhost-only mode is unaffected);
// once a token is configured, only a matching presented token is allowed.
func TestCheckLANToken(t *testing.T) {
	cases := []struct {
		name            string
		configuredToken string
		presentedToken  string
		want            bool
	}{
		{
			name:            "no token configured allows even with no header",
			configuredToken: "",
			presentedToken:  "",
			want:            true,
		},
		{
			name:            "no token configured allows any presented value",
			configuredToken: "",
			presentedToken:  "whatever",
			want:            true,
		},
		{
			name:            "token configured and header matches allows",
			configuredToken: "s3cret",
			presentedToken:  "s3cret",
			want:            true,
		},
		{
			name:            "token configured and header missing rejects",
			configuredToken: "s3cret",
			presentedToken:  "",
			want:            false,
		},
		{
			name:            "token configured and header mismatched rejects",
			configuredToken: "s3cret",
			presentedToken:  "wrong",
			want:            false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkLANToken(tc.configuredToken, tc.presentedToken)
			if got != tc.want {
				t.Errorf("checkLANToken(%q, %q) = %v, want %v", tc.configuredToken, tc.presentedToken, got, tc.want)
			}
		})
	}
}

// TestCheckLANAccess covers the gate's full decision once the access cookie
// exists (issue #181): the header still works on its own, a cookie derived
// from the configured token works on its own, and a cookie derived from any
// other token — including the one before a rotation — does not.
func TestCheckLANAccess(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		header     string
		cookie     string
		want       bool
	}{
		{name: "default mode allows with nothing presented", configured: "", want: true},
		{name: "matching header allows", configured: "s3cret", header: "s3cret", want: true},
		{name: "matching cookie allows", configured: "s3cret", cookie: accessCookieValue("s3cret"), want: true},
		{name: "nothing presented rejects", configured: "s3cret", want: false},
		{name: "wrong header and no cookie rejects", configured: "s3cret", header: "nope", want: false},
		{name: "cookie from a rotated-away token rejects", configured: "n3w", cookie: accessCookieValue("s3cret"), want: false},
		{name: "raw token as cookie rejects", configured: "s3cret", cookie: "s3cret", want: false},
		{name: "wrong header but matching cookie allows", configured: "s3cret", header: "nope", cookie: accessCookieValue("s3cret"), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkLANAccess(tc.configured, tc.header, tc.cookie); got != tc.want {
				t.Fatalf("checkLANAccess(%q, %q, %q) = %v, want %v", tc.configured, tc.header, tc.cookie, got, tc.want)
			}
		})
	}
}
