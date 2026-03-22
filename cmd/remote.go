package cmd

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"rrun/internal/config"
)

var (
	flagLocalPath  string
	flagRemotePath string
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote machines",
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <name> <host>",
	Short: "Add a remote machine",
	Long: `Add a named remote machine. <host> is any value accepted by SSH
(hostname, alias from ~/.ssh/config, or user@host).

By default, the local git root path is mirrored exactly on the remote.
Use --local-path and --remote-path to configure a prefix substitution:

  rrun remote add workstation gpu-box \
    --local-path /home/alice \
    --remote-path /home/alice

  rrun remote add lambda ubuntu@1.2.3.4 \
    --local-path /home/alice/Code \
    --remote-path /home/ubuntu/Code`,
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE:         remoteAdd,
}

var remoteRemoveCmd = &cobra.Command{
	Use:          "remove <name>",
	Aliases:      []string{"rm"},
	Short:        "Remove a remote machine",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         remoteRemove,
}

var remoteListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List configured remotes",
	SilenceUsage: true,
	RunE:         remoteList,
}

var remoteShowCmd = &cobra.Command{
	Use:          "show <name>",
	Short:        "Show details for a remote",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         remoteShow,
}

var remoteDefaultCmd = &cobra.Command{
	Use:          "default <name>",
	Short:        "Set the default remote",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         remoteDefault,
}

func init() {
	remoteAddCmd.Flags().StringVar(&flagLocalPath, "local-path", "", "local path prefix to replace in path mapping")
	remoteAddCmd.Flags().StringVar(&flagRemotePath, "remote-path", "", "remote path prefix to use in path mapping")
	remoteCmd.AddCommand(remoteAddCmd, remoteRemoveCmd, remoteListCmd, remoteShowCmd, remoteDefaultCmd)
	rootCmd.AddCommand(remoteCmd)
}

func remoteAdd(_ *cobra.Command, args []string) error {
	name, host := args[0], args[1]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	r := config.Remote{Host: host}
	if flagLocalPath != "" || flagRemotePath != "" {
		r.PathMap = config.PathMap{
			Local:  flagLocalPath,
			Remote: flagRemotePath,
		}
	}

	cfg.Remotes[name] = r

	// Auto-set as default if it's the first remote.
	if cfg.DefaultRemote == "" {
		cfg.DefaultRemote = name
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	log.Info("Remote added", "name", name, "host", host)
	if cfg.DefaultRemote == name {
		log.Info("Set as default remote", "name", name)
	}
	return nil
}

func remoteRemove(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if _, ok := cfg.Remotes[name]; !ok {
		return fmt.Errorf("remote %q not found", name)
	}

	delete(cfg.Remotes, name)
	if cfg.DefaultRemote == name {
		cfg.DefaultRemote = ""
		log.Warn("Removed the default remote; set a new one with: rrun remote default <name>")
	}

	if err := config.Save(cfg); err != nil {
		return err
	}
	log.Info("Remote removed", "name", name)
	return nil
}

func remoteList(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Remotes) == 0 {
		fmt.Println("No remotes configured. Add one with: rrun remote add <name> <host>")
		return nil
	}
	for name, r := range cfg.Remotes {
		marker := ""
		if name == cfg.DefaultRemote {
			marker = " (default)"
		}
		fmt.Printf("  %-20s %s%s\n", name, r.Host, marker)
	}
	return nil
}

func remoteShow(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r, ok := cfg.Remotes[name]
	if !ok {
		return fmt.Errorf("remote %q not found", name)
	}

	fmt.Printf("name:    %s\n", name)
	fmt.Printf("host:    %s\n", r.Host)
	if r.PathMap.Local != "" {
		fmt.Printf("pathmap: %s  →  %s\n", r.PathMap.Local, r.PathMap.Remote)
	} else {
		fmt.Printf("pathmap: (mirror local path)\n")
	}
	if name == cfg.DefaultRemote {
		fmt.Printf("default: yes\n")
	}
	return nil
}

func remoteDefault(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if _, ok := cfg.Remotes[name]; !ok {
		return fmt.Errorf("remote %q not found", name)
	}

	cfg.DefaultRemote = name
	if err := config.Save(cfg); err != nil {
		return err
	}
	log.Info("Default remote set", "name", name)
	return nil
}
