package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mreiferson/go-options"
)

func mainFlagSet() *flag.FlagSet {
	flagSet := flag.NewFlagSet("oauth2_proxy", flag.ExitOnError)

	emailDomains := StringArray{}
	whitelistDomains := StringArray{}
	upstreams := StringArray{}
	skipAuthRegex := StringArray{}
	googleGroups := StringArray{}
	gitlabGroups := StringArray{}
	githubTeams := StringArray{}

	flagSet.String("http-address", "127.0.0.1:4180", "[http://]<addr>:<port> or unix://<path> to listen on for HTTP clients")
	flagSet.String("https-address", ":443", "<addr>:<port> to listen on for HTTPS clients")
	flagSet.Bool("force-https", false, "redirect http requests to https")
	flagSet.String("tls-cert-file", "", "path to certificate file")
	flagSet.String("tls-key-file", "", "path to private key file")
	flagSet.String("redirect-url", "", "the OAuth Redirect URL. e.g.: \"https://internalapp.yourcompany.com/oauth2/callback\"")
	flagSet.Var(&upstreams, "upstream", "the http url(s) of the upstream endpoint or file:// paths for static files. Routing is based on the path")
	flagSet.Bool("set-xauthrequest", false, "set X-Auth-Request-User and X-Auth-Request-Email response headers (useful in Nginx auth_request mode)")
	flagSet.Bool("pass-user-headers", true, "pass X-Forwarded-User and X-Forwarded-Email information to upstream")
	flagSet.Bool("pass-basic-auth", true, "pass HTTP Basic Auth header to upstream")
	flagSet.String("basic-auth-password", "", "the password to set when passing the HTTP Basic Auth header")
	flagSet.Bool("pass-access-token", false, "pass OAuth access_token to upstream via X-Forwarded-Access-Token header")
	flagSet.Bool("pass-host-header", true, "pass the request Host Header to upstream")
	flagSet.Var(&skipAuthRegex, "skip-auth-regex", "bypass authentication for requests with paths that match (may be given multiple times)")
	flagSet.Bool("skip-auth-strip-headers", true, "strip upstream request http headers that are normally set by this proxy, also for requests allowed by --skip-auth-regex")
	flagSet.Bool("skip-provider-button", false, "will skip sign-in-page to directly reach the next step: oauth/start")
	flagSet.Bool("skip-auth-preflight", false, "will skip authentication for OPTIONS requests")
	flagSet.Bool("ssl-insecure-skip-verify", false, "skip validation of certificates presented when using HTTPS")
	flagSet.Duration("flush-interval", 0, "period between response flushing when streaming responses (disabled by default)")

	flagSet.Var(&emailDomains, "email-domain", "authenticate emails with the specified domain (may be given multiple times). Use * to authenticate any email")
	flagSet.Var(&whitelistDomains, "whitelist-domain", "allowed domain for redirection after authentication, leading '.' allows subdomains (may be given multiple times)")
	flagSet.String("azure-tenant", "common", "go to a tenant-specific or common (tenant-independent) endpoint.")
	flagSet.String("bitbucket-team", "", "restrict logins to members of this team")
	flagSet.String("github-org", "", "restrict logins to members of this organisation")
	flagSet.Var(&githubTeams, "github-team", "restrict logins to members of this team (slug) (may be given multiple times)")
	flagSet.Var(&gitlabGroups, "gitlab-group", "restrict logins to members of this group (full path) (may be given multiple times)")
	flagSet.Var(&googleGroups, "google-group", "restrict logins to members of this google group (may be given multiple times)")
	flagSet.String("google-admin-email", "", "the google admin to impersonate for api calls")
	flagSet.String("google-service-account-json", "", "the path to the service account json credentials")
	flagSet.String("client-id", "", "the OAuth Client ID: e.g.: \"123456.apps.googleusercontent.com\"")
	flagSet.String("client-secret", "", "the OAuth Client Secret")
	flagSet.String("authenticated-emails-file", "", "authenticate against emails via file (one per line)")
	flagSet.String("htpasswd-file", "", "additionally authenticate against a htpasswd file. Entries must be created with \"htpasswd -s\" for SHA encryption or \"htpasswd -B\" for bcrypt encryption")
	flagSet.Bool("display-htpasswd-form", true, "display username / password login form if an htpasswd file is provided")
	flagSet.String("custom-templates-dir", "", "path to custom html templates")
	flagSet.String("banner", "", "custom sign-in banner text/html. Use \"-\" to disable default banner.")
	flagSet.String("footer", "", "custom footer text/html. Use \"-\" to disable default footer.")
	flagSet.String("proxy-prefix", "/oauth2", "the url root path that this proxy should be nested under (e.g. /<oauth2>/sign_in)")
	flagSet.Bool("proxy-websockets", true, "enables WebSocket proxying")

	flagSet.String("cookie-name", "_oauth2_proxy", "the name of the cookie that the oauth_proxy creates")
	flagSet.String("cookie-secret", "", "the seed string for secure cookies (optionally base64 encoded)")
	flagSet.String("cookie-domain", "", "an optional cookie domain (e.g. '.yourcompany.com')")
	flagSet.String("cookie-path", "/", "url path under which cookie applies (e.g. '/poc/')")
	flagSet.Duration("cookie-expire", time.Duration(168)*time.Hour, "expire timeframe for cookie")
	flagSet.Duration("cookie-refresh", time.Duration(0), "refresh the cookie after this duration; 0 to disable")
	flagSet.Bool("cookie-secure", true, "set secure (HTTPS) cookie flag")
	flagSet.Bool("cookie-httponly", true, "set HttpOnly cookie flag")
	flagSet.String("cookie-samesite", "", "set SameSite cookie attribute (lax, strict, none, or \"\")")

	flagSet.Bool("request-logging", true, "Log requests to stdout")
	flagSet.String("request-logging-format", defaultRequestLoggingFormat, "Template for request log lines")
	flagSet.String("real-client-ip-header", "X-Real-IP", "HTTP header indicating the actual ip address of the client (blank to disable)")

	flagSet.String("provider", "google", "OAuth provider")
	flagSet.String("oidc-issuer-url", "", "OpenID Connect issuer URL (e.g. https://accounts.google.com)")
	flagSet.String("oidc-jwks-url", "", "OpenID Connect JWKS URL for token verification (e.g. https://www.googleapis.com/oauth2/v3/certs)")
	flagSet.Bool("skip-oidc-discovery", false, "Skip OIDC discovery (login-url, redeem-url and oidc-jwks-url must be configured)")
	flagSet.String("login-url", "", "Authentication endpoint")
	flagSet.String("redeem-url", "", "Token redemption endpoint")
	flagSet.String("profile-url", "", "Profile access endpoint")
	flagSet.String("resource", "", "The resource that is protected (Azure AD only)")
	flagSet.String("validate-url", "", "Access token validation endpoint")
	flagSet.String("scope", "", "OAuth scope specification")
	flagSet.String("prompt", "", "OIDC prompt (overrides approval-prompt)")
	flagSet.String("approval-prompt", "force", "OAuth approval_prompt (see also: prompt)")

	flagSet.String("signature-key", "", "GAP-Signature request signature key (algorithm:secretkey)")

	return flagSet
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flagSet := mainFlagSet()

	config := flagSet.String("config", "", "path to config file")
	showVersion := flagSet.Bool("version", false, "print version string")

	flagSet.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("oauth2_proxy v%s (built with %s)\n", VERSION, runtime.Version())
		return
	}

	opts := NewOptions()

	cfg := make(EnvOptions)
	if *config != "" {
		_, err := toml.DecodeFile(*config, &cfg)
		if err != nil {
			log.Fatalf("ERROR: failed to load config file %s - %s", *config, err)
		}
	}
	cfg.LoadEnvForStruct(opts)
	options.Resolve(opts, flagSet, cfg)

	err := opts.Validate()
	if err != nil {
		log.Printf("%s", err)
		os.Exit(1)
	}
	validator := NewValidator(opts.EmailDomains, opts.AuthenticatedEmailsFile)
	oauthproxy := NewOAuthProxy(opts, validator)

	if opts.Banner != "" {
		if opts.Banner == "-" {
			oauthproxy.SignInMessage = ""
		} else {
			oauthproxy.SignInMessage = opts.Banner
		}
	} else if len(opts.EmailDomains) != 0 && opts.AuthenticatedEmailsFile == "" {
		if len(opts.EmailDomains) > 1 {
			oauthproxy.SignInMessage = fmt.Sprintf("Authenticate using one of the following domains: %v", strings.Join(opts.EmailDomains, ", "))
		} else if opts.EmailDomains[0] != "*" {
			oauthproxy.SignInMessage = fmt.Sprintf("Authenticate using %v", opts.EmailDomains[0])
		}
	}

	if opts.HtpasswdFile != "" {
		log.Printf("using htpasswd file %s", opts.HtpasswdFile)
		oauthproxy.HtpasswdFile, err = NewHtpasswdFromFile(opts.HtpasswdFile)
		oauthproxy.DisplayHtpasswdForm = opts.DisplayHtpasswdForm
		if err != nil {
			log.Fatalf("FATAL: unable to open %s %s", opts.HtpasswdFile, err)
		}
	}

	var handler http.Handler = oauthproxy
	if opts.ForceHTTPS {
		handler = redirectToHTTPS(handler, opts.HttpsAddress)
	}
	if opts.RequestLogging {
		handler = LoggingHandler(
			os.Stdout, handler, opts.RealClientIPHeader, opts.RequestLoggingFormat,
		)
	} else {
		handler = NoLoggingHandler(handler)
	}
	s := &Server{
		Handler: handler,
		Opts:    opts,
	}
	s.ListenAndServe()
}
