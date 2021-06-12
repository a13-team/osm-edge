package ads

import (
	"fmt"
	"time"

	mapset "github.com/deckarep/golang-set"
	xds_discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/rs/zerolog"

	"github.com/openservicemesh/osm/pkg/certificate"
	"github.com/openservicemesh/osm/pkg/envoy"
	"github.com/openservicemesh/osm/pkg/metricsstore"
)

const (
	// MaxXdsLogsPerProxy keeps a higher bound of how many timestamps do we keep per proxy
	MaxXdsLogsPerProxy = 20
)

func xdsPathTimeTrack(startedAt time.Time, log *zerolog.Event, typeURI envoy.TypeURI, proxy *envoy.Proxy, success bool) {
	elapsed := time.Since(startedAt)

	log.Msgf("[%s] processing for Proxy with Certificate SerialNumber=%s took %s", typeURI, proxy.GetCertificateSerialNumber(), elapsed)

	metricsstore.DefaultMetricsStore.ProxyConfigUpdateTime.
		WithLabelValues(typeURI.String(), fmt.Sprintf("%t", success)).
		Observe(elapsed.Seconds())
}

func (s *Server) trackXDSLog(cn certificate.CommonName, typeURL envoy.TypeURI) {
	s.withXdsLogMutex(func() {
		if _, ok := s.xdsLog[cn]; !ok {
			s.xdsLog[cn] = make(map[envoy.TypeURI][]time.Time)
		}

		timeSlice, ok := s.xdsLog[cn][typeURL]
		if !ok {
			s.xdsLog[cn][typeURL] = []time.Time{time.Now()}
			return
		}

		timeSlice = append(timeSlice, time.Now())
		if len(timeSlice) > MaxXdsLogsPerProxy {
			timeSlice = timeSlice[1:]
		}
		s.xdsLog[cn][typeURL] = timeSlice
	})
}

// validateRequestResponse is a utility function to validate the response generated by a given request.
// currently, it checks that all resources responded for a request are being responded to,
// or will log as `warn` otherwise
// Returns the number of resources NOT being answered for a request. For wildcard case, this is always 0.
func validateRequestResponse(proxy *envoy.Proxy, request *xds_discovery.DiscoveryRequest, respResources []types.Resource) int {
	// No validation is done for wildcard cases
	resourcesRequested := mapset.NewSet()
	resourcesToSend := mapset.NewSet()

	// Get resources being requested
	for _, reqResource := range request.ResourceNames {
		resourcesRequested.Add(reqResource)
	}

	// Get resources being responded by name
	for _, res := range respResources {
		resourcesToSend.Add(cache.GetResourceName(res))
	}

	// Compute difference from request resources' perspective
	resDifference := resourcesRequested.Difference(resourcesToSend)
	diffCardinality := resDifference.Cardinality()
	if diffCardinality != 0 {
		log.Warn().Msgf("Proxy %s: not all request resources for type %s are being responded to req [%v] resp [%v] diff [%v]",
			proxy.String(), envoy.TypeURI(request.TypeUrl).Short(),
			resourcesRequested, resourcesToSend, resDifference)
	}

	return diffCardinality
}
