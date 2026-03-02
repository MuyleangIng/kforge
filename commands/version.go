package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "v0.1.0"

func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show kforge version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("kforge %s\n", Version)
			fmt.Println("Founded by KhmerStack · Built by Ing Muyleang")
			fmt.Println("https://github.com/MuyleangIng/kforge")
			return nil
		},
	}
}
