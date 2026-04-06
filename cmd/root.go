package cmd

import "github.com/oneslash/upwind-cli/internal/app"

func Execute() error {
	rootCmd, err := app.NewRootCmd()
	if err != nil {
		return err
	}

	return rootCmd.Execute()
}
