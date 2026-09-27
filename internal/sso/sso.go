// Package sso talks to AWS IAM Identity Center: the device-code login, the
// account and role listing, and the role credentials.
//
// The SSO token is cached in ~/.aws/sso/cache using the same format as the
// AWS CLI, so a session opened in PickRole also works for `aws --profile`
// commands that use the same sso-session, and vice versa.
package sso

import (
	"context"
	"crypto/sha1" // #nosec G505 -- names the cache file like the AWS CLI; not a security use
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	oidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"

	"github.com/pickrole/pickrole/internal/awsenv"
	"github.com/pickrole/pickrole/internal/awsfiles"
	"github.com/pickrole/pickrole/internal/fsutil"
	"github.com/pickrole/pickrole/internal/i18n"
	"github.com/pickrole/pickrole/internal/store"
)

const (
	clientName      = "pickrole"
	grantDeviceCode = "urn:ietf:params:oauth:grant-type:device_code"
	grantRefresh    = "refresh_token"
	scopeAccess     = "sso:account:access"
)

// ErrLoginRequired means there is no valid SSO session and the user has to
// authorise again in the browser.
var ErrLoginRequired = i18n.New("sso.login_required")

// Token mirrors the AWS CLI's SSO cache file.
type Token struct {
	StartURL              string `json:"startUrl"`
	Region                string `json:"region"`
	AccessToken           string `json:"accessToken"`
	ExpiresAt             string `json:"expiresAt"`
	ClientID              string `json:"clientId,omitempty"`
	ClientSecret          string `json:"clientSecret,omitempty"`
	RegistrationExpiresAt string `json:"registrationExpiresAt,omitempty"`
	RefreshToken          string `json:"refreshToken,omitempty"`
}

// Expiry parses ExpiresAt.
func (t Token) Expiry() time.Time {
	ts, _ := time.Parse(time.RFC3339, t.ExpiresAt)
	return ts
}

// Valid reports whether the access token is usable for at least one more
// minute.
func (t Token) Valid() bool {
	return t.AccessToken != "" && time.Until(t.Expiry()) > time.Minute
}

// DeviceAuth is what the user needs to authorise PickRole in the browser.
type DeviceAuth struct {
	UserCode        string    `json:"userCode"`
	VerificationURL string    `json:"verificationUrl"`
	ExpiresAt       time.Time `json:"expiresAt"`

	deviceCode string
	interval   time.Duration
}

// Client is an SSO session for one start URL.
type Client struct {
	startURL    string
	region      string
	sessionName string

	oidc *ssooidc.Client
	sso  *sso.Client

	mu    sync.Mutex
	token Token
}

// New creates a client. Neither API needs AWS credentials: the OIDC calls
// are anonymous and the SSO portal calls use the bearer access token.
func New(startURL, region, sessionName string) *Client {
	cfg := aws.Config{Region: region, HTTPClient: awsenv.HTTPClient()}
	c := &Client{
		startURL:    startURL,
		region:      region,
		sessionName: sessionName,
		oidc: ssooidc.NewFromConfig(cfg, func(o *ssooidc.Options) {
			o.BaseEndpoint = awsenv.Endpoint(awsenv.SSOOIDC)
		}),
		sso: sso.NewFromConfig(cfg, func(o *sso.Options) {
			o.BaseEndpoint = awsenv.Endpoint(awsenv.SSO)
		}),
	}
	c.token, _ = c.readCache()
	return c
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// cachePath follows the AWS CLI convention: sha1 of the sso-session name.
func (c *Client) cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(c.sessionName)) // #nosec G401 -- AWS CLI cache file name, not a security use
	return filepath.Join(home, ".aws", "sso", "cache", hex.EncodeToString(sum[:])+".json"), nil
}

func (c *Client) readCache() (Token, error) {
	path, err := c.cachePath()
	if err != nil {
		return Token{}, err
	}
	var t Token
	if _, err := fsutil.ReadJSON(path, &t); err != nil {
		return Token{}, err
	}
	if t.StartURL != c.startURL {
		return Token{}, nil // cache belongs to another SSO
	}
	return t, nil
}

