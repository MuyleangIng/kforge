package commands

import (
	"fmt"
	"strings"

	"github.com/MuyleangIng/kforge/builder"
	"github.com/spf13/cobra"
)

// BuilderCmd returns the `kforge builder` subcommand group.
func BuilderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builder",
		Short: "Manage builder instances",
	}
	cmd.AddCommand(
		builderCreateCmd(),
		builderLsCmd(),
		builderUseCmd(),
		builderRmCmd(),
	)
	return cmd
}

func builderCreateCmd() *cobra.Command {
	var name, driver, endpoint string
	var platforms []string
	var use, bootstrap bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new builder instance",
		Example: `  kforge builder create --name mybuilder
  kforge builder create --name mybuilder --driver docker-container
  kforge builder create --name remote --driver remote --endpoint tcp://buildkitd:1234`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				name = "mybuilder"
			}
			if driver == "" {
				driver = "docker-container"
			}
			cfg := builder.Config{
				Name:      name,
				Driver:    driver,
				Endpoint:  endpoint,
				Platforms: platforms,
			}
			if _, err := builder.Create(cmd.Context(), cfg, use, bootstrap); err != nil {
				return err
			}
			if err := builder.Save(cfg); err != nil {
				return fmt.Errorf("failed to save builder: %w", err)
			}
			if use {
				if err := builder.SetCurrent(name); err != nil {
					return err
				}
			}
			fmt.Printf("Builder %q created (driver: %s)\n", name, driver)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&name, "name", "", "Builder name (default: mybuilder)")
	flags.StringVar(&driver, "driver", "docker-container", "Driver to use: docker-container, remote")
	flags.StringVar(&endpoint, "endpoint", "", "Driver endpoint (for remote driver)")
	flags.StringSliceVar(&platforms, "platform", nil, "Fixed platforms for this builder")
	flags.BoolVar(&use, "use", true, "Set the builder as active after creating it")
	flags.BoolVar(&bootstrap, "bootstrap", true, "Bootstrap the builder immediately")
	return cmd
}

func builderLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List builder instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgs, err := builder.List()
			if err != nil {
				return err
			}
			if len(cfgs) == 0 {
				fmt.Println("No builders found. Run: kforge builder create")
				return nil
			}
			current := builder.Current()
			fmt.Printf("%-20s %-20s %-10s %s\n", "NAME", "DRIVER", "ACTIVE", "PLATFORMS")
			for _, c := range cfgs {
				active := ""
				if c.Name == current {
					active = "*"
				}
				plats := strings.Join(c.Platforms, ",")
				if plats == "" {
					plats = "(auto)"
				}
				fmt.Printf("%-20s %-20s %-10s %s\n", c.Name, c.Driver, active, plats)
			}
			return nil
		},
	}
}

func builderUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use NAME",
		Short: "Set the active builder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			// Verify it exists
			if _, err := builder.Load(name); err != nil {
				return err
			}
			if err := builder.Use(cmd.Context(), name); err != nil {
				return err
			}
			if err := builder.SetCurrent(name); err != nil {
				return err
			}
			fmt.Printf("Now using builder %q\n", name)
			return nil
		},
	}
}

func builderRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm NAME",
		Short: "Remove a builder instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := builder.RemoveBuildx(cmd.Context(), name); err != nil {
				return err
			}
			if err := builder.Remove(name); err != nil {
				return fmt.Errorf("failed to remove builder %q: %w", name, err)
			}
			fmt.Printf("Builder %q removed\n", name)
			return nil
		},
	}
}
