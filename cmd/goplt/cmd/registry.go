// cmd/goplt/cmd/registry.go
package cmd

import (
	"context"
	"fmt"

	"github.com/piprim/goplt"
	"github.com/spf13/cobra"
)

// registryStoreFactory returns the store to use. Tests override it to inject a
// temp-dir store; production keeps the default $XDG_CONFIG_HOME-based location.
var registryStoreFactory = goplt.DefaultRegistryStore

func newRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage registered template-collection repositories",
	}

	cmd.AddCommand(newRegistryAddCmd())
	cmd.AddCommand(newRegistryRemoveCmd())
	cmd.AddCommand(newRegistryListCmd())

	return cmd
}

func newRegistryAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <module>",
		Short: "Register a template-collection Go-module root",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runRegistryAdd(c.Context(), args[0])
		},
	}
}

func runRegistryAdd(ctx context.Context, module string) error {
	if err := goplt.ValidateRegistryModule(module); err != nil {
		return fmt.Errorf("validate module: %w", err)
	}

	if _, _, err := remoteResolverFn(ctx, module); err != nil {
		return fmt.Errorf("resolve module %q: %w", module, err)
	}

	store, err := registryStoreFactory()
	if err != nil {
		return fmt.Errorf("registry store: %w", err)
	}

	r, err := store.Load()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	r, err = r.Add(module)
	if err != nil {
		return fmt.Errorf("add: %w", err)
	}

	if err := store.Save(r); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	return nil
}

func newRegistryRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <module>",
		Short: "Unregister a template-collection Go-module root",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runRegistryRemove(args[0])
		},
	}
}

func runRegistryRemove(module string) error {
	store, err := registryStoreFactory()
	if err != nil {
		return fmt.Errorf("registry store: %w", err)
	}

	r, err := store.Load()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	r, err = r.Remove(module)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	if err := store.Save(r); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	return nil
}

func newRegistryListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered template-collection roots",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runRegistryList(c)
		},
	}
}

func runRegistryList(c *cobra.Command) error {
	store, err := registryStoreFactory()
	if err != nil {
		return fmt.Errorf("registry store: %w", err)
	}

	r, err := store.Load()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	out := c.OutOrStdout()
	for _, e := range r.Collections {
		fmt.Fprintln(out, e.Module)
	}

	return nil
}
