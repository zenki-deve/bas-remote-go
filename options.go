package basremote

import (
	"errors"
	"os"
	"path/filepath"
)

// Options holds client configuration.
type Options struct {
	// WorkingDir is the directory used to store downloaded engine files.
	// Defaults to <cwd>/data.
	WorkingDir string

	// ScriptName is the name of the private BAS script to connect to.
	ScriptName string

	// Login is the BAS account login with access to the script.
	Login string

	// Password is the BAS account password.
	Password string
}

// validate normalises Options and returns an error if required fields are missing.
func (o *Options) validate() error {
	if o.ScriptName == "" {
		return errors.New("Options.ScriptName must not be empty")
	}
	if o.WorkingDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		o.WorkingDir = filepath.Join(cwd, "data")
	}
	abs, err := filepath.Abs(o.WorkingDir)
	if err != nil {
		return err
	}
	o.WorkingDir = abs
	return nil
}
