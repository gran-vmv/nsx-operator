package subnet

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	"github.com/vmware-tanzu/nsx-operator/pkg/controllers/common"
	"github.com/vmware-tanzu/nsx-operator/pkg/logger"
	servicecommon "github.com/vmware-tanzu/nsx-operator/pkg/nsx/services/common"
	"github.com/vmware-tanzu/nsx-operator/pkg/nsx/services/subnet"
	nsxutil "github.com/vmware-tanzu/nsx-operator/pkg/nsx/util"
)

var (
	log                     = &logger.Log
	ResultNormal            = common.ResultNormal
	ResultRequeue           = common.ResultRequeue
	ResultRequeueAfter5mins = common.ResultRequeueAfter5mins
	ResultRequeueAfter10sec = common.ResultRequeueAfter10sec
	MetricResTypeSubnet     = common.MetricResTypeSubnet
)

// SubnetReconciler reconciles a SubnetSet object
type SubnetReconciler struct {
	Client            client.Client
	Scheme            *apimachineryruntime.Scheme
	SubnetService     *subnet.SubnetService
	SubnetPortService servicecommon.SubnetPortServiceProvider
	VPCService        servicecommon.VPCServiceProvider
	Recorder          record.EventRecorder
	StatusUpdater     common.StatusUpdater
}

func (r *SubnetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	defer func() {
		log.Info("Finished reconciling Subnet", "Subnet", req.NamespacedName, "duration(ms)", time.Since(startTime).Milliseconds())
	}()

	r.StatusUpdater.IncreaseSyncTotal()
	subnetCR := &v1alpha1.Subnet{}

	if err := r.Client.Get(ctx, req.NamespacedName, subnetCR); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.deleteSubnetByName(req.Name, req.Namespace); err != nil {
				r.StatusUpdater.DeleteFail(req.NamespacedName, nil, err)
				return ResultRequeue, err
			}
			r.StatusUpdater.DeleteSuccess(req.NamespacedName, nil)
			return ResultNormal, nil
		}
		log.Error(err, "Unable to fetch Subnet CR", "req", req.NamespacedName)
		return ResultRequeue, err
	}
	if !subnetCR.DeletionTimestamp.IsZero() {
		r.StatusUpdater.IncreaseDeleteTotal()
		if err := r.deleteSubnetByID(string(subnetCR.GetUID())); err != nil {
			r.StatusUpdater.DeleteFail(req.NamespacedName, nil, err)
			return ResultRequeue, err
		}
		r.StatusUpdater.DeleteSuccess(req.NamespacedName, nil)
		return ResultNormal, nil
	}

	r.StatusUpdater.IncreaseUpdateTotal()

	// Spec mutation check and update if necessary
	specChanged := false
	if subnetCR.Spec.AccessMode == "" {
		subnetCR.Spec.AccessMode = v1alpha1.AccessMode(v1alpha1.AccessModePrivate)
		specChanged = true
	}

	if subnetCR.Spec.IPv4SubnetSize == 0 {
		vpcNetworkConfig := r.VPCService.GetVPCNetworkConfigByNamespace(subnetCR.Namespace)
		if vpcNetworkConfig == nil {
			err := fmt.Errorf("VPCNetworkConfig not found for Subnet CR")
			r.StatusUpdater.UpdateFail(ctx, subnetCR, err, "Failed to find VPCNetworkConfig", setSubnetReadyStatusFalse)
			return ResultRequeue, err
		}
		subnetCR.Spec.IPv4SubnetSize = vpcNetworkConfig.DefaultSubnetSize
		specChanged = true
	}
	if specChanged {
		if err := r.Client.Update(ctx, subnetCR); err != nil {
			r.StatusUpdater.UpdateFail(ctx, subnetCR, err, "Failed to update Subnet", setSubnetReadyStatusFalse)
			return ResultRequeue, err
		}
		log.Info("Updated Subnet CR", "Subnet", req.NamespacedName)
	}

	tags := r.SubnetService.GenerateSubnetNSTags(subnetCR)
	if tags == nil {
		log.Error(nil, "Failed to generate Subnet tags", "Subnet", req.NamespacedName)
		return ResultRequeue, errors.New("failed to generate Subnet tags")
	}
	// List VPC Info
	vpcInfoList := r.VPCService.ListVPCInfo(req.Namespace)
	if len(vpcInfoList) == 0 {
		log.Info("No VPC info found, requeueing", "Namespace", req.Namespace)
		return ResultRequeueAfter10sec, nil
	}
	// Create or update the subnet in NSX
	if _, err := r.SubnetService.CreateOrUpdateSubnet(subnetCR, vpcInfoList[0], tags); err != nil {
		if err != nil {
			if errors.As(err, &nsxutil.ExceedTagsError{}) {
				r.StatusUpdater.UpdateFail(ctx, subnetCR, err, "Tags limit exceeded", setSubnetReadyStatusFalse)
				return ResultNormal, nil
			}
			r.StatusUpdater.UpdateFail(ctx, subnetCR, err, "Failed to create/update Subnet", setSubnetReadyStatusFalse)
			return ResultRequeue, err
		}
	}
	// Update status
	if err := r.updateSubnetStatus(subnetCR); err != nil {
		r.StatusUpdater.UpdateFail(ctx, subnetCR, err, "Failed to update Subnet status", setSubnetReadyStatusFalse)
		return ResultRequeue, err
	}
	r.StatusUpdater.UpdateSuccess(ctx, subnetCR, setSubnetReadyStatusTrue)
	return ctrl.Result{}, nil
}

