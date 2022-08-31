package http

import (
	"errors"
	"fmt"
	"github.com/Ivory-Chan/caddy_discovery_plugin/discovery"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(DiscoveryUpstreams{})
}

// DiscoveryUpstreams provides upstreams from SRV lookups.
// The lookup DNS name can be configured either by
// its individual parts (that is, specifying the
// service, protocol, and name separately) to form
// the standard "_service._proto.name" domain, or
// the domain can be specified directly in name by
// leaving service and proto empty. See RFC 2782.
//
// Lookups are cached and refreshed at the configured
// refresh interval.
//
// Returned upstreams are sorted by priority and weight.
type DiscoveryUpstreams struct {
	// The service label.
	Service string `json:"service,omitempty"`

	Config *discovery.Config `json:"config,omitempty"`

	// The interval at which to refresh the SRV lookup.
	// Results are cached between lookups. Default: 1m
	Refresh caddy.Duration `json:"refresh,omitempty"`

	logger    *zap.Logger
	discovery discovery.Discovery
}

// CaddyModule returns the Caddy module information.
func (DiscoveryUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.discovery",
		New: func() caddy.Module { return new(DiscoveryUpstreams) },
	}
}

func (su *DiscoveryUpstreams) Provision(ctx caddy.Context) error {
	var err error

	su.logger = ctx.Logger(su)
	if su.Refresh == 0 {
		su.Refresh = caddy.Duration(time.Minute)
	}
	if su.Config == nil {
		return errors.New("there is not available config")
	}
	su.discovery, err = discovery.New(su.Config, su.logger)
	if err != nil {
		return err
	}

	go func() {
		c := ctx.Context

		watcher, err := su.discovery.Watch(c, su.Service)
		if err != nil {
			return
		}
		for {
			fmt.Println("Wait for reload upstreams...")
			services, err := watcher.Next()
			fmt.Println("Reload upstreams from discovery ...")
			if err != nil {
				continue
			}
			upstreams, err := su.LoadUpstreamsFromDiscovery(services)
			if err != nil {
				continue
			}

			srvsMu.Lock()
			su.SetUpstreamsCache(upstreams)
			srvsMu.Unlock()

			//fmt.Println(upstreams)
		}
	}()

	return nil
}

func (su DiscoveryUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {

	// first, use a cheap read-lock to return a cached result quickly
	srvsMu.RLock()
	cached := srvs[su.Service]
	srvsMu.RUnlock()
	if cached.isFresh() {
		return cached.upstreams, nil
	}

	// otherwise, obtain a write-lock to update the cached value
	srvsMu.Lock()
	defer srvsMu.Unlock()

	// check to see if it's still stale, since we're now in a different
	// lock from when we first checked freshness; another goroutine might
	// have refreshed it in the meantime before we re-obtained our lock
	cached = srvs[su.Service]
	if cached.isFresh() {
		return cached.upstreams, nil
	}

	su.logger.Debug("refreshing discovery upstreams",
		zap.String("service", su.Service),
	)

	// get services from discovery
	services, err := su.discovery.GetService(r.Context(), su.Service)

	if err != nil {
		if len(services) == 0 {
			return nil, err
		}
		su.logger.Warn("discovery service filtered", zap.Error(err))
	}

	upstreams, err := su.LoadUpstreamsFromDiscovery(services)
	if err != nil {
		su.logger.Warn("discovery service failed", zap.Error(err))
		return nil, err
	}

	srvs[su.Service] = srvLookup{
		DiscoveryUpstreams: su,
		freshness:          time.Now(),
		upstreams:          upstreams,
	}

	return upstreams, nil
}

func (su DiscoveryUpstreams) LoadUpstreamsFromDiscovery(services []*registry.ServiceInstance) ([]*reverseproxy.Upstream, error) {

	var upstreams []*reverseproxy.Upstream
	var nodes []selector.Node

	for _, service := range services {
		su.logger.Debug("discovered records from discovery") //zap.String("target", rec.),

		for n := 0; n < len(service.Endpoints); n++ {
			addr, _ := url.Parse(service.Endpoints[n])
			nodes = append(nodes, selector.NewNode(addr.Scheme, addr.Host, service))
		}
	}

	for n := 0; n < len(nodes); n++ {
		upstreams = append(upstreams, &reverseproxy.Upstream{Dial: nodes[n].Address()})
	}

	return upstreams, nil
}

func (su DiscoveryUpstreams) SetUpstreamsCache(upstreams []*reverseproxy.Upstream) {

	//// check to see if it's still stale, since we're now in a different
	//// lock from when we first checked freshness; another goroutine might
	//// have refreshed it in the meantime before we re-obtained our lock
	cached := srvs[su.Service]

	//before adding a new one to the cache (as opposed to replacing stale one), make room if cache is full
	if cached.freshness.IsZero() && len(srvs) >= 100 {
		for randomKey := range srvs {
			delete(srvs, randomKey)
			break
		}
	}

	srvs[su.Service] = srvLookup{
		DiscoveryUpstreams: su,
		freshness:          time.Now(),
		upstreams:          upstreams,
	}
}

func (su DiscoveryUpstreams) String() string {
	return su.Service
}

// formattedAddr the RFC 2782 representation of the SRV domain, in
// the form "_service._proto.name".
func (DiscoveryUpstreams) formattedAddr(service, proto, name string) string {
	return fmt.Sprintf("_%s._%s.%s", service, proto, name)
}

type srvLookup struct {
	DiscoveryUpstreams DiscoveryUpstreams
	freshness          time.Time
	upstreams          []*reverseproxy.Upstream
}

func (sl srvLookup) isFresh() bool {
	return time.Since(sl.freshness) < time.Duration(sl.DiscoveryUpstreams.Refresh)
}

var (
	srvs   = make(map[string]srvLookup)
	srvsMu sync.RWMutex
)

// Interface guards
var (
	_ caddy.Provisioner           = (*DiscoveryUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*DiscoveryUpstreams)(nil)
)
