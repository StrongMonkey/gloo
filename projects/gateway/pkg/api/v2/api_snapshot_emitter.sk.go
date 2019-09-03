// Code generated by solo-kit. DO NOT EDIT.

package v2

import (
	"sync"
	"time"

	gateway_solo_io "github.com/solo-io/gloo/projects/gateway/pkg/api/v1"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	"github.com/solo-io/go-utils/errutils"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
	"github.com/solo-io/solo-kit/pkg/errors"
)

var (
	mApiSnapshotIn     = stats.Int64("api.gateway.solo.io.v2/snap_emitter/snap_in", "The number of snapshots in", "1")
	mApiSnapshotOut    = stats.Int64("api.gateway.solo.io.v2/snap_emitter/snap_out", "The number of snapshots out", "1")
	mApiSnapshotMissed = stats.Int64("api.gateway.solo.io.v2/snap_emitter/snap_missed", "The number of snapshots missed", "1")

	apisnapshotInView = &view.View{
		Name:        "api.gateway.solo.io.v2_snap_emitter/snap_in",
		Measure:     mApiSnapshotIn,
		Description: "The number of snapshots updates coming in",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
	apisnapshotOutView = &view.View{
		Name:        "api.gateway.solo.io.v2/snap_emitter/snap_out",
		Measure:     mApiSnapshotOut,
		Description: "The number of snapshots updates going out",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
	apisnapshotMissedView = &view.View{
		Name:        "api.gateway.solo.io.v2/snap_emitter/snap_missed",
		Measure:     mApiSnapshotMissed,
		Description: "The number of snapshots updates going missed. this can happen in heavy load. missed snapshot will be re-tried after a second.",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
)

func init() {
	view.Register(apisnapshotInView, apisnapshotOutView, apisnapshotMissedView)
}

type ApiEmitter interface {
	Register() error
	VirtualService() gateway_solo_io.VirtualServiceClient
	RouteTable() gateway_solo_io.RouteTableClient
	Gateway() GatewayClient
	Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *ApiSnapshot, <-chan error, error)
}

func NewApiEmitter(virtualServiceClient gateway_solo_io.VirtualServiceClient, routeTableClient gateway_solo_io.RouteTableClient, gatewayClient GatewayClient) ApiEmitter {
	return NewApiEmitterWithEmit(virtualServiceClient, routeTableClient, gatewayClient, make(chan struct{}))
}

func NewApiEmitterWithEmit(virtualServiceClient gateway_solo_io.VirtualServiceClient, routeTableClient gateway_solo_io.RouteTableClient, gatewayClient GatewayClient, emit <-chan struct{}) ApiEmitter {
	return &apiEmitter{
		virtualService: virtualServiceClient,
		routeTable:     routeTableClient,
		gateway:        gatewayClient,
		forceEmit:      emit,
	}
}

type apiEmitter struct {
	forceEmit      <-chan struct{}
	virtualService gateway_solo_io.VirtualServiceClient
	routeTable     gateway_solo_io.RouteTableClient
	gateway        GatewayClient
}

func (c *apiEmitter) Register() error {
	if err := c.virtualService.Register(); err != nil {
		return err
	}
	if err := c.routeTable.Register(); err != nil {
		return err
	}
	if err := c.gateway.Register(); err != nil {
		return err
	}
	return nil
}

func (c *apiEmitter) VirtualService() gateway_solo_io.VirtualServiceClient {
	return c.virtualService
}

func (c *apiEmitter) RouteTable() gateway_solo_io.RouteTableClient {
	return c.routeTable
}

func (c *apiEmitter) Gateway() GatewayClient {
	return c.gateway
}

func (c *apiEmitter) Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *ApiSnapshot, <-chan error, error) {

	if len(watchNamespaces) == 0 {
		watchNamespaces = []string{""}
	}

	for _, ns := range watchNamespaces {
		if ns == "" && len(watchNamespaces) > 1 {
			return nil, nil, errors.Errorf("the \"\" namespace is used to watch all namespaces. Snapshots can either be tracked for " +
				"specific namespaces or \"\" AllNamespaces, but not both.")
		}
	}

	errs := make(chan error)
	var done sync.WaitGroup
	ctx := opts.Ctx
	/* Create channel for VirtualService */
	type virtualServiceListWithNamespace struct {
		list      gateway_solo_io.VirtualServiceList
		namespace string
	}
	virtualServiceChan := make(chan virtualServiceListWithNamespace)

	var initialVirtualServiceList gateway_solo_io.VirtualServiceList
	/* Create channel for RouteTable */
	type routeTableListWithNamespace struct {
		list      gateway_solo_io.RouteTableList
		namespace string
	}
	routeTableChan := make(chan routeTableListWithNamespace)

	var initialRouteTableList gateway_solo_io.RouteTableList
	/* Create channel for Gateway */
	type gatewayListWithNamespace struct {
		list      GatewayList
		namespace string
	}
	gatewayChan := make(chan gatewayListWithNamespace)

	var initialGatewayList GatewayList

	currentSnapshot := ApiSnapshot{}

	for _, namespace := range watchNamespaces {
		/* Setup namespaced watch for VirtualService */
		{
			virtualServices, err := c.virtualService.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "initial VirtualService list")
			}
			initialVirtualServiceList = append(initialVirtualServiceList, virtualServices...)
		}
		virtualServiceNamespacesChan, virtualServiceErrs, err := c.virtualService.Watch(namespace, opts)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "starting VirtualService watch")
		}

		done.Add(1)
		go func(namespace string) {
			defer done.Done()
			errutils.AggregateErrs(ctx, errs, virtualServiceErrs, namespace+"-virtualServices")
		}(namespace)
		/* Setup namespaced watch for RouteTable */
		{
			routeTables, err := c.routeTable.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "initial RouteTable list")
			}
			initialRouteTableList = append(initialRouteTableList, routeTables...)
		}
		routeTableNamespacesChan, routeTableErrs, err := c.routeTable.Watch(namespace, opts)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "starting RouteTable watch")
		}

		done.Add(1)
		go func(namespace string) {
			defer done.Done()
			errutils.AggregateErrs(ctx, errs, routeTableErrs, namespace+"-routeTables")
		}(namespace)
		/* Setup namespaced watch for Gateway */
		{
			gateways, err := c.gateway.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "initial Gateway list")
			}
			initialGatewayList = append(initialGatewayList, gateways...)
		}
		gatewayNamespacesChan, gatewayErrs, err := c.gateway.Watch(namespace, opts)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "starting Gateway watch")
		}

		done.Add(1)
		go func(namespace string) {
			defer done.Done()
			errutils.AggregateErrs(ctx, errs, gatewayErrs, namespace+"-gateways")
		}(namespace)

		/* Watch for changes and update snapshot */
		go func(namespace string) {
			for {
				select {
				case <-ctx.Done():
					return
				case virtualServiceList := <-virtualServiceNamespacesChan:
					select {
					case <-ctx.Done():
						return
					case virtualServiceChan <- virtualServiceListWithNamespace{list: virtualServiceList, namespace: namespace}:
					}
				case routeTableList := <-routeTableNamespacesChan:
					select {
					case <-ctx.Done():
						return
					case routeTableChan <- routeTableListWithNamespace{list: routeTableList, namespace: namespace}:
					}
				case gatewayList := <-gatewayNamespacesChan:
					select {
					case <-ctx.Done():
						return
					case gatewayChan <- gatewayListWithNamespace{list: gatewayList, namespace: namespace}:
					}
				}
			}
		}(namespace)
	}
	/* Initialize snapshot for VirtualServices */
	currentSnapshot.VirtualServices = initialVirtualServiceList.Sort()
	/* Initialize snapshot for RouteTables */
	currentSnapshot.RouteTables = initialRouteTableList.Sort()
	/* Initialize snapshot for Gateways */
	currentSnapshot.Gateways = initialGatewayList.Sort()

	snapshots := make(chan *ApiSnapshot)
	go func() {
		// sent initial snapshot to kick off the watch
		initialSnapshot := currentSnapshot.Clone()
		snapshots <- &initialSnapshot

		timer := time.NewTicker(time.Second * 1)
		previousHash := currentSnapshot.Hash()
		sync := func() {
			currentHash := currentSnapshot.Hash()
			if previousHash == currentHash {
				return
			}

			sentSnapshot := currentSnapshot.Clone()
			select {
			case snapshots <- &sentSnapshot:
				stats.Record(ctx, mApiSnapshotOut.M(1))
				previousHash = currentHash
			default:
				stats.Record(ctx, mApiSnapshotMissed.M(1))
			}
		}
		virtualServicesByNamespace := make(map[string]gateway_solo_io.VirtualServiceList)
		routeTablesByNamespace := make(map[string]gateway_solo_io.RouteTableList)
		gatewaysByNamespace := make(map[string]GatewayList)

		for {
			record := func() { stats.Record(ctx, mApiSnapshotIn.M(1)) }

			select {
			case <-timer.C:
				sync()
			case <-ctx.Done():
				close(snapshots)
				done.Wait()
				close(errs)
				return
			case <-c.forceEmit:
				sentSnapshot := currentSnapshot.Clone()
				snapshots <- &sentSnapshot
			case virtualServiceNamespacedList := <-virtualServiceChan:
				record()

				namespace := virtualServiceNamespacedList.namespace

				// merge lists by namespace
				virtualServicesByNamespace[namespace] = virtualServiceNamespacedList.list
				var virtualServiceList gateway_solo_io.VirtualServiceList
				for _, virtualServices := range virtualServicesByNamespace {
					virtualServiceList = append(virtualServiceList, virtualServices...)
				}
				currentSnapshot.VirtualServices = virtualServiceList.Sort()
			case routeTableNamespacedList := <-routeTableChan:
				record()

				namespace := routeTableNamespacedList.namespace

				// merge lists by namespace
				routeTablesByNamespace[namespace] = routeTableNamespacedList.list
				var routeTableList gateway_solo_io.RouteTableList
				for _, routeTables := range routeTablesByNamespace {
					routeTableList = append(routeTableList, routeTables...)
				}
				currentSnapshot.RouteTables = routeTableList.Sort()
			case gatewayNamespacedList := <-gatewayChan:
				record()

				namespace := gatewayNamespacedList.namespace

				// merge lists by namespace
				gatewaysByNamespace[namespace] = gatewayNamespacedList.list
				var gatewayList GatewayList
				for _, gateways := range gatewaysByNamespace {
					gatewayList = append(gatewayList, gateways...)
				}
				currentSnapshot.Gateways = gatewayList.Sort()
			}
		}
	}()
	return snapshots, errs, nil
}