func (c *Client) writeCache(t Token) error {
	path, err := c.cachePath()
	if err != nil {
		return err
	}
	return fsutil.WriteJSON(path, t, 0o600)
}

// Session returns the cached token and whether it is still valid.
func (c *Client) Session() (Token, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token, c.token.Valid()
}

// ensureClient registers PickRole as a public OIDC client, reusing a cached
// registration until it expires.
func (c *Client) ensureClient(ctx context.Context) error {
	if c.token.ClientID != "" {
		exp, _ := time.Parse(time.RFC3339, c.token.RegistrationExpiresAt)
		if time.Until(exp) > time.Hour {
			return nil
		}
	}
	out, err := c.oidc.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(clientName),
		ClientType: aws.String("public"),
		// With the sso:account:access scope the device-code flow also
		// returns a refresh token, which Refresh uses to renew silently.
		Scopes: []string{scopeAccess},
	})
	if err != nil {
		return i18n.Wrap("sso.register_client", err)
	}
	c.token.ClientID = aws.ToString(out.ClientId)
	c.token.ClientSecret = aws.ToString(out.ClientSecret)
	c.token.RegistrationExpiresAt = formatTime(time.Unix(out.ClientSecretExpiresAt, 0))
	return nil
}

// StartLogin begins the device authorisation flow. Show UserCode and open
// VerificationURL, then call WaitLogin.
func (c *Client) StartLogin(ctx context.Context) (*DeviceAuth, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureClient(ctx); err != nil {
		return nil, err
	}
	start := func() (*ssooidc.StartDeviceAuthorizationOutput, error) {
		return c.oidc.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
			ClientId:     aws.String(c.token.ClientID),
			ClientSecret: aws.String(c.token.ClientSecret),
			StartUrl:     aws.String(c.startURL),
		})
	}
	out, err := start()
	// The cached registration can stop being valid before its expiry date
	// (revoked, or from another endpoint): register again and retry once.
	var invalid *oidctypes.InvalidClientException
	if errors.As(err, &invalid) {
		c.token.ClientID = ""
		if err := c.ensureClient(ctx); err != nil {
			return nil, err
		}
		out, err = start()
	}
	if err != nil {
		return nil, i18n.Wrap("sso.start_authorization", err)
	}
	interval := time.Duration(out.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &DeviceAuth{
		UserCode:        aws.ToString(out.UserCode),
		VerificationURL: aws.ToString(out.VerificationUriComplete),
		ExpiresAt:       time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		deviceCode:      aws.ToString(out.DeviceCode),
		interval:        interval,
	}, nil
}

// WaitLogin polls until the user approves the device code in the browser,
// the code expires or ctx is cancelled.
func (c *Client) WaitLogin(ctx context.Context, auth *DeviceAuth) error {
	interval := auth.interval
	ctx, cancel := context.WithDeadline(ctx, auth.ExpiresAt)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return i18n.Wrap("sso.not_authorized", ctx.Err())
		case <-time.After(interval):
		}

		c.mu.Lock()
		out, err := c.oidc.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     aws.String(c.token.ClientID),
			ClientSecret: aws.String(c.token.ClientSecret),
			GrantType:    aws.String(grantDeviceCode),
			DeviceCode:   aws.String(auth.deviceCode),
		})
		c.mu.Unlock()

		var pending *oidctypes.AuthorizationPendingException
		var slowDown *oidctypes.SlowDownException
		switch {
		case errors.As(err, &pending):
			continue
		case errors.As(err, &slowDown):
			interval += 5 * time.Second
			continue
		case err != nil:
			return i18n.Wrap("sso.get_token", err)
		}
		return c.saveToken(out)
	}
}

// Refresh renews the access token with the refresh token, without the
// browser. It returns ErrLoginRequired when that is not possible.
func (c *Client) Refresh(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token.RefreshToken == "" || c.token.ClientID == "" {
		return ErrLoginRequired
	}
	out, err := c.oidc.CreateToken(ctx, &ssooidc.CreateTokenInput{
		ClientId:     aws.String(c.token.ClientID),
		ClientSecret: aws.String(c.token.ClientSecret),
		GrantType:    aws.String(grantRefresh),
		RefreshToken: aws.String(c.token.RefreshToken),
	})
	if err != nil {
		return ErrLoginRequired
	}
	return c.saveTokenLocked(out)
}

