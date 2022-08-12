package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

func NewCacheInfoSubcommand(cacheImpl *config.CacheImpl) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "cache info",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(cacheImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := cacheImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(cacheImpl)
			return toCLIError(cacheImpl.GlobalImpl, a.ShowCacheDir(cacheImpl))
		},
	}

	return cmd
}

func NewCacheCleanupSubcommand(cacheImpl *config.CacheImpl) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "clean up cache directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(cacheImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := cacheImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(cacheImpl)
			return toCLIError(cacheImpl.GlobalImpl, a.CleanCacheDir(cacheImpl))
		},
	}
	return cmd
}

// NewCacheCmd returns cache subcmd
func NewCacheCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	cacheOptions := config.NewCacheOptions()
	cacheImpl := config.NewCacheImpl(globalCfg, cacheOptions)

	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache management",
	}

	cmd.AddCommand(
		NewCacheCleanupSubcommand(cacheImpl),
		NewCacheInfoSubcommand(cacheImpl),
	)

	return cmd
}
