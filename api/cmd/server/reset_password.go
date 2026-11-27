package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"

	"toto/api/internal/api"
	"toto/api/internal/config"
	"toto/api/internal/database"
)

const resetPasswordUsage = "usage: server reset-password <username>"

// resetPassword lets an operator with access to the container recover an account, e.g. the only administrator.
// The password is read from the terminal without echo, or from the first line of stdin when it is not a terminal,
// so that it never appears in the command line or the shell history.
func resetPassword(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return withAdminList(ctx, pool, errors.New(resetPasswordUsage))
	}
	username := strings.TrimSpace(args[0])

	password, err := readNewPassword(os.Stdin, os.Stderr)
	if err != nil {
		return err
	}
	user, err := api.ResetPassword(ctx, pool, username, password)
	if errors.Is(err, api.ErrUserNotFound) {
		return fmt.Errorf("no local account named %q", username)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Password of %s reset; its sessions were closed.\n", user.Username)
	return nil
}

// withAdminList names the local administrators in err, for an operator who forgot the username too.
func withAdminList(ctx context.Context, pool *pgxpool.Pool, err error) error {
	admins, listErr := api.LocalAdminUsernames(ctx, pool)
	if listErr != nil {
		return errors.Join(err, listErr)
	}
	if len(admins) == 0 {
		return fmt.Errorf("%w\nthere is no local administrator yet", err)
	}
	return fmt.Errorf("%w\nlocal administrators: %s", err, strings.Join(admins, ", "))
}

func readNewPassword(in *os.File, out io.Writer) (string, error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(out, "New password: ")
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	fmt.Fprint(out, "Confirm password: ")
	confirm, err := term.ReadPassword(fd)
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	if string(password) != string(confirm) {
		return "", errors.New("passwords do not match")
	}
	return string(password), nil
}
