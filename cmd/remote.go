package cmd

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/bstee615/rrun/internal/config"
	"github.com/bstee615/rrun/internal/sshconf"
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
	Long: `Add a named remote machine. <host> is any value accepted by SSH:
a bare hostname, an alias from ~/.ssh/config, or user@host[:port].

rrun inherits all SSH config settings (HostName, User, Port, IdentityFile, etc.)
for host aliases — no need to duplicate them here.

Examples:
  rrun remote add workstation gpu-box          # SSH alias from ~/.ssh/config
  rrun remote add lambda ubuntu@1.2.3.4        # direct host
  rrun remote add dev ubuntu@1.2.3.4:2222      # with non-standard port
  rrun remote add workstation gpu-box \
    --local-path /home/you --remote-path /home/gpu-user`,
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
	Short:        "Show details for a remote, including resolved SSH config",
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

var remoteSetURLCmd = &cobra.Command{
	Use:          "set-url <name> <host>",
	Short:        "Update the host for a remote",
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE:         remoteSetURL,
}

var remoteGetURLCmd = &cobra.Command{
	Use:          "get-url <name>",
	Short:        "Print the host for a remote",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         remoteGetURL,
}

var remoteRenameCmd = &cobra.Command{
	Use:          "rename <old-name> <new-name>",
	Short:        "Rename a remote",
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE:         remoteRename,
}

var remoteSetPathCmd = &cobra.Command{
	Use:   "set-path <name>",
	Short: "Update the path mapping for a remote",
	Long: `Update the local/remote path prefix mapping for a remote.
Use --local-path and --remote-path to set the prefixes.
Omit both flags to clear the path map (revert to mirroring local path).`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         remoteSetPath,
}

func init() {
	remoteAddCmd.Flags().StringVar(&flagLocalPath, "local-path", "", "local path prefix for path mapping")
	remoteAddCmd.Flags().StringVar(&flagRemotePath, "remote-path", "", "remote path prefix for path mapping")
	remoteSetPathCmd.Flags().StringVar(&flagLocalPath, "local-path", "", "local path prefix")
	remoteSetPathCmd.Flags().StringVar(&flagRemotePath, "remote-path", "", "remote path prefix")

	remoteCmd.AddCommand(
		remoteAddCmd,
		remoteRemoveCmd,
		remoteListCmd,
		remoteShowCmd,
		remoteDefaultCmd,
		remoteSetURLCmd,
		remoteGetURLCmd,
		remoteRenameCmd,
		remoteSetPathCmd,
	)
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
		r.PathMap = config.PathMap{Local: flagLocalPath, Remote: flagRemotePath}
	}

	cfg.Remotes[name] = r
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

	// Resolve SSH config to surface inherited settings.
	sc := sshconf.Resolve(r.Host)

	fmt.Printf("name:      %s\n", name)
	fmt.Printf("host:      %s\n", r.Host)
	if sc.Hostname != r.Host {
		fmt.Printf("hostname:  %s  (from SSH config)\n", sc.Hostname)
	}
	if sc.User != "" {
		fmt.Printf("user:      %s  (from SSH config)\n", sc.User)
	}
	if sc.Port != 0 {
		fmt.Printf("port:      %d  (from SSH config)\n", sc.Port)
	}
	if sc.IdentityFile != "" {
		fmt.Printf("identity:  %s  (from SSH config)\n", sc.IdentityFile)
	}
	if r.PathMap.Local != "" {
		fmt.Printf("pathmap:   %s  →  %s\n", r.PathMap.Local, r.PathMap.Remote)
	} else {
		fmt.Printf("pathmap:   (mirror local path)\n")
	}
	if name == cfg.DefaultRemote {
		fmt.Printf("default:   yes\n")
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

func remoteSetURL(_ *cobra.Command, args []string) error {
	name, host := args[0], args[1]
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r, ok := cfg.Remotes[name]
	if !ok {
		return fmt.Errorf("remote %q not found", name)
	}
	r.Host = host
	cfg.Remotes[name] = r
	if err := config.Save(cfg); err != nil {
		return err
	}
	log.Info("Remote URL updated", "name", name, "host", host)
	return nil
}

func remoteGetURL(_ *cobra.Command, args []string) error {
	name := args[0]
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r, ok := cfg.Remotes[name]
	if !ok {
		return fmt.Errorf("remote %q not found", name)
	}
	fmt.Println(r.Host)
	return nil
}

func remoteRename(_ *cobra.Command, args []string) error {
	oldName, newName := args[0], args[1]
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r, ok := cfg.Remotes[oldName]
	if !ok {
		return fmt.Errorf("remote %q not found", oldName)
	}
	if _, exists := cfg.Remotes[newName]; exists {
		return fmt.Errorf("remote %q already exists", newName)
	}

	delete(cfg.Remotes, oldName)
	cfg.Remotes[newName] = r
	if cfg.DefaultRemote == oldName {
		cfg.DefaultRemote = newName
	}

	if err := config.Save(cfg); err != nil {
		return err
	}
	log.Info("Remote renamed", "from", oldName, "to", newName)
	return nil
}

func remoteSetPath(_ *cobra.Command, args []string) error {
	name := args[0]
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r, ok := cfg.Remotes[name]
	if !ok {
		return fmt.Errorf("remote %q not found", name)
	}

	r.PathMap = config.PathMap{Local: flagLocalPath, Remote: flagRemotePath}
	cfg.Remotes[name] = r
	if err := config.Save(cfg); err != nil {
		return err
	}

	if flagLocalPath == "" && flagRemotePath == "" {
		log.Info("Path map cleared (will mirror local path)", "remote", name)
	} else {
		log.Info("Path map updated", "remote", name,
			"local", flagLocalPath, "remote_path", flagRemotePath)
	}
	return nil
}
