package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"

	"github.com/CharlyCeballos/soundsync/internal/i18n"

	"github.com/spf13/cobra"
)

const shellPrompt = "soundsync> "

func newShellCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: i18n.T("Run several commands with one login per service (tokens kept in memory until exit)"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShell(a, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
}

// runShell reads commands line by line and runs them against the same app,
// so logins happen once per service. Google access is revoked by the caller
// (main) when the shell ends.
func runShell(a *app, in io.Reader, out, errOut io.Writer) error {
	fmt.Fprintln(out, i18n.T("Soundsync shell. Each service asks for a browser login on first use;"))
	fmt.Fprintln(out, i18n.T("tokens stay in memory until you type 'exit'. Type 'help' for commands."))
	checkForUpdate(a.baseCtx, out, http.DefaultClient, latestReleaseURL, version)

	// Ctrl+C while a command runs cancels that command (see command()).
	// At the prompt it only reminds the user how to leave, so the shell
	// does not die without revoking Google access.
	var busy atomic.Bool
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	defer signal.Stop(sigs)
	go func() {
		for range sigs {
			if !busy.Load() {
				fmt.Fprint(out, i18n.T("\n(type 'exit' to quit)\n")+shellPrompt)
			}
		}
	}()

	sc := bufio.NewScanner(in)
	// Confirmations inside commands read from this same scanner.
	a.readLine = func() (string, error) {
		if sc.Scan() {
			return sc.Text(), nil
		}
		if err := sc.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	for {
		fmt.Fprint(out, shellPrompt)
		if !sc.Scan() {
			fmt.Fprintln(out)
			return sc.Err() // nil on EOF (Ctrl+Z/Ctrl+D)
		}
		args, err := splitArgs(sc.Text())
		if err != nil {
			fmt.Fprintf(errOut, i18n.T("Error: %v\n"), err)
			continue
		}
		if len(args) == 0 {
			continue
		}
		switch args[0] {
		case "exit", "quit":
			return nil
		case "shell":
			fmt.Fprintln(errOut, i18n.T("Error: already in the shell"))
			continue
		}

		root := newRootCmd(a)
		root.SetArgs(args)
		root.SetIn(in)
		root.SetOut(out)
		root.SetErr(errOut)
		busy.Store(true)
		// Errors are already printed by cobra; the shell keeps going.
		_ = root.ExecuteContext(context.Background())
		busy.Store(false)
	}
}

// splitArgs splits a command line on spaces, honouring "double" and
// 'single' quotes so playlist names with spaces stay one argument.
func splitArgs(line string) ([]string, error) {
	var args []string
	var cur strings.Builder
	var quote rune
	inArg := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inArg = true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, errors.New(i18n.T("unclosed quote"))
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}
