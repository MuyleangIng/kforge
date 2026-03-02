package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/MuyleangIng/kforge/commands.Version=vX.Y.Z"
var Version = "dev"

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