func (r *SubnetReconciler) deleteSubnetByID(subnetID string) error {
	nsxSubnets := r.SubnetService.SubnetStore.GetByIndex(servicecommon.TagScopeSubnetCRUID, subnetID)
	return r.deleteSubnets(nsxSubnets)
}

func (r *SubnetReconciler) deleteSubnets(nsxSubnets []*model.VpcSubnet) error {
	if len(nsxSubnets) == 0 {
		return nil
	}
	for _, nsxSubnet := range nsxSubnets {
		portNums := len(r.SubnetPortService.GetPortsOfSubnet(*nsxSubnet.Id))
		if portNums > 0 {
			err := fmt.Errorf("cannot delete Subnet %s, still attached by %d port(s)", *nsxSubnet.Id, portNums)
			log.Error(err, "Delete Subnet from NSX failed")
			return err
		}
		if err := r.SubnetService.DeleteSubnet(*nsxSubnet); err != nil {
			log.Error(err, "Failed to delete Subnet", "ID", *nsxSubnet.Id)
			return err
		}
		log.Info("Successfully deleted Subnet", "ID", *nsxSubnet.Id)
	}
	log.Info("Successfully cleaned Subnets", "subnetCount", len(nsxSubnets))
	return nil
}

func (r *SubnetReconciler) deleteStaleSubnets(nsxSubnets []*model.VpcSubnet) error {
	crdSubnetIDs, err := r.listSubnetIDsFromCRs(context.Background())
	if err != nil {
		log.Error(err, "Failed to list Subnet CRs")
		return err
	}
	crdSubnetIDsSet := sets.NewString(crdSubnetIDs...)
	nsxSubnetsToDelete := make([]*model.VpcSubnet, 0, len(nsxSubnets))
	for _, nsxSubnet := range nsxSubnets {
		uid := nsxutil.FindTag(nsxSubnet.Tags, servicecommon.TagScopeSubnetCRUID)
		if crdSubnetIDsSet.Has(uid) {
			log.Info("Skipping deletion, Subnet CR still exists in K8s", "ID", *nsxSubnet.Id)
			continue
		}
		nsxSubnetsToDelete = append(nsxSubnetsToDelete, nsxSubnet)
	}
	log.Info("Cleaning stale Subnets", "Count", len(nsxSubnetsToDelete))
	return r.deleteSubnets(nsxSubnetsToDelete)
}

func (r *SubnetReconciler) deleteSubnetByName(name, ns string) error {
	nsxSubnets := r.SubnetService.ListSubnetByName(ns, name)
	return r.deleteStaleSubnets(nsxSubnets)
}

