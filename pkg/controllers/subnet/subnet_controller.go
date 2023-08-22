package subnet

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
	v1 "k8s.io/api/core/v1"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/vmware-tanzu/nsx-operator/pkg/apis/v1alpha1"
	"github.com/vmware-tanzu/nsx-operator/pkg/controllers/common"
	commonctl "github.com/vmware-tanzu/nsx-operator/pkg/controllers/common"
	"github.com/vmware-tanzu/nsx-operator/pkg/logger"
	"github.com/vmware-tanzu/nsx-operator/pkg/metrics"
	servicecommon "github.com/vmware-tanzu/nsx-operator/pkg/nsx/services/common"
	"github.com/vmware-tanzu/nsx-operator/pkg/nsx/services/subnet"
)

var (
	log                     = logger.Log
	ResultNormal            = common.ResultNormal
	ResultRequeue           = common.ResultRequeue
	ResultRequeueAfter5mins = common.ResultRequeueAfter5mins
	MetricResTypeSubnet     = common.MetricResTypeSubnet
)

// SubnetReconciler reconciles a SubnetSet object
type SubnetReconciler struct {
	Client  client.Client
	Scheme  *apimachineryruntime.Scheme
	Service *subnet.SubnetService
}

func (r *SubnetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &v1alpha1.Subnet{}
	log.Info("reconciling subnet CR", "subnet", req.NamespacedName)
	metrics.CounterInc(r.Service.NSXConfig, metrics.ControllerSyncTotal, MetricResTypeSubnet)

	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		log.Error(err, "unable to fetch Subnet CR", "req", req.NamespacedName)
		return ResultNormal, client.IgnoreNotFound(err)
	}

	if obj.ObjectMeta.DeletionTimestamp.IsZero() {
		metrics.CounterInc(r.Service.NSXConfig, metrics.ControllerUpdateTotal, MetricResTypeSubnet)
		if !controllerutil.ContainsFinalizer(obj, servicecommon.SubnetFinalizerName) {
			controllerutil.AddFinalizer(obj, servicecommon.SubnetFinalizerName)
			if err := r.Client.Update(ctx, obj); err != nil {
				log.Error(err, "add finalizer", "subnet", req.NamespacedName)
				updateFail(r, &ctx, obj)
				return ResultRequeue, err
			}
			log.V(1).Info("added finalizer on subnet CR", "subnet", req.NamespacedName)
		}
		if obj.Spec.AccessMode == "" || obj.Spec.IPv4SubnetSize == 0 {
			vpcNetworkConfig := commonctl.ServiceMediator.GetVPCNetworkConfigByNamespace(obj.Namespace)
			if vpcNetworkConfig == nil {
				err := fmt.Errorf("operate failed: cannot get configuration for Subnet CR")
				log.Error(nil, "failed to find VPCNetworkConfig for Subnet CR", "subnet", req.NamespacedName, "namespace %s", obj.Namespace)
				updateFail(r, &ctx, obj)
				return ResultRequeue, err
			}
			if obj.Spec.AccessMode == "" {
				obj.Spec.AccessMode = v1alpha1.AccessMode(vpcNetworkConfig.DefaultSubnetAccessMode)
			}
			if obj.Spec.IPv4SubnetSize == 0 {
				obj.Spec.IPv4SubnetSize = vpcNetworkConfig.DefaultIPv4SubnetSize
			}
		}

		var tags []model.Tag
		namespace := &v1.Namespace{}
		namespacedName := types.NamespacedName{
			Name: req.Namespace,
		}
		if err := r.Client.Get(context.Background(), namespacedName, namespace); err != nil {
			log.Error(err, "unable to fetch namespace of Subnet CR", "req", req.NamespacedName)
			updateFail(r, &ctx, obj)
			return ResultRequeue, err
		}
		namespace_uid := namespace.UID
		// user create subnet CR, it is only for VM subnet, no need to add TagScopeNamespaceUID/TagScopeNamespace for pod subnet
		tags = append(tags,
			model.Tag{Scope: servicecommon.String(servicecommon.TagScopeVMNamespaceUID), Tag: servicecommon.String(string(namespace_uid))},
			model.Tag{Scope: servicecommon.String(servicecommon.TagScopeVMNamespace), Tag: servicecommon.String(req.Namespace)})

		if _, err := r.Service.CreateOrUpdateSubnet(obj, tags); err != nil {
			log.Error(err, "operate failed, would retry exponentially", "subnet", req.NamespacedName)
			updateFail(r, &ctx, obj)
			return ResultRequeue, err
		}
		if err := r.updateSubnetStatus(obj); err != nil {
			log.Error(err, "update subnet status failed, would retry exponentially", "subnet", req.NamespacedName)
			updateFail(r, &ctx, obj)
			return ResultRequeue, err
		}
		updateSuccess(r, &ctx, obj)
	} else {
		if controllerutil.ContainsFinalizer(obj, servicecommon.SubnetFinalizerName) {
			metrics.CounterInc(r.Service.NSXConfig, metrics.ControllerDeleteTotal, MetricResTypeSubnet)
			if err := r.DeleteSubnet(*obj); err != nil {
				log.Error(err, "deletion failed, would retry exponentially", "subnet", req.NamespacedName)
				deleteFail(r, &ctx, obj)
				return ResultRequeue, err
			}
			controllerutil.RemoveFinalizer(obj, servicecommon.SubnetFinalizerName)
			if err := r.Client.Update(ctx, obj); err != nil {
				log.Error(err, "deletion failed, would retry exponentially", "subnet", req.NamespacedName)
				deleteFail(r, &ctx, obj)
				return ResultRequeue, err
			}
			log.V(1).Info("removed finalizer", "subnet", req.NamespacedName)
			deleteSuccess(r, &ctx, obj)
		} else {
			log.Info("finalizers cannot be recognized", "subnet", req.NamespacedName)
		}
	}
	return ctrl.Result{}, nil
}

