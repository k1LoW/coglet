/*
Copyright © 2025 Ken'ichiro Oyama <k1lowxb@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/k1LoW/coglet/userpool"
	"github.com/spf13/cobra"
)

var client string

var loginAsCmd = &cobra.Command{
	Use:   "login-as [USER_POOL_ID_OR_NAME] [USERNAME]",
	Short: "login as the user in the user pool",
	Long:  `login as the user in the user pool.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		idOrName := args[0]
		username := args[1]
		up, err := userpool.New(idOrName)
		if err != nil {
			return err
		}
		if password == "" {
			password = os.Getenv("COGLET_PASSWORD")
		}
		cm, err := parseClientMetadata(clientMetadata)
		if err != nil {
			return nil
		}
		user := userpool.User{
			Username:       username,
			Password:       password,
			ClientMetadata: cm,
		}
		out, err := up.LoginAs(ctx, user, userpool.WithClientIDOrName(client))
		if err != nil {
			return err
		}
		b, err := json.Marshal(out)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginAsCmd)
	loginAsCmd.Flags().StringVarP(&password, "password", "p", "", "password. if not set, use COGLET_PASSWORD env")
	loginAsCmd.Flags().StringVarP(&client, "client", "c", "", "user pool client id or name")
	loginAsCmd.Flags().StringVarP(&clientMetadata, "client-metadata", "m", "", "set client metadata")
}