func (r *SubnetReconciler) updateSubnetStatus(obj *v1alpha1.Subnet) error {
	// if the nsxSubnet is nil, GetSubnetByKey will return error: NSX subnet not found in store
	nsxSubnet, err := r.SubnetService.GetSubnetByKey(r.SubnetService.BuildSubnetID(obj))
	if err != nil {
		return fmt.Errorf("failed to get NSX Subnet from store: %v", err)
	}
	obj.Status.NetworkAddresses = obj.Status.NetworkAddresses[:0]
	obj.Status.GatewayAddresses = obj.Status.GatewayAddresses[:0]
	obj.Status.DHCPServerAddresses = obj.Status.DHCPServerAddresses[:0]
	statusList, err := r.SubnetService.GetSubnetStatus(nsxSubnet)
	if err != nil {
		return err
	}
	for _, status := range statusList {
		obj.Status.NetworkAddresses = append(obj.Status.NetworkAddresses, *status.NetworkAddress)
		obj.Status.GatewayAddresses = append(obj.Status.GatewayAddresses, *status.GatewayAddress)
		// DHCPServerAddress is only for the Subnet with DHCP enabled
		if status.DhcpServerAddress != nil {
			obj.Status.DHCPServerAddresses = append(obj.Status.DHCPServerAddresses, *status.DhcpServerAddress)
		}
	}
	return nil
}

func setSubnetReadyStatusTrue(client client.Client, ctx context.Context, obj client.Object, transitionTime metav1.Time, _ ...interface{}) {
	subnet := obj.(*v1alpha1.Subnet)
	dhcpMode := subnet.Spec.SubnetDHCPConfig.Mode
	if dhcpMode == "" {
		dhcpMode = v1alpha1.DHCPConfigMode(v1alpha1.DHCPConfigModeDeactivated)
	}
	newConditions := []v1alpha1.Condition{
		{
			Type:               v1alpha1.Ready,
			Status:             v1.ConditionTrue,
			Message:            fmt.Sprintf("NSX Subnet with %s has been successfully created/updated", dhcpMode),
			Reason:             "SubnetReady",
			LastTransitionTime: transitionTime,
		},
	}
	updateSubnetStatusConditions(client, ctx, subnet, newConditions)
}

func setSubnetReadyStatusFalse(client client.Client, ctx context.Context, obj client.Object, transitionTime metav1.Time, err error, _ ...interface{}) {
	subnet := obj.(*v1alpha1.Subnet)
	newConditions := []v1alpha1.Condition{
		{
			Type:               v1alpha1.Ready,
			Status:             v1.ConditionFalse,
			Message:            "NSX Subnet could not be created/updated",
			Reason:             "SubnetNotReady",
			LastTransitionTime: transitionTime,
		},
	}
	if err != nil {
		newConditions[0].Message = fmt.Sprintf("Error occurred while processing the Subnet CR. Please check the config and try again. Error: %v", err)
	}
	updateSubnetStatusConditions(client, ctx, subnet, newConditions)
}

func updateSubnetStatusConditions(client client.Client, ctx context.Context, subnet *v1alpha1.Subnet, newConditions []v1alpha1.Condition) {
	conditionsUpdated := false
	for i := range newConditions {
		if mergeSubnetStatusCondition(subnet, &newConditions[i]) {
			conditionsUpdated = true
		}
	}
	if conditionsUpdated {
		if err := client.Status().Update(ctx, subnet); err != nil {
			log.Error(err, "Failed to update Subnet status", "Name", subnet.Name, "Namespace", subnet.Namespace)
		} else {
			log.Info("Updated Subnet", "Name", subnet.Name, "Namespace", subnet.Namespace, "New Conditions", newConditions)
		}
	}
}