func (r *SubnetReconciler) DeleteSubnet(obj v1alpha1.Subnet) error {
	nsxSubnets := r.Service.SubnetStore.GetByIndex(servicecommon.TagScopeSubnetCRUID, string(obj.GetUID()))
	if len(nsxSubnets) == 0 {
		log.Info("no subnet found for subnet CR", "uid", string(obj.GetUID()))
		return nil
	}
	portNums := len(common.ServiceMediator.GetPortsOfSubnet(*nsxSubnets[0].Id))
	if portNums > 0 {
		err := errors.New("subnet still attached by port")
		log.Error(err, "", "ID", *nsxSubnets[0].Id)
		return err
	}
	return r.Service.DeleteSubnet(nsxSubnets[0])
}

func (r *SubnetReconciler) updateSubnetStatus(obj *v1alpha1.Subnet) error {
	nsxSubnet := r.Service.SubnetStore.GetByKey(string(obj.GetUID()))
	obj.Status.IPAddresses = obj.Status.IPAddresses[:0]
	statusList, err := r.Service.GetSubnetStatus(nsxSubnet)
	if err != nil {
		return err
	}
	for _, status := range statusList {
		obj.Status.IPAddresses = append(obj.Status.IPAddresses, *status.NetworkAddress)
	}
	obj.Status.NSXResourcePath = *nsxSubnet.Path
	return nil
}

func (r *SubnetReconciler) setSubnetReadyStatusTrue(ctx *context.Context, subnet *v1alpha1.Subnet) {
	newConditions := []v1alpha1.Condition{
		{
			Type:    v1alpha1.Ready,
			Status:  v1.ConditionTrue,
			Message: "NSX Subnet has been successfully created/updated",
			Reason:  "SubnetCreated",
		},
	}
	r.updateSubnetStatusConditions(ctx, subnet, newConditions)
}

