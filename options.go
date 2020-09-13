package main

import (
	"crypto"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mbland/hmacauth"
	"github.com/ploxiln/oauth2_proxy/providers"
)

// Configuration Options that can be set by Command Line Flag, or Config File
type Options struct {
	ProxyPrefix     string `flag:"proxy-prefix" cfg:"proxy_prefix"`
	ProxyWebSockets bool   `flag:"proxy-websockets" cfg:"proxy_websockets"`
	HttpAddress     string `flag:"http-address" cfg:"http_address"`
	HttpsAddress    string `flag:"https-address" cfg:"https_address"`
	ForceHTTPS      bool   `flag:"force-https" cfg:"force_https"`
	RedirectURL     string `flag:"redirect-url" cfg:"redirect_url"`
	ClientID        string `flag:"client-id" cfg:"client_id" env:"OAUTH2_PROXY_CLIENT_ID"`
	ClientSecret    string `flag:"client-secret" cfg:"client_secret" env:"OAUTH2_PROXY_CLIENT_SECRET"`
	TLSCertFile     string `flag:"tls-cert-file" cfg:"tls_cert_file"`
	TLSKeyFile      string `flag:"tls-key-file" cfg:"tls_key_file"`

	AuthenticatedEmailsFile  string   `flag:"authenticated-emails-file" cfg:"authenticated_emails_file"`
	AzureTenant              string   `flag:"azure-tenant" cfg:"azure_tenant"`
	BitbucketTeam            string   `flag:"bitbucket-team" cfg:"bitbucket_team"`
	EmailDomains             []string `flag:"email-domain" cfg:"email_domains"`
	WhitelistDomains         []string `flag:"whitelist-domain" cfg:"whitelist_domains" env:"OAUTH2_PROXY_WHITELIST_DOMAINS"`
	GitHubOrg                string   `flag:"github-org" cfg:"github_org"`
	GitHubTeams              []string `flag:"github-team" cfg:"github_teams"`
	GitLabGroups             []string `flag:"gitlab-group" cfg:"gitlab_groups"`
	GoogleGroups             []string `flag:"google-group" cfg:"google_groups"`
	GoogleAdminEmail         string   `flag:"google-admin-email" cfg:"google_admin_email"`
	GoogleServiceAccountJSON string   `flag:"google-service-account-json" cfg:"google_service_account_json"`
	HtpasswdFile             string   `flag:"htpasswd-file" cfg:"htpasswd_file"`
	DisplayHtpasswdForm      bool     `flag:"display-htpasswd-form" cfg:"display_htpasswd_form"`
	CustomTemplatesDir       string   `flag:"custom-templates-dir" cfg:"custom_templates_dir"`
	Banner                   string   `flag:"banner" cfg:"banner"`
	Footer                   string   `flag:"footer" cfg:"footer"`

	CookieName     string        `flag:"cookie-name" cfg:"cookie_name" env:"OAUTH2_PROXY_COOKIE_NAME"`
	CookieSecret   string        `flag:"cookie-secret" cfg:"cookie_secret" env:"OAUTH2_PROXY_COOKIE_SECRET"`
	CookieDomain   string        `flag:"cookie-domain" cfg:"cookie_domain" env:"OAUTH2_PROXY_COOKIE_DOMAIN"`
	CookiePath     string        `flag:"cookie-path" cfg:"cookie_path" env:"OAUTH2_PROXY_COOKIE_PATH"`
	CookieExpire   time.Duration `flag:"cookie-expire" cfg:"cookie_expire" env:"OAUTH2_PROXY_COOKIE_EXPIRE"`
	CookieRefresh  time.Duration `flag:"cookie-refresh" cfg:"cookie_refresh" env:"OAUTH2_PROXY_COOKIE_REFRESH"`
	CookieSecure   bool          `flag:"cookie-secure" cfg:"cookie_secure"`
	CookieHttpOnly bool          `flag:"cookie-httponly" cfg:"cookie_httponly"`
	CookieSameSite string        `flag:"cookie-samesite" cfg:"cookie_samesite"`

	Upstreams             []string `flag:"upstream" cfg:"upstreams"`
	SkipAuthRegex         []string `flag:"skip-auth-regex" cfg:"skip_auth_regex"`
	SkipAuthStripHeaders  bool     `flag:"skip-auth-strip-headers" cfg:"skip_auth_strip_headers"`
	PassBasicAuth         bool     `flag:"pass-basic-auth" cfg:"pass_basic_auth"`
	BasicAuthPassword     string   `flag:"basic-auth-password" cfg:"basic_auth_password"`
	PassAccessToken       bool     `flag:"pass-access-token" cfg:"pass_access_token"`
	PassHostHeader        bool     `flag:"pass-host-header" cfg:"pass_host_header"`
	SkipProviderButton    bool     `flag:"skip-provider-button" cfg:"skip_provider_button"`
	PassUserHeaders       bool     `flag:"pass-user-headers" cfg:"pass_user_headers"`
	SSLInsecureSkipVerify bool     `flag:"ssl-insecure-skip-verify" cfg:"ssl_insecure_skip_verify"`
	SetXAuthRequest       bool     `flag:"set-xauthrequest" cfg:"set_xauthrequest"`
	SkipAuthPreflight     bool     `flag:"skip-auth-preflight" cfg:"skip_auth_preflight"`

	FlushInterval time.Duration `flag:"flush-interval" cfg:"flush_interval"`

	// These options allow for other providers besides Google, with
	// potential overrides.
	Provider          string `flag:"provider" cfg:"provider"`
	OIDCIssuerURL     string `flag:"oidc-issuer-url" cfg:"oidc_issuer_url"`
	OIDCJwksURL       string `flag:"oidc-jwks-url" cfg:"oidc_jwks_url"`
	SkipOIDCDiscovery bool   `flag:"skip-oidc-discovery" cfg:"skip_oidc_discovery"`
	LoginURL          string `flag:"login-url" cfg:"login_url"`
	RedeemURL         string `flag:"redeem-url" cfg:"redeem_url"`
	ProfileURL        string `flag:"profile-url" cfg:"profile_url"`
	ProtectedResource string `flag:"resource" cfg:"resource"`
	ValidateURL       string `flag:"validate-url" cfg:"validate_url"`
	Scope             string `flag:"scope" cfg:"scope"`
	Prompt            string `flag:"prompt" cfg:"prompt"`
	ApprovalPrompt    string `flag:"approval-prompt" cfg:"approval_prompt"` // Deprecated by OIDC 1.0

	RequestLogging       bool   `flag:"request-logging" cfg:"request_logging"`
	RequestLoggingFormat string `flag:"request-logging-format" cfg:"request_logging_format"`
	RealClientIPHeader   string `flag:"real-client-ip-header" cfg:"real_client_ip_header"`

	SignatureKey string `flag:"signature-key" cfg:"signature_key" env:"OAUTH2_PROXY_SIGNATURE_KEY"`

	// internal values that are set after config validation
	redirectURL   *url.URL
	proxyURLs     []*url.URL
	CompiledRegex []*regexp.Regexp
	provider      providers.Provider
	signatureData *SignatureData
}

