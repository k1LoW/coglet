package userpool

import (
	"context"
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
	Username   string         `json:"username"`
	Password   string         `json:"password,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type ApplyUserOption struct {
	Password              string
	RandomPassword        bool
	PermanentPassword     bool
	SendPasswordResetCode bool
}

type ApplyUserOptionFunc func(*ApplyUserOption) error

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

func (c *Client) createUser(ctx context.Context, user User) error {
	input := &cognito.AdminCreateUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(user.Username),
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
