// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	heartbeatcmd "github.com/gardener/gardener/extensions/pkg/controller/heartbeat/cmd"
	"github.com/gardener/gardener/extensions/pkg/util"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	openstackinstall "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackcmd "github.com/gardener/gardener-extension-provider-openstack/pkg/cmd"
	openstackbackupbucket "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/backupbucket"
	openstackbackupentry "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/backupentry"
	openstackbastion "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/bastion"
	openstackcontrolplane "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/controlplane"
	openstackdnsrecord "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/healthcheck"
	openstackinfrastructure "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure"
	openstackworker "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackseedprovider "github.com/gardener/gardener-extension-provider-openstack/pkg/webhook/seedprovider"
)

// NewControllerManagerCommand creates a new command for running a OpenStack provider controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	var (
		generalOpts = &controllercmd.GeneralOptions{}
		restOpts    = &controllercmd.RESTOptions{}
		mgrOpts     = &controllercmd.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        controllercmd.LeaderElectionNameID(openstack.Name),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:       443,
			WebhookCertDir:          "/tmp/gardener-extensions-cert",
			MetricsBindAddress:      ":8080",
			HealthBindAddress:       ":8081",
		}
		configFileOpts = &openstackcmd.ConfigOptions{}

		// options for the backupbucket controller
		backupBucketCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the backupentry controller
		backupEntryCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the bastion controller
		bastionCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the health care controller
		healthCheckCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the heartbeat controller
		heartbeatCtrlOpts = &heartbeatcmd.Options{
			ExtensionName:        openstack.Name,
			RenewIntervalSeconds: 30,
			Namespace:            os.Getenv("LEADER_ELECTION_NAMESPACE"),
		}

		// options for the infrastructure controller
		infraCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		reconcileOpts = &controllercmd.ReconcilerOptions{}

		// options for the control plane controller
		controlPlaneCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsrecord controller
		dnsRecordCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the worker controller
		workerCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the webhook server
		webhookServerOptions = &webhookcmd.ServerOptions{
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}

		controllerSwitches = openstackcmd.ControllerSwitchOptions()
		webhookSwitches    = openstackcmd.WebhookSwitchOptions()
		webhookOptions     = webhookcmd.NewAddToManagerOptions(
			openstack.Name,
			genericactuator.ShootWebhooksResourceName,
			genericactuator.ShootWebhookNamespaceSelector(openstack.Type),
			webhookServerOptions,
			webhookSwitches,
		)

		aggOption = controllercmd.NewOptionAggregator(
			generalOpts,
			restOpts,
			mgrOpts,
			controllercmd.PrefixOption("backupbucket-", backupBucketCtrlOpts),
			controllercmd.PrefixOption("backupentry-", backupEntryCtrlOpts),
			controllercmd.PrefixOption("bastion-", bastionCtrlOpts),
			controllercmd.PrefixOption("controlplane-", controlPlaneCtrlOpts),
			controllercmd.PrefixOption("dnsrecord-", dnsRecordCtrlOpts),
			controllercmd.PrefixOption("infrastructure-", infraCtrlOpts),
			controllercmd.PrefixOption("worker-", workerCtrlOpts),
			controllercmd.PrefixOption("healthcheck-", healthCheckCtrlOpts),
			controllercmd.PrefixOption("heartbeat-", heartbeatCtrlOpts),
			controllerSwitches,
			configFileOpts,
			reconcileOpts,
			webhookOptions,
		)
	)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s-controller-manager", openstack.Name),

		RunE: func(_ *cobra.Command, _ []string) error {
			verflag.PrintAndExitIfRequested()

			if err := aggOption.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			if err := heartbeatCtrlOpts.Validate(); err != nil {
				return err
			}

			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, restOpts.Completed().Config)

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				return fmt.Errorf("could not instantiate manager: %w", err)
			}

			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := druidcorev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := openstackinstall.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := vpaautoscalingv1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := machinev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := monitoringv1.AddToScheme(mgr.GetScheme()); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}

			log := mgr.GetLogger()
			gardenCluster, err := getGardenCluster(log)
			if err != nil {
				return err
			}
			log.Info("Adding garden cluster to manager")
			if err := mgr.Add(gardenCluster); err != nil {
				return fmt.Errorf("failed adding garden cluster to manager: %w", err)
			}

			log.Info("Adding controllers to manager")
			configFileOpts.Completed().ApplyETCDStorage(&openstackseedprovider.DefaultAddOptions.ETCDStorage)
			configFileOpts.Completed().ApplyHealthCheckConfig(&healthcheck.DefaultAddOptions.HealthCheckConfig)
			configFileOpts.Completed().ApplyBastionConfig(&openstackbastion.DefaultAddOptions.BastionConfig)
			healthCheckCtrlOpts.Completed().Apply(&healthcheck.DefaultAddOptions.Controller)
			heartbeatCtrlOpts.Completed().Apply(&heartbeat.DefaultAddOptions)
			backupBucketCtrlOpts.Completed().Apply(&openstackbackupbucket.DefaultAddOptions.Controller)
			backupEntryCtrlOpts.Completed().Apply(&openstackbackupentry.DefaultAddOptions.Controller)
			bastionCtrlOpts.Completed().Apply(&openstackbastion.DefaultAddOptions.Controller)
			controlPlaneCtrlOpts.Completed().Apply(&openstackcontrolplane.DefaultAddOptions.Controller)
			dnsRecordCtrlOpts.Completed().Apply(&openstackdnsrecord.DefaultAddOptions.Controller)
			infraCtrlOpts.Completed().Apply(&openstackinfrastructure.DefaultAddOptions.Controller)
			reconcileOpts.Completed().Apply(&openstackinfrastructure.DefaultAddOptions.IgnoreOperationAnnotation, &openstackinfrastructure.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&openstackcontrolplane.DefaultAddOptions.IgnoreOperationAnnotation, &openstackcontrolplane.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&openstackworker.DefaultAddOptions.IgnoreOperationAnnotation, &openstackworker.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&openstackbastion.DefaultAddOptions.IgnoreOperationAnnotation, &openstackbastion.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&openstackbackupbucket.DefaultAddOptions.IgnoreOperationAnnotation, &openstackbackupbucket.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&openstackbackupentry.DefaultAddOptions.IgnoreOperationAnnotation, &openstackbackupentry.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&openstackdnsrecord.DefaultAddOptions.IgnoreOperationAnnotation, &openstackdnsrecord.DefaultAddOptions.ExtensionClass)
			workerCtrlOpts.Completed().Apply(&openstackworker.DefaultAddOptions.Controller)
			openstackworker.DefaultAddOptions.GardenCluster = gardenCluster
			openstackworker.DefaultAddOptions.AutonomousShootCluster = generalOpts.Completed().AutonomousShootCluster

			if _, err := webhookOptions.Completed().AddToManager(ctx, mgr, nil, generalOpts.Completed().AutonomousShootCluster); err != nil {
				return fmt.Errorf("could not add webhooks to manager: %w", err)
			}
			openstackcontrolplane.DefaultAddOptions.WebhookServerNamespace = webhookOptions.Server.Namespace

			if err := controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
				return fmt.Errorf("could not add controllers to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
				return fmt.Errorf("could not add readycheck for informers: %w", err)
			}

			if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
				return fmt.Errorf("could not add health check to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
				return fmt.Errorf("could not add ready check for webhook server to manager: %w", err)
			}

			// TODO (georgibaltiev): Remove after the release of version 1.50.0
			if reconcileOpts.ExtensionClass != "garden" {
				log.Info("Adding migration runnables")
				if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
					return purgeMachineControllerManagerRBACResources(ctx, mgr.GetClient(), log)
				})); err != nil {
					return fmt.Errorf("error adding migrations: %w", err)
				}
			}

			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("error running manager: %w", err)
			}

			return nil
		},
	}

	verflag.AddFlags(cmd.Flags())
	aggOption.AddFlags(cmd.Flags())

	return cmd
}

func getGardenCluster(log logr.Logger) (cluster.Cluster, error) {
	log.Info("Getting rest config for garden")
	gardenRESTConfig, err := kubernetes.RESTConfigFromKubeconfigFile(os.Getenv("GARDEN_KUBECONFIG"), kubernetes.AuthTokenFile)
	if err != nil {
		return nil, err
	}

	log.Info("Setting up cluster object for garden")
	gardenCluster, err := cluster.New(gardenRESTConfig, func(opts *cluster.Options) {
		opts.Scheme = kubernetes.GardenScheme
		opts.Logger = log
	})
	if err != nil {
		return nil, fmt.Errorf("failed creating garden cluster object: %w", err)
	}

	return gardenCluster, nil
}