type SignatureData struct {
	hash crypto.Hash
	key  string
}

func NewOptions() *Options {
	return &Options{
		ProxyPrefix:          "/oauth2",
		ProxyWebSockets:      true,
		HttpAddress:          "127.0.0.1:4180",
		HttpsAddress:         ":443",
		ForceHTTPS:           false,
		DisplayHtpasswdForm:  true,
		CookieName:           "_oauth2_proxy",
		CookieSecure:         true,
		CookieHttpOnly:       true,
		CookieExpire:         time.Duration(168) * time.Hour,
		CookieRefresh:        time.Duration(0),
		SetXAuthRequest:      false,
		SkipAuthPreflight:    false,
		SkipAuthStripHeaders: true,
		PassBasicAuth:        true,
		PassUserHeaders:      true,
		PassAccessToken:      false,
		PassHostHeader:       true,
		Prompt:               "", // Change to "login" when ApprovalPrompt deprecated/removed
		ApprovalPrompt:       "force",
		RequestLogging:       true,
		RequestLoggingFormat: defaultRequestLoggingFormat,
		RealClientIPHeader:   "X-Real-IP",
	}
}

func parseURL(to_parse string, urltype string, msgs []string) (*url.URL, []string) {
	parsed, err := url.Parse(to_parse)
	if err != nil {
		return nil, append(msgs, fmt.Sprintf(
			"error parsing %s-url=%q %s", urltype, to_parse, err))
	}
	return parsed, msgs
}

