# coglet

coglet is a tool for User pool of Amazon Cognito.

## Commands

### `coglet apply-users`

The `coglet apply-users` command allows you to create or update users in an Amazon Cognito user pool from a file.

```
coglet apply-users [USER_POOL_ID_OR_NAME] [USERS_FILE]
```

- `USER_POOL_ID_OR_NAME`: The ID or name of the Cognito user pool. You can specify either the pool ID (e.g., `us-east-1_abc123`) or the pool name (e.g., `MyUserPool`). If multiple pools have the same name, an error will be returned.

- `USERS_FILE`: Path to a file containing user data. Each line in the file should contain a JSON object representing a user (known as [JSONL](https://jsonlines.org/)) or CSV. Empty lines and lines starting with `#` are ignored.

#### Flags

- `--password <string>`: Set a specific password for all users. This overrides any passwords specified in the users file.

- `--random-password`: Generate random passwords for users that comply with the user pool's password policy. Cannot be used with `--password`.

- `--permanent-password`: Make passwords permanent (not requiring change on first login).

- `--send-password-reset-code`: Send password reset codes to users, allowing them to set their own passwords.

- `--filter <regex>`: Only apply users whose usernames match the specified regular expression.

- `--dry-run`: Perform a dry run without actually creating or updating users. Useful for testing.

- `--client-metadata <string>`: Set client metadata for all users. This can be provided in JSON format (`{"key1":"value1","key2":"value2"}`) or as key-value pairs (`key1=value1,key2=value2`). This metadata is passed to the Cognito service during user creation/update and can be used for custom workflows.

- `--columns <string>`: Define the column structure for CSV format. Specify a comma-separated list of column names that map to user attributes. Use `username` and `password` for those fields, and attribute names for other columns. Empty values (,,) are skipped. Example: `--columns username,password,email,email_verified,,phone_number,custom:attribute`

#### Users file format

##### JSONL

The user JSON format by line should be:

```json
{"username": "user1", "password": "optional-password", "attributes": {"email": "user1@example.com", "email_verified": true, "phone_number": "+1234567890", "custom:attribute": "value"}, "clientMetadata": {"KeyName1":"string"}}
```

expanded is:


```json
{
  "username": "user1",
  "password": "optional-password",
  "attributes": {
    "email": "user1@example.com",
    "email_verified": true,
    "phone_number": "+1234567890",
    "custom:attribute": "value"
  },
  "clientMetadata": {
    "KeyName1":"string"
  }
}
```

##### CSV

Read as CSV file by defining CSV format with `--columns` flag.

```
--columns username,password,email,email_verified,,phone_number,custom:attribute
```

```csv
user1,optional-password,user1@example.com,true,,+1234567890,value
```


#### Examples

Create or update users from a file:

```
coglet apply-users MyUserPool users.jsonl
```

Apply only users with usernames starting with "admin":

```
coglet apply-users MyUserPool users.jsonl --filter "^admin"
```

Create users with random passwords that don't require changing:

```
coglet apply-users MyUserPool users.jsonl --random-password --permanent-password
```

Send password reset codes to all users:

```
coglet apply-users MyUserPool users.jsonl --send-password-reset-code
```

Test the command without making changes:

```
coglet apply-users MyUserPool users.jsonl --dry-run
```

Apply users with additional client metadata:

```
coglet apply-users MyUserPool users.jsonl --client-metadata '{"source":"batch-import","department":"HR"}'
```

### `coglet login-as`

The `coglet login-as` command allows you to authenticate as a specific user in an Amazon Cognito user pool and obtain authentication tokens.

```
coglet login-as [USER_POOL_ID_OR_NAME] [USERNAME]
```

- `USER_POOL_ID_OR_NAME`: The ID or name of the Cognito user pool. You can specify either the pool ID (e.g., `us-east-1_abc123`) or the pool name (e.g., `MyUserPool`). If multiple pools have the same name, an error will be returned.

- `USERNAME`: The username of the user to authenticate as.

#### Flags

- `--password <string>`, `-p <string>`: Set the password for authentication. If not provided, the command will use the `COGLET_PASSWORD` environment variable.

- `--client <string>`, `-c <string>`: Specify the user pool client ID or name to use for authentication.

- `--client-metadata <string>`, `-m <string>`: Set client metadata for the authentication request. This can be provided in JSON format (`{"key1":"value1","key2":"value2"}`) or as key-value pairs (`key1=value1,key2=value2`).

#### Examples

Authenticate as a user with password provided as a flag:

```
coglet login-as MyUserPool user1 --password MyPassword123
```

Authenticate as a user with password from environment variable:

```
export COGLET_PASSWORD=MyPassword123
coglet login-as MyUserPool user1
```

Authenticate using a specific client:

```
coglet login-as MyUserPool user1 --password MyPassword123 --client MyClientApp
```

Authenticate with client metadata:

```
coglet login-as MyUserPool user1 --password MyPassword123 --client-metadata '{"device":"mobile","location":"tokyo"}'
```

## Required AWS IAM Permissions for coglet

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cognito-idp:ListUserPools",
        "cognito-idp:DescribeUserPool"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "cognito-idp:AdminGetUser",
        "cognito-idp:AdminCreateUser",
        "cognito-idp:AdminUpdateUserAttributes",
        "cognito-idp:AdminSetUserPassword",
        "cognito-idp:AdminResetUserPassword",
        "cognito-idp:ListUserPoolClients",
        "cognito-idp:DescribeUserPoolClient",
        "cognito-idp:InitiateAuth"
      ],
      "Resource": "arn:aws:cognito-idp:*:*:userpool/*"
    }
  ]
}
```

## Install

**deb:**

``` console
$ export COGLET_VERSION=X.X.X
$ curl -o coglet.deb -L https://github.com/k1LoW/coglet/releases/download/v$COGLET_VERSION/coglet_$COGLET_VERSION-1_amd64.deb
$ dpkg -i coglet.deb
```

**RPM:**

``` console
$ export COGLET_VERSION=X.X.X
$ yum install https://github.com/k1LoW/coglet/releases/download/v$COGLET_VERSION/coglet_$COGLET_VERSION-1_amd64.rpm
```

**apk:**

``` console
$ export COGLET_VERSION=X.X.X
$ curl -o coglet.apk -L https://github.com/k1LoW/coglet/releases/download/v$COGLET_VERSION/coglet_$COGLET_VERSION-1_amd64.apk
$ apk add coglet.apk
```

**homebrew tap:**

```console
$ brew install k1LoW/tap/coglet
```

**[aqua](https://aquaproj.github.io/):**

```console
$ aqua g -i k1LoW/coglet
```

**manually:**

Download binary from [releases page](https://github.com/k1LoW/coglet/releases)

**go install:**

```console
$ go install github.com/k1LoW/coglet/cmd/coglet@latest
```
