package userpool

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cognito "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"go.1password.io/spg"
)

type Client struct {
	userPoolID string
	client     *cognito.Client
}

type User struct {
	Username       string            `json:"username"`
	Password       string            `json:"password,omitempty"`
	Attributes     map[string]any    `json:"attributes,omitempty"`
	ClientMetadata map[string]string `json:"clientMetadata,omitempty"`
}

type ApplyUserOption struct {
	Password              string
	RandomPassword        bool
	PermanentPassword     bool
	SendPasswordResetCode bool
}

type ApplyUserOptionFunc func(*ApplyUserOption) error

type LoginAsOption struct {
	ClientIDOrName string
}

type LoginAsOptionFunc func(*LoginAsOption) error

func WithPassword(password string) ApplyUserOptionFunc {
	return func(opt *ApplyUserOption) error {
		if opt.RandomPassword {
			return errors.New("cannot specify password with random password")
		}
		opt.Password = password
		return nil
	}
}

func WithRandomPassword() ApplyUserOptionFunc {
	return func(opt *ApplyUserOption) error {
		if opt.Password != "" {
			return errors.New("cannot specify password with random password")
		}
		opt.RandomPassword = true
		return nil
	}
}

func WithPermanentPassword() ApplyUserOptionFunc {
	return func(opt *ApplyUserOption) error {
		opt.PermanentPassword = true
		return nil
	}
}

func WithSendPasswordResetCode() ApplyUserOptionFunc {
	return func(opt *ApplyUserOption) error {
		opt.SendPasswordResetCode = true
		return nil
	}
}

func WithClientIDOrName(clientIDOrName string) LoginAsOptionFunc {
	return func(opt *LoginAsOption) error {
		opt.ClientIDOrName = clientIDOrName
		return nil
	}
}

func New(userPoolIDOrName string) (*Client, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := cognito.NewFromConfig(cfg)
	c := &Client{
		client: client,
	}
	userPoolID, err := c.detectUserPoolID(ctx, userPoolIDOrName)
	if err != nil {
		return nil, err
	}
	c.userPoolID = userPoolID
	return c, nil
}

