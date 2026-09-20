package anthropic

import (
	"cmp"

	"github.com/dwertyfa288/CLI/vendordeps/aws/aws-sdk-go-v2/aws"
	"github.com/dwertyfa288/CLI/vendordeps/aws/smithy-go/auth/bearer"
)

func bedrockBasicAuthConfig(apiKey string, region string) aws.Config {
	return aws.Config{
		Region:                  cmp.Or(region, "us-east-1"),
		BearerAuthTokenProvider: bearer.StaticTokenProvider{Token: bearer.Token{Value: apiKey}},
	}
}
