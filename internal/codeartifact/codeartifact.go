// Package codeartifact fetches CodeArtifact authorization tokens.
package codeartifact

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	"github.com/aws/smithy-go"

	"github.com/pickrole/pickrole/internal/awsenv"
	"github.com/pickrole/pickrole/internal/awsfiles"
	"github.com/pickrole/pickrole/internal/i18n"
)

// ErrNoAccess means the loaded role cannot get a token for the domain. This
// is expected for roles such as ReadOnly and is shown as "no CodeArtifact"
// rather than as an error.
var ErrNoAccess = i18n.New("codeartifact.no_access")

// Token is a CodeArtifact authorization token.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// GetToken asks CodeArtifact for a token using the role credentials.
func GetToken(ctx context.Context, creds awsfiles.Credentials, region, domain, owner string) (Token, error) {
	cfg := aws.Config{
		Region:     region,
		HTTPClient: awsenv.HTTPClient(),
		Credentials: credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken),
	}
	client := codeartifact.NewFromConfig(cfg, func(o *codeartifact.Options) {
		o.BaseEndpoint = awsenv.Endpoint(awsenv.CodeArtifact)
	})
	out, err := client.GetAuthorizationToken(ctx, &codeartifact.GetAuthorizationTokenInput{
		Domain:      aws.String(domain),
		DomainOwner: aws.String(owner),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "AccessDeniedException" {
			return Token{}, ErrNoAccess
		}
		return Token{}, i18n.Wrap("codeartifact.token", err)
	}
	return Token{
		Value:     aws.ToString(out.AuthorizationToken),
		ExpiresAt: aws.ToTime(out.Expiration),
	}, nil
}