func mergeSubnetStatusCondition(subnet *v1alpha1.Subnet, newCondition *v1alpha1.Condition) bool {
	matchedCondition := getExistingConditionOfType(newCondition.Type, subnet.Status.Conditions)

	if reflect.DeepEqual(matchedCondition, newCondition) {
		log.V(2).Info("Conditions already match", "New Condition", newCondition, "Existing Condition", matchedCondition)
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

func StartSubnetController(mgr ctrl.Manager, subnetService *subnet.SubnetService, subnetPortService servicecommon.SubnetPortServiceProvider, vpcService servicecommon.VPCServiceProvider, hookServer webhook.Server) error {
	// Create the Subnet Reconciler with the necessary services and configuration
	subnetReconciler := &SubnetReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		SubnetService:     subnetService,
		SubnetPortService: subnetPortService,
		VPCService:        vpcService,
		Recorder:          mgr.GetEventRecorderFor("subnet-controller"),
	}
	subnetReconciler.StatusUpdater = common.NewStatusUpdater(subnetReconciler.Client, subnetReconciler.SubnetService.NSXConfig, subnetReconciler.Recorder, MetricResTypeSubnet, "Subnet", "Subnet")
	// Start the controller
	if err := subnetReconciler.start(mgr, hookServer); err != nil {
		log.Error(err, "Failed to create controller", "controller", "Subnet")
		return err
	}
	// Start garbage collector in a separate goroutine
	go common.GenericGarbageCollector(make(chan bool), servicecommon.GCInterval, subnetReconciler.collectGarbage)
	return nil
}

// start sets up the manager for the Subnet Reconciler
func (r *SubnetReconciler) start(mgr ctrl.Manager, hookServer webhook.Server) error {
	err := r.setupWithManager(mgr)
	if err != nil {
		return err
	}
	if hookServer != nil {
		hookServer.Register("/validate-crd-nsx-vmware-com-v1alpha1-subnet",
			&webhook.Admission{
				Handler: &SubnetValidator{
					Client:  mgr.GetClient(),
					decoder: admission.NewDecoder(mgr.GetScheme()),
				},
			})
	}
	return nil
}

// setupWithManager configures the controller to watch Subnet resources
func (r *SubnetReconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Subnet{}).
		// Watches for changes in Namespaces and triggers reconciliation
		Watches(
			&v1.Namespace{},
			&EnqueueRequestForNamespace{Client: mgr.GetClient()},
			builder.WithPredicates(PredicateFuncsNs),
		).
		// Set controller options, including max concurrent reconciles
		WithOptions(
			controller.Options{
				MaxConcurrentReconciles: common.NumReconcile(),
			}).
		Complete(r)
}

func (r *SubnetReconciler) listSubnetIDsFromCRs(ctx context.Context) ([]string, error) {
	crdSubnetList, err := listSubnet(r.Client, ctx)
	if err != nil {
		return nil, err
	}

	crdSubnetIDs := make([]string, 0, len(crdSubnetList.Items))
	for _, sr := range crdSubnetList.Items {
		crdSubnetIDs = append(crdSubnetIDs, string(sr.UID))
	}
	return crdSubnetIDs, nil
}

// collectGarbage implements the interface GarbageCollector method.
func (r *SubnetReconciler) collectGarbage(ctx context.Context) {
	startTime := time.Now()
	defer func() {
		log.Info("Subnet garbage collection completed", "duration(ms)", time.Since(startTime).Milliseconds())
	}()

	crdSubnetIDs, err := r.listSubnetIDsFromCRs(ctx)
	if err != nil {
		log.Error(err, "Failed to list Subnet CRs")
		return
	}
	crdSubnetIDsSet := sets.New[string](crdSubnetIDs...)

	subnetUIDs := r.SubnetService.ListSubnetIDsFromNSXSubnets()
	subnetIDsToDelete := subnetUIDs.Difference(crdSubnetIDsSet)
	for subnetID := range subnetIDsToDelete {
		nsxSubnets := r.SubnetService.ListSubnetCreatedBySubnet(subnetID)
		r.StatusUpdater.IncreaseDeleteTotal()

		log.Info("Subnet garbage collection, cleaning stale Subnets", "Count", len(nsxSubnets))
		if err := r.deleteSubnets(nsxSubnets); err != nil {
			log.Error(err, "Subnet garbage collection, failed to delete NSX subnet", "SubnetUID", subnetID)
			r.StatusUpdater.IncreaseDeleteFailTotal()
		} else {
			log.Info("Subnet garbage collection, successfully deleted NSX subnet", "SubnetUID", subnetID)
			r.StatusUpdater.IncreaseDeleteSuccessTotal()
		}
	}
}