func (o *Options) Validate() error {
	msgs := make([]string, 0)

	if o.SSLInsecureSkipVerify {
		// TODO: Accept a certificate bundle.
		default_transport, ok := http.DefaultTransport.(*http.Transport)
		if ok {
			default_transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			msgs = append(msgs, "error setting insecure tls config: DefaultTransport is unexpected type")
		}
	}

	if o.CookieSecret == "" {
		msgs = append(msgs, "missing setting: cookie-secret")
	}
	if o.ClientID == "" {
		msgs = append(msgs, "missing setting: client-id")
	}
	if o.ClientSecret == "" {
		msgs = append(msgs, "missing setting: client-secret")
	}
	if o.AuthenticatedEmailsFile == "" && len(o.EmailDomains) == 0 && o.HtpasswdFile == "" {
		msgs = append(msgs, "missing setting for email validation: email-domain or authenticated-emails-file required."+
			"\n      use email-domain=* to authorize all email addresses")
	}

	o.redirectURL, msgs = parseURL(o.RedirectURL, "redirect", msgs)

	for _, u := range o.Upstreams {
		upstreamURL, err := url.Parse(u)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("error parsing upstream: %s", err))
		} else {
			if upstreamURL.Path == "" {
				upstreamURL.Path = "/"
			}
			o.proxyURLs = append(o.proxyURLs, upstreamURL)
		}
	}

	for _, u := range o.SkipAuthRegex {
		CompiledRegex, err := regexp.Compile(u)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("error compiling regex=%q %s", u, err))
			continue
		}
		o.CompiledRegex = append(o.CompiledRegex, CompiledRegex)
	}

	msgs = parseProviderInfo(o, msgs)

	if o.PassAccessToken || (o.CookieRefresh != time.Duration(0)) {
		valid_cookie_secret_size := false
		for _, i := range []int{16, 24, 32} {
			if len(secretBytes(o.CookieSecret)) == i {
				valid_cookie_secret_size = true
			}
		}
		var decoded bool
		if string(secretBytes(o.CookieSecret)) != o.CookieSecret {
			decoded = true
		}
		if valid_cookie_secret_size == false {
			var suffix string
			if decoded {
				suffix = fmt.Sprintf(" note: cookie secret was base64 decoded from %q", o.CookieSecret)
			}
			msgs = append(msgs, fmt.Sprintf(
				"cookie_secret must be 16, 24, or 32 bytes "+
					"to create an AES cipher when "+
					"pass_access_token == true or "+
					"cookie_refresh != 0, but is %d bytes.%s",
				len(secretBytes(o.CookieSecret)), suffix))
		}
	}

	if o.CookieRefresh >= o.CookieExpire {
		msgs = append(msgs, fmt.Sprintf(
			"cookie_refresh (%s) must be less than "+
				"cookie_expire (%s)",
			o.CookieRefresh.String(),
			o.CookieExpire.String()))
	}

	switch o.CookieSameSite {
	case "", "none", "lax", "strict":
	default:
		msgs = append(msgs, fmt.Sprintf("cookie_samesite (%s) must be one of ['', 'lax', 'strict', 'none']", o.CookieSameSite))
	}

	msgs = parseSignatureKey(o, msgs)
	msgs = validateCookieName(o, msgs)

	if o.RealClientIPHeader != "" {
		valid := false
		realClientIPHeaders := []string{
			"X-Real-IP",
			"X-Forwarded-For",
			"X-ProxyUser-IP",
		}
		for _, s := range realClientIPHeaders {
			if o.RealClientIPHeader == s {
				valid = true
			}
		}
		if !valid {
			msgs = append(msgs, fmt.Sprintf("unsupported real-client-ip-header %q", o.RealClientIPHeader))
		}
	}

	if len(msgs) != 0 {
		return fmt.Errorf("Invalid configuration:\n  %s",
			strings.Join(msgs, "\n  "))
	}
	return nil
}