func (c *Client) ApplyUser(ctx context.Context, user User, opts ...ApplyUserOptionFunc) error {
	if user.Username == "" {
		return errors.New("username is required")
	}
	var opt ApplyUserOption
	for _, o := range opts {
		if err := o(&opt); err != nil {
			return err
		}
	}

	ok, err := c.userExists(ctx, user.Username)
	if err != nil {
		return err
	}
	if !ok {
		// create user
		if err := c.createUser(ctx, user); err != nil {
			return err
		}
	}
	// update user attributes
	if err := c.updateUserAttributes(ctx, user); err != nil {
		return err
	}

	switch {
	case opt.Password != "":
		user.Password = opt.Password
	case opt.RandomPassword:
		p, err := c.client.DescribeUserPool(ctx, &cognito.DescribeUserPoolInput{
			UserPoolId: aws.String(c.userPoolID),
		})
		if err != nil {
			return err
		}
		password, err := generatePassword(*p.UserPool.Policies.PasswordPolicy)
		if err != nil {
			return err
		}
		user.Password = password
	}

	// update user password
	if err := c.updateUserPassword(ctx, user, opt.PermanentPassword); err != nil {
		return err
	}

	if opt.SendPasswordResetCode {
		// send reset password notification
		if _, err := c.client.AdminResetUserPassword(ctx, &cognito.AdminResetUserPasswordInput{
			UserPoolId: aws.String(c.userPoolID),
			Username:   aws.String(user.Username),
		}); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) LoginAs(ctx context.Context, user User, opts ...LoginAsOptionFunc) (*cognito.InitiateAuthOutput, error) {
	opt := LoginAsOption{}
	for _, o := range opts {
		if err := o(&opt); err != nil {
			return nil, err
		}
	}

	// list user pool clients
	out, err := c.client.ListUserPoolClients(ctx, &cognito.ListUserPoolClientsInput{
		UserPoolId: aws.String(c.userPoolID),
	})
	if err != nil {
		return nil, err
	}
	if len(out.UserPoolClients) == 0 {
		return nil, errors.New("no user pool clients found")
	}
	var (
		clientID *string
	)

	if len(out.UserPoolClients) == 1 {
		if opt.ClientIDOrName != "" {
			if *out.UserPoolClients[0].ClientId != opt.ClientIDOrName && *out.UserPoolClients[0].ClientName != opt.ClientIDOrName {
				return nil, fmt.Errorf("client not found: %s", opt.ClientIDOrName)
			}
		}
		clientID = out.UserPoolClients[0].ClientId
	} else {
		if opt.ClientIDOrName == "" {
			return nil, errors.New("client ID or name is required")
		}
		for _, c := range out.UserPoolClients {
			if *c.ClientId == opt.ClientIDOrName {
				clientID = c.ClientId
				break
			}
			if *c.ClientName == opt.ClientIDOrName {
				if clientID != nil {
					return nil, fmt.Errorf("client name is ambiguous: %s", opt.ClientIDOrName)
				}
				clientID = c.ClientId
			}
		}
		if clientID == nil {
			return nil, fmt.Errorf("client not found: %s", opt.ClientIDOrName)
		}
	}

	uc, err := c.client.DescribeUserPoolClient(ctx, &cognito.DescribeUserPoolClientInput{
		UserPoolId: aws.String(c.userPoolID),
		ClientId:   clientID,
	})
	if err != nil {
		return nil, err
	}

	input := &cognito.InitiateAuthInput{
		ClientId: clientID,
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		AuthParameters: map[string]string{
			"USERNAME":    user.Username,
			"PASSWORD":    user.Password,
			"SECRET_HASH": secretHash(*clientID, *uc.UserPoolClient.ClientSecret, user.Username),
		},
		ClientMetadata: user.ClientMetadata,
	}

	// use initiated-login
	return c.client.InitiateAuth(ctx, input)
}

func (c *Client) createUser(ctx context.Context, user User) error {
	input := &cognito.AdminCreateUserInput{
		UserPoolId:     aws.String(c.userPoolID),
		Username:       aws.String(user.Username),
		ClientMetadata: user.ClientMetadata,
	}
	if _, err := c.client.AdminCreateUser(ctx, input); err != nil {
		return err
	}
	return nil
}

func (c *Client) updateUserAttributes(ctx context.Context, user User) error {
	var userAttrs []types.AttributeType
	for key, value := range user.Attributes {
		userAttrs = append(userAttrs, types.AttributeType{
			Name:  aws.String(key),
			Value: aws.String(fmt.Sprintf("%v", value)),
		})
	}

	if _, err := c.client.AdminUpdateUserAttributes(ctx, &cognito.AdminUpdateUserAttributesInput{
		UserPoolId:     aws.String(c.userPoolID),
		Username:       aws.String(user.Username),
		UserAttributes: userAttrs,
		ClientMetadata: user.ClientMetadata,
	}); err != nil {
		return err
	}

	return nil
}

func (c *Client) userExists(ctx context.Context, username string) (bool, error) {
	if _, err := c.client.AdminGetUser(ctx, &cognito.AdminGetUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(username),
	}); err != nil {
		var userNotFound *types.UserNotFoundException
		if errors.As(err, &userNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (c *Client) updateUserPassword(ctx context.Context, user User, permanent bool) error {
	if user.Password == "" {
		return nil
	}
	_, err := c.client.AdminSetUserPassword(ctx, &cognito.AdminSetUserPasswordInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(user.Username),
		Password:   aws.String(user.Password),
		Permanent:  permanent,
	})
	return err
}

func (c *Client) detectUserPoolID(ctx context.Context, userPoolIDOrName string) (string, error) {
	var foundIDByName string
	var nextToken *string
	for {
		resp, err := c.client.ListUserPools(ctx, &cognito.ListUserPoolsInput{
			MaxResults: aws.Int32(60),
			NextToken:  nextToken,
		})
		if err != nil {
			return "", err
		}
		for _, pool := range resp.UserPools {
			if *pool.Id == userPoolIDOrName {
				return *pool.Id, nil
			}
			if *pool.Name == userPoolIDOrName {
				if foundIDByName != "" {
					return "", fmt.Errorf("user pool name is ambiguous: %s", userPoolIDOrName)
				}
				foundIDByName = *pool.Id
			}
		}
		if resp.NextToken == nil {
			break
		}
		nextToken = resp.NextToken
	}

	if foundIDByName != "" {
		return foundIDByName, nil
	}
	return "", fmt.Errorf("user pool not found: %s", userPoolIDOrName)
}

func generatePassword(policy types.PasswordPolicyType) (string, error) {
	minLen := int(*policy.MinimumLength)
	len := minLen + 8
	requireLowercase := policy.RequireLowercase
	requireNumbers := policy.RequireNumbers
	requireSymbols := policy.RequireSymbols
	requireUppercase := policy.RequireUppercase

	var allow, require spg.CTFlag
	if requireLowercase {
		allow |= spg.Lowers
		require |= spg.Lowers
	}
	if requireNumbers {
		allow |= spg.Digits
		require |= spg.Digits
	}
	if requireSymbols {
		allow |= spg.Symbols
		require |= spg.Symbols
	}
	if requireUppercase {
		allow |= spg.Uppers
		require |= spg.Uppers
	}
	r := spg.CharRecipe{
		Length:  len,
		Allow:   allow,
		Require: require,
		Exclude: spg.Ambiguous,
	}
	pass, err := r.Generate()
	if err != nil {
		return "", err
	}
	return pass.String(), nil
}

func secretHash(clientID, clientSecret, email string) string {
	h := hmac.New(sha256.New, []byte(clientSecret))
	h.Write([]byte(email + clientID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
