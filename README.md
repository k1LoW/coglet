# coglet

coglet is a tool for User pool of Amazon Cognito.

## Commands

### apply-users

The `apply-users` command allows you to create or update users in an Amazon Cognito user pool from a file.

```
coglet apply-users [USER_POOL_ID_OR_NAME] [USERS_FILE]
```

#### Arguments

- `USER_POOL_ID_OR_NAME`: The ID or name of the Cognito user pool. You can specify either the pool ID (e.g., `us-east-1_abc123`) or the pool name (e.g., `MyUserPool`). If multiple pools have the same name, an error will be returned.

- `USERS_FILE`: Path to a file containing user data. Each line in the file should contain a JSON object representing a user. Empty lines and lines starting with `#` are ignored.

The user JSON format should be:

```json
{
  "username": "user1",
  "password": "optional-password",
  "attributes": {
    "email": "user1@example.com",
    "phone_number": "+1234567890",
    "custom:attribute": "value"
  }
}
```

#### Flags

- `--password <string>`: Set a specific password for all users. This overrides any passwords specified in the users file.

- `--random-password`: Generate random passwords for users that comply with the user pool's password policy. Cannot be used with `--password`.

- `--permanent-password`: Make passwords permanent (not requiring change on first login).

- `--send-password-reset-code`: Send password reset codes to users, allowing them to set their own passwords.

- `--filter <regex>`: Only apply users whose usernames match the specified regular expression.

- `--dry-run`: Perform a dry run without actually creating or updating users. Useful for testing.

#### Examples

Create or update users from a file:

```
coglet apply-users MyUserPool users.json
```

Apply only users with usernames starting with "admin":

```
coglet apply-users MyUserPool users.json --filter "^admin"
```

Create users with random passwords that don't require changing:

```
coglet apply-users MyUserPool users.json --random-password --permanent-password
```

Send password reset codes to all users:

```
coglet apply-users MyUserPool users.json --send-password-reset-code
```

Test the command without making changes:

```
coglet apply-users MyUserPool users.json --dry-run
```