func parseProviderInfo(o *Options, msgs []string) []string {
	p := &providers.ProviderData{
		Scope:          o.Scope,
		ClientID:       o.ClientID,
		ClientSecret:   o.ClientSecret,
		Prompt:         o.Prompt,
		ApprovalPrompt: o.ApprovalPrompt,
	}
	p.LoginURL, msgs = parseURL(o.LoginURL, "login", msgs)
	p.RedeemURL, msgs = parseURL(o.RedeemURL, "redeem", msgs)
	p.ProfileURL, msgs = parseURL(o.ProfileURL, "profile", msgs)
	p.ValidateURL, msgs = parseURL(o.ValidateURL, "validate", msgs)
	p.ProtectedResource, msgs = parseURL(o.ProtectedResource, "resource", msgs)

	o.provider = providers.New(o.Provider, p)
	switch p := o.provider.(type) {
	case *providers.AzureProvider:
		p.Configure(o.AzureTenant)
	case *providers.BitbucketProvider:
		p.SetTeam(o.BitbucketTeam)
	case *providers.GitHubProvider:
		p.SetOrgTeam(o.GitHubOrg, o.GitHubTeams)
	case *providers.GitLabProvider:
		p.SetGroups(o.GitLabGroups)
	case *providers.GoogleProvider:
		if len(o.GoogleGroups) > 0 || o.GoogleAdminEmail != "" || o.GoogleServiceAccountJSON != "" {
			if len(o.GoogleGroups) < 1 {
				msgs = append(msgs, "missing setting: google-group")
			}
			if o.GoogleAdminEmail == "" {
				msgs = append(msgs, "missing setting: google-admin-email")
			}
			if o.GoogleServiceAccountJSON == "" {
				msgs = append(msgs, "missing setting: google-service-account-json")
			}
		}
		if o.GoogleServiceAccountJSON != "" {
			file, err := os.Open(o.GoogleServiceAccountJSON)
			if err != nil {
				msgs = append(msgs, "invalid Google credentials file: "+o.GoogleServiceAccountJSON)
			} else {
				p.SetGroupRestriction(o.GoogleGroups, o.GoogleAdminEmail, file)
			}
		}
	case *providers.OIDCProvider:
		if o.OIDCIssuerURL == "" {
			msgs = append(msgs, "missing-setting: oidc-issuer-url")
		}
		if o.SkipOIDCDiscovery {
			if o.LoginURL == "" {
				msgs = append(msgs, "missing setting: login-url")
			}
			if o.RedeemURL == "" {
				msgs = append(msgs, "missing setting: redeem-url")
			}
			if o.OIDCJwksURL == "" {
				msgs = append(msgs, "missing setting: oidc-jwks-url")
			}
			if o.OIDCIssuerURL != "" && o.OIDCJwksURL != "" {
				p.SetVerifier(o.OIDCIssuerURL, o.OIDCJwksURL)
			}
		} else {
			if o.OIDCIssuerURL != "" {
				err := p.SetIssuerURL(o.OIDCIssuerURL)
				if err != nil {
					msgs = append(msgs, err.Error())
				}
			}
		}
	}
	return msgs
}

func parseSignatureKey(o *Options, msgs []string) []string {
	if o.SignatureKey == "" {
		return msgs
	}

	components := strings.Split(o.SignatureKey, ":")
	if len(components) != 2 {
		return append(msgs, "invalid signature hash:key spec: "+
			o.SignatureKey)
	}

	algorithm, secretKey := components[0], components[1]
	if hash, err := hmacauth.DigestNameToCryptoHash(algorithm); err != nil {
		return append(msgs, "unsupported signature hash algorithm: "+
			o.SignatureKey)
	} else {
		o.signatureData = &SignatureData{hash, secretKey}
	}
	return msgs
}

func validateCookieName(o *Options, msgs []string) []string {
	cookie := &http.Cookie{Name: o.CookieName}
	if cookie.String() == "" {
		return append(msgs, fmt.Sprintf("invalid cookie name: %q", o.CookieName))
	}
	return msgs
}

// for base64 which has had '=' padding trimmed off
func addPadding(secret string) string {
	switch len(secret) % 4 {
	case 2:
		return secret + "=="
	case 3:
		return secret + "="
	default:
		return secret
	}
}

// secretBytes attempts to base64 decode the secret, if that fails it treats the secret as binary
func secretBytes(secret string) []byte {
	b, err := base64.URLEncoding.DecodeString(addPadding(secret))
	if err == nil {
		if len(b) == 16 || len(b) == 24 || len(b) == 32 {
			return b
		}
	}
	return []byte(secret)
}
