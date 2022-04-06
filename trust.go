package main

import (
	"bytes"
	"fmt"
	"log"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

func addTrust(cmd *cobra.Command) {
	var h string
	var remove bool
	trust := &cobra.Command{
		Use:           "trust IDENTITY...",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("loading config file: %w", err)
			}

			ids := cfg.Identities
			if h != "" {
				u, err := neturl.Parse("scheme://" + h)
				if err != nil {
					return fmt.Errorf("parsing URL: %w", err)
				}
				h = u.Hostname()
				log.Println("adding trusted identities for host:", h)
				ids = cfg.Hosts[h].Identities
			}
			m := map[string]struct{}{}
			for _, i := range ids {
				m[i] = struct{}{}
			}
			if remove {
				// Add trusted identities.
				for _, a := range args {
					if _, ok := m[a]; !ok {
						log.Println("trusted identity not found, will not be removed:", a)
					} else {
						log.Println("removing trusted identity:", a)
						delete(m, a)
					}
				}
			} else {
				// Add trusted identities.
				for _, a := range args {
					if _, ok := m[a]; ok {
						log.Println("already trusted identity:", a)
					} else {
						log.Println("adding trusted identity:", a)
						m[a] = struct{}{}
					}
				}
			}
			ids = make([]string, 0, len(m))
			for i := range m {
				ids = append(ids, i)
			}
			sort.Strings(ids)

			if cfg.Hosts == nil {
				cfg.Hosts = map[string]host{}
			}
			if h != "" {
				cfg.Hosts[h] = host{Identities: ids}
			} else {
				cfg.Identities = ids
			}
			if err := writeConfig(*cfg); err != nil {
				return fmt.Errorf("writing config: %v", err)
			}
			return nil
		},
	}
	trust.Flags().StringVar(&h, "host", "", "If set, scope trust to this host")
	trust.Flags().BoolVar(&remove, "rm", false, "If set, remove trusted identities")
	cmd.AddCommand(trust)
}

type config struct {
	Identities []string
	Hosts      map[string]host
}

type host struct {
	Identities []string
}

func configPath() (string, error) {
	dir := os.Getenv("SGET_CONFIG")
	if dir == "" {
		home, err := homedir.Dir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".sget")
	}
	return filepath.Join(dir, "config.toml"), nil
}

func loadConfig() (*config, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}
	f, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &config{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	var config config
	if err := toml.Unmarshal(f, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func writeConfig(c config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	return os.WriteFile(p, buf.Bytes(), 0755)
}