func (r *SubnetReconciler) setSubnetReadyStatusFalse(ctx *context.Context, subnet *v1alpha1.Subnet) {
	newConditions := []v1alpha1.Condition{
		{
			Type:    v1alpha1.Ready,
			Status:  v1.ConditionFalse,
			Message: "NSX Subnet could not be created/updated",
			Reason:  "SubnetNotReady",
		},
	}
	r.updateSubnetStatusConditions(ctx, subnet, newConditions)
}

func (r *SubnetReconciler) updateSubnetStatusConditions(ctx *context.Context, subnet *v1alpha1.Subnet, newConditions []v1alpha1.Condition) {
	conditionsUpdated := false
	for i := range newConditions {
		if r.mergeSubnetStatusCondition(ctx, subnet, &newConditions[i]) {
			conditionsUpdated = true
		}
	}
	if conditionsUpdated {
		r.Client.Status().Update(*ctx, subnet)
		log.V(1).Info("updated Subnet", "Name", subnet.Name, "Namespace", subnet.Namespace,
			"New Conditions", newConditions)
	}
}

func (r *SubnetReconciler) mergeSubnetStatusCondition(ctx *context.Context, subnet *v1alpha1.Subnet, newCondition *v1alpha1.Condition) bool {
	matchedCondition := getExistingConditionOfType(newCondition.Type, subnet.Status.Conditions)

	if reflect.DeepEqual(matchedCondition, newCondition) {
		log.V(2).Info("conditions already match", "New Condition", newCondition, "Existing Condition", matchedCondition)
		return false
	}

	if matchedCondition != nil {
		matchedCondition.Reason = newCondition.Reason
		matchedCondition.Message = newCondition.Message
		matchedCondition.Status = newCondition.Status
	} else {
		subnet.Status.Conditions = append(subnet.Status.Conditions, *newCondition)
	}
	return true
}

func getExistingConditionOfType(conditionType v1alpha1.ConditionType, existingConditions []v1alpha1.Condition) *v1alpha1.Condition {
	for i := range existingConditions {
		if existingConditions[i].Type == conditionType {
			return &existingConditions[i]
		}
	}
	return nil
}

func updateFail(r *SubnetReconciler, c *context.Context, o *v1alpha1.Subnet) {
	r.setSubnetReadyStatusFalse(c, o)
	metrics.CounterInc(r.Service.NSXConfig, metrics.ControllerUpdateFailTotal, MetricResTypeSubnet)
}

func deleteFail(r *SubnetReconciler, c *context.Context, o *v1alpha1.Subnet) {
	r.setSubnetReadyStatusFalse(c, o)
	metrics.CounterInc(r.Service.NSXConfig, metrics.ControllerDeleteFailTotal, MetricResTypeSubnet)
}

func updateSuccess(r *SubnetReconciler, c *context.Context, o *v1alpha1.Subnet) {
	r.setSubnetReadyStatusTrue(c, o)
	metrics.CounterInc(r.Service.NSXConfig, metrics.ControllerUpdateSuccessTotal, MetricResTypeSubnet)
}

func deleteSuccess(r *SubnetReconciler, _ *context.Context, _ *v1alpha1.Subnet) {
	metrics.CounterInc(r.Service.NSXConfig, metrics.ControllerDeleteSuccessTotal, MetricResTypeSubnet)
}

func (r *SubnetReconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Subnet{}).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Suppress Delete events to avoid filtering them out in the Reconcile function
				return false
			},
		}).
		WithOptions(
			controller.Options{
				MaxConcurrentReconciles: runtime.NumCPU(),
			}).
		Complete(r)
}

func StartSubnetController(mgr ctrl.Manager, commonService servicecommon.Service) error {
	subnetReconciler := &SubnetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	subnetReconciler.Service = subnet.GetSubnetService(commonService)
	common.ServiceMediator.SubnetService = subnetReconciler.Service
	if err := subnetReconciler.Start(mgr); err != nil {
		log.Error(err, "failed to create controller", "controller", "Subnet")
		return err
	}
	return nil
}

// Start setup manager
func (r *SubnetReconciler) Start(mgr ctrl.Manager) error {
	err := r.setupWithManager(mgr)
	if err != nil {
		return err
	}
	return nil
}
