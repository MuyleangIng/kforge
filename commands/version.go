package commands

import (
	"fmt"

	"github.com/MuyleangIng/kforge/internal/meta"
	"github.com/spf13/cobra"
)

func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show kforge version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("%s %s\n", meta.ToolName, meta.DisplayVersion())
			fmt.Println("Founded by KhmerStack · Built by Ing Muyleang")
			fmt.Println(meta.URL)
			return nil
		},
	}
}
