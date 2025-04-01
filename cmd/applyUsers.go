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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/k1LoW/coglet/userpool"
	"github.com/k1LoW/donegroup"
	"github.com/spf13/cobra"
)

var (
	password              string
	randomPassword        bool
	permanentPassword     bool
	sendPasswordResetCode bool
	filter                string
	dryRun                bool
	verbose               bool
	cols                  string
)

var applyUsersCmd = &cobra.Command{
	Use:   "apply-users [USER_POOL_ID_OR_NAME] [USERS_FILE]",
	Short: "apply users to the user pool",
	Long:  `apply users to the user pool.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		idOrName := args[0]
		p := args[1]
		up, err := userpool.New(idOrName)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		l := 0
		opts := []userpool.ApplyUserOptionFunc{}
		if password != "" {
			opts = append(opts, userpool.WithPassword(password))
		}
		if randomPassword {
			opts = append(opts, userpool.WithRandomPassword())
		}
		if permanentPassword {
			opts = append(opts, userpool.WithPermanentPassword())
		}
		if sendPasswordResetCode {
			opts = append(opts, userpool.WithSendPasswordResetCode())
		}
		var filterRe *regexp.Regexp
		if filter != "" {
			filterRe, err = regexp.Compile(filter)
			if err != nil {
				return err
			}
		}

		if dryRun {
			slog.Info("dry-run: apply users started")
		} else {
			slog.Info("apply users started")
		}

		ctx, cancel := donegroup.WithCancel(ctx)

		applied := atomic.Int64{}
		skipped := atomic.Int64{}
		defer func() {
			if dryRun {
				slog.Info("dry-run: apply users completed", slog.Int64("total", applied.Load()), slog.Int64("skipped", skipped.Load()))
				return
			}
			slog.Info("apply users completed", slog.Int64("total", applied.Load()), slog.Int64("skipped", skipped.Load()))
		}()

		for scanner.Scan() {
			l++
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			var user userpool.User
			if cols == "" {
				// AS JSONL
				if err := json.Unmarshal([]byte(line), &user); err != nil {
					return fmt.Errorf("line %d: %w", l, err)
				}
			} else {
				// CSV
				user.Attributes = map[string]any{}
				keys := strings.Split(cols, ",")
				fields := strings.Split(line, ",")
				if len(keys) != len(fields) {
					return fmt.Errorf("line %d: invalid format", l)
				}
				for i, key := range keys {
					switch {
					case key == "username":
						user.Username = fields[i]
					case key == "password":
						user.Password = fields[i]
					case key == "":
						continue
					default:
						user.Attributes[key] = fields[i]
					}
				}
			}
			if filterRe != nil && !filterRe.MatchString(user.Username) {
				if verbose {
					slog.Info("skip user", slog.String("username", user.Username))
				}
				skipped.Add(1)
				continue
			}
			if verbose {
				slog.Info("appliying user", slog.String("username", user.Username))
			}
			if dryRun {
				applied.Add(1)
				continue
			}
			select {
			case <-ctx.Done():
				continue
			default:
			}

			func(l int) {
				donegroup.Go(ctx, func() error {
					if err := up.ApplyUser(context.WithoutCancel(ctx), user, opts...); err != nil {
						cancel()
						return fmt.Errorf("line %d: %w", l, err)
					}
					applied.Add(1)
					return nil
				})
			}(l)

		}
		if err := scanner.Err(); err != nil {
			return err
		}
		cancel()
		if err := donegroup.Wait(ctx); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(applyUsersCmd)
	applyUsersCmd.Flags().StringVarP(&password, "password", "p", "", "set password")
	applyUsersCmd.Flags().BoolVarP(&randomPassword, "random-password", "r", false, "set random password")
	applyUsersCmd.Flags().BoolVarP(&permanentPassword, "permanent-password", "P", false, "set permanent password")
	applyUsersCmd.Flags().BoolVarP(&sendPasswordResetCode, "send-password-reset-code", "s", false, "send password reset code")
	applyUsersCmd.Flags().StringVarP(&filter, "filter", "f", "", "filter apply users")
	applyUsersCmd.Flags().StringVarP(&cols, "columns", "c", "", "define columns for CSV format")
	applyUsersCmd.Flags().BoolVar(&dryRun, "dry-run", false, "dry run")
	applyUsersCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
