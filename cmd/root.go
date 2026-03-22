package cmd

import "upwind-cli/internal/app"

func Execute() error {
	rootCmd, err := app.NewRootCmd()
	if err != nil {
		return err
	}

	return rootCmd.Execute()
}
