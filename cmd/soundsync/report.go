package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/report"
)

// checkReportPath rejects a bad report path before any login or quota.
func checkReportPath(path string) error {
	if _, err := report.FormatOf(path); err != nil {
		return fmt.Errorf(i18n.T("--report %q: use a .csv or .json file name"), path)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf(i18n.T("--report %q already exists; choose another file name (Soundsync never overwrites files)"), path)
	}
	return nil
}

// saveReport writes the run's report; it refuses to replace an existing file.
func saveReport(out io.Writer, path string, jobs []*copier.Job) error {
	rows := report.Rows(jobs)
	err := report.Save(path, rows)
	switch {
	case errors.Is(err, fs.ErrExist):
		return fmt.Errorf(i18n.T("report not saved: %q already exists"), path)
	case err != nil:
		return fmt.Errorf(i18n.T("report not saved: %w"), err)
	}
	fmt.Fprintf(out, i18n.T("Report saved to %s (%d rows).\n"), path, len(rows))
	return nil
}
