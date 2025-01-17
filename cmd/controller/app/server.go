// Copyright (c) 2020 Red Hat, Inc.

package app

import (
	"context"
	"time"

	"github.com/open-cluster-management/multicloud-operators-foundation/cmd/controller/app/options"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/addon"
	actionv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/imageregistry/v1alpha1"
	clusterinfov1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/internal.open-cluster-management.io/v1beta1"
	inventoryv1alpha1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/inventory/v1alpha1"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/cache"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/certrotation"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterca"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterinfo"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterrole"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterset/clusterclaim"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterset/clusterdeployment"
	clustersetmapper "github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterset/clustersetmapper"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterset/syncclusterrolebinding"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/clusterset/syncrolebinding"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/gc"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/imageregistry"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/controllers/inventory"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/helpers"
	"github.com/open-cluster-management/multicloud-operators-foundation/pkg/utils"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	hiveinternalv1alpha1 "github.com/openshift/hive/apis/hiveinternal/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	clusterv1client "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterv1informers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1alaph1 "open-cluster-management.io/api/cluster/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = inventoryv1alpha1.AddToScheme(scheme)
	_ = hiveinternalv1alpha1.AddToScheme(scheme)
	_ = hivev1.AddToScheme(scheme)
	_ = clusterinfov1beta1.AddToScheme(scheme)
	_ = clusterv1.Install(scheme)
	_ = actionv1beta1.AddToScheme(scheme)
	_ = clusterv1alaph1.Install(scheme)
	_ = v1alpha1.AddToScheme(scheme)
}

func Run(o *options.ControllerRunOptions, ctx context.Context) error {

	//clusterset to cluster map
	clusterSetClusterMapper := helpers.NewClusterSetMapper()

	//clusterset to namespace resource map, like clusterdeployment, clusterpool, clusterclaim. the map value format is "<ResourceType>/<Namespace>/<Name>"
	clusterSetNamespaceMapper := helpers.NewClusterSetMapper()

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", o.KubeConfig)
	if err != nil {
		klog.Errorf("unable to get kube config: %v", err)
		return err
	}

	kubeConfig.QPS = o.QPS
	kubeConfig.Burst = o.Burst

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		klog.Errorf("unable to create kube client: %v", err)
		return err
	}

	clusterClient, err := clusterv1client.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	clusterInformers := clusterv1informers.NewSharedInformerFactory(clusterClient, 10*time.Minute)
	kubeInfomers := kubeinformers.NewSharedInformerFactory(kubeClient, 10*time.Minute)

	mgr, err := ctrl.NewManager(kubeConfig, ctrl.Options{
		Scheme:                 scheme,
		LeaderElectionID:       "foundation-controller",
		LeaderElection:         o.EnableLeaderElection,
		HealthProbeBindAddress: ":8000",
		Logger:                 ctrlruntimelog.NullLogger{},
	})
	if err != nil {
		klog.Errorf("unable to start manager: %v", err)
		return err
	}

	// add healthz/readyz check handler
	if err := mgr.AddHealthzCheck("healthz-ping", healthz.Ping); err != nil {
		klog.Errorf("unable to add healthz check handler: %v", err)
		return err
	}

	if err := mgr.AddReadyzCheck("readyz-ping", healthz.Ping); err != nil {
		klog.Errorf("unable to add readyz check handler: %v", err)
		return err
	}

	clusterSetAdminCache := cache.NewClusterSetCache(
		clusterInformers.Cluster().V1alpha1().ManagedClusterSets(),
		kubeInfomers.Rbac().V1().ClusterRoles(),
		kubeInfomers.Rbac().V1().ClusterRoleBindings(),
		utils.GetAdminResourceFromClusterRole,
	)
	clusterSetViewCache := cache.NewClusterSetCache(
		clusterInformers.Cluster().V1alpha1().ManagedClusterSets(),
		kubeInfomers.Rbac().V1().ClusterRoles(),
		kubeInfomers.Rbac().V1().ClusterRoleBindings(),
		utils.GetViewResourceFromClusterRole,
	)

	addonMgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		klog.Errorf("unable to setup addon manager: %v", err)
		return err
	}
	if o.EnableAddonDeploy {
		addonMgr.AddAgent(addon.NewAgent(kubeClient, "work-manager", o.AddonImage))
	}

	// Setup reconciler
	if o.EnableInventory {
		if err = inventory.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup inventory reconciler: %v", err)
			return err
		}
	}

	if err = clusterinfo.SetupWithManager(mgr, o.LogCertSecret); err != nil {
		klog.Errorf("unable to setup clusterInfo reconciler: %v", err)
		return err
	}

	if err = clustersetmapper.SetupWithManager(mgr, kubeClient, clusterSetClusterMapper, clusterSetNamespaceMapper); err != nil {
		klog.Errorf("unable to setup clustersetmapper reconciler: %v", err)
		return err
	}
	if err = clusterdeployment.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to setup clusterdeployment reconciler: %v", err)
		return err
	}
	if err = clusterclaim.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to setup clusterclaim reconciler: %v", err)
		return err
	}
	if err = imageregistry.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to setup imageregistry reconciler: %v", err)
		return err
	}

	clusterrolebindingSync := syncclusterrolebinding.NewReconciler(kubeClient, clusterSetAdminCache.Cache, clusterSetViewCache.Cache, clusterSetClusterMapper)

	rolebindingSync := syncrolebinding.NewReconciler(kubeClient, clusterSetAdminCache.Cache, clusterSetViewCache.Cache, clusterSetClusterMapper, clusterSetNamespaceMapper)

	if err = clusterrole.SetupWithManager(mgr, kubeClient); err != nil {
		klog.Errorf("unable to setup clusterrole reconciler: %v", err)
		return err
	}

	if err = clusterca.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to setup clusterca reconciler: %v", err)
		return err
	}
	if err = gc.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to setup gc reconciler: %v", err)
		return err
	}

	if err = certrotation.SetupWithManager(mgr, o.LogCertSecret); err != nil {
		klog.Errorf("unable to setup cert rotation reconciler: %v", err)
		return err
	}
	cleanGarbageFinalizer := gc.NewCleanGarbageFinalizer(kubeClient)

	go func() {
		<-mgr.Elected()
		go clusterInformers.Start(ctx.Done())
		go kubeInfomers.Start(ctx.Done())

		go clusterSetViewCache.Run(5 * time.Second)
		go clusterSetAdminCache.Run(5 * time.Second)
		go clusterrolebindingSync.Run(5 * time.Second)
		go rolebindingSync.Run(5 * time.Second)

		go cleanGarbageFinalizer.Run(ctx.Done())

		if o.EnableAddonDeploy {
			go addonMgr.Start(ctx)
		}
	}()

	// Start manager
	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("Controller-runtime manager exited non-zero, %v", err)
		return err
	}

	return nil
}