func (c *Client) saveToken(out *ssooidc.CreateTokenOutput) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveTokenLocked(out)
}

func (c *Client) saveTokenLocked(out *ssooidc.CreateTokenOutput) error {
	c.token.StartURL = c.startURL
	c.token.Region = c.region
	c.token.AccessToken = aws.ToString(out.AccessToken)
	c.token.ExpiresAt = formatTime(time.Now().Add(time.Duration(out.ExpiresIn) * time.Second))
	if out.RefreshToken != nil {
		c.token.RefreshToken = aws.ToString(out.RefreshToken)
	}
	return c.writeCache(c.token)
}

func (c *Client) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.token.Valid() {
		return "", ErrLoginRequired
	}
	return c.token.AccessToken, nil
}

func mapErr(err error) error {
	var unauthorized *ssotypes.UnauthorizedException
	if errors.As(err, &unauthorized) {
		return ErrLoginRequired
	}
	return err
}

// ListAccounts returns every account and its roles. Roles are fetched with
// limited parallelism so large organisations stay fast without throttling.
func (c *Client) ListAccounts(ctx context.Context) ([]store.Account, error) {
	token, err := c.accessToken()
	if err != nil {
		return nil, err
	}

	var accounts []store.Account
	p := sso.NewListAccountsPaginator(c.sso, &sso.ListAccountsInput{AccessToken: aws.String(token)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, mapErr(err)
		}
		for _, a := range page.AccountList {
			accounts = append(accounts, store.Account{
				ID:    aws.ToString(a.AccountId),
				Name:  aws.ToString(a.AccountName),
				Email: aws.ToString(a.EmailAddress),
			})
		}
	}

	const workers = 6
	sem := make(chan struct{}, workers)
	errs := make(chan error, len(accounts))
	var wg sync.WaitGroup
	for i := range accounts {
		wg.Add(1)
		sem <- struct{}{}
		go func(a *store.Account) {
			defer wg.Done()
			defer func() { <-sem }()
			roles, err := c.listRoles(ctx, token, a.ID)
			if err != nil {
				errs <- err
				return
			}
			a.Roles = roles
		}(&accounts[i])
	}
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		return nil, mapErr(err)
	}
	return accounts, nil
}

func (c *Client) listRoles(ctx context.Context, token, accountID string) ([]string, error) {
	var roles []string
	p := sso.NewListAccountRolesPaginator(c.sso, &sso.ListAccountRolesInput{
		AccessToken: aws.String(token),
		AccountId:   aws.String(accountID),
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range page.RoleList {
			roles = append(roles, aws.ToString(r.RoleName))
		}
	}
	return roles, nil
}

// RoleCredentials returns temporary credentials for accountID/role.
func (c *Client) RoleCredentials(ctx context.Context, accountID, role string) (awsfiles.Credentials, error) {
	token, err := c.accessToken()
	if err != nil {
		return awsfiles.Credentials{}, err
	}
	out, err := c.sso.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: aws.String(token),
		AccountId:   aws.String(accountID),
		RoleName:    aws.String(role),
	})
	if err != nil {
		return awsfiles.Credentials{}, mapErr(err)
	}
	rc := out.RoleCredentials
	if rc == nil {
		return awsfiles.Credentials{}, i18n.New("sso.no_credentials")
	}
	creds := awsfiles.Credentials{
		AccessKeyID:     aws.ToString(rc.AccessKeyId),
		SecretAccessKey: aws.ToString(rc.SecretAccessKey),
		SessionToken:    aws.ToString(rc.SessionToken),
		Expiration:      time.UnixMilli(rc.Expiration),
	}
	if err := creds.Validate(); err != nil {
		return awsfiles.Credentials{}, err
	}
	return creds, nil
}
