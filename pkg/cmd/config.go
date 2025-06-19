// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	healthcheckconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	"github.com/spf13/pflag"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	configloader "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config/loader"
)

// ConfigOptions are command line options that can be set for config.ControllerConfiguration.
type ConfigOptions struct {
	// Kubeconfig is the path to a kubeconfig.
	ConfigFilePath string

	config *Config
}

// Config is a completed controller configuration.
type Config struct {
	// Config is the controller configuration.
	Config *config.ControllerConfiguration
}

func (c *ConfigOptions) buildConfig() (*config.ControllerConfiguration, error) {
	if len(c.ConfigFilePath) == 0 {
		return nil, fmt.Errorf("config file path not set")
	}
	return configloader.LoadFromFile(c.ConfigFilePath)
}

// Complete implements RESTCompleter.Complete.
func (c *ConfigOptions) Complete() error {
	config, err := c.buildConfig()
	if err != nil {
		return err
	}

	c.config = &Config{config}
	return nil
}

// Completed returns the completed Config. Only call this if `Complete` was successful.
func (c *ConfigOptions) Completed() *Config {
	return c.config
}

// AddFlags implements Flagger.AddFlags.
func (c *ConfigOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFilePath, "config-file", "", "path to the controller manager configuration file")
}

// Apply sets the values of this Config in the given config.ControllerConfiguration.
func (c *Config) Apply(cfg *config.ControllerConfiguration) {
	*cfg = *c.Config
}

// ApplyETCDMain sets the given etcd storage configuration to that of this Config.
func (c *Config) ApplyETCDMain(etcdStorage *config.ETCDStorage) {
	*etcdStorage = c.Config.ETCD.Main
}

// ApplyETCDEvents sets the given etcd storage configuration to that of this Config.
func (c *Config) ApplyETCDEvents(etcdStorage *config.ETCDStorage) {
	*etcdStorage = c.Config.ETCD.Events
}

// ApplyETCDBackup sets the given etcd backup configuration to that of this Config.
func (c *Config) ApplyETCDBackup(etcdBackup *config.ETCDBackup) {
	*etcdBackup = c.Config.ETCD.Backup
}

// Options initializes empty config.ControllerConfiguration, applies the set values and returns it.
func (c *Config) Options() config.ControllerConfiguration {
	var cfg config.ControllerConfiguration
	c.Apply(&cfg)
	return cfg
}

// ApplyHealthCheckConfig applies the HealthCheckConfig to the config
func (c *Config) ApplyHealthCheckConfig(config *healthcheckconfigv1alpha1.HealthCheckConfig) {
	if c.Config.HealthCheckConfig != nil {
		*config = *c.Config.HealthCheckConfig
	}
}

// ApplyBastionConfig applies the BastionConfig to the config
// Deprecated: Configuring the bastion will be done via CloudProfile in future
func (c *Config) ApplyBastionConfig(config *config.BastionConfig) {
	if c.Config.BastionConfig != nil {
		*config = *c.Config.BastionConfig
	}
}
