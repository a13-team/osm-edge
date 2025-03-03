package repo

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openservicemesh/osm/pkg/catalog"
	"github.com/openservicemesh/osm/pkg/certificate"
	"github.com/openservicemesh/osm/pkg/errcode"
	"github.com/openservicemesh/osm/pkg/identity"
	"github.com/openservicemesh/osm/pkg/k8s"
	"github.com/openservicemesh/osm/pkg/service"
	"github.com/openservicemesh/osm/pkg/sidecar/providers/pipy"
	"github.com/openservicemesh/osm/pkg/sidecar/providers/pipy/client"
)

// PipyConfGeneratorJob is the job to generate pipy policy json
type PipyConfGeneratorJob struct {
	proxy      *pipy.Proxy
	repoServer *Server

	// Optional waiter
	done chan struct{}
}

// GetDoneCh returns the channel, which when closed, indicates the job has been finished.
func (job *PipyConfGeneratorJob) GetDoneCh() <-chan struct{} {
	return job.done
}

// Run is the logic unit of job
func (job *PipyConfGeneratorJob) Run() {
	defer close(job.done)
	if job.proxy == nil {
		return
	}

	s := job.repoServer
	proxy := job.proxy

	proxy.Mutex.Lock()
	defer proxy.Mutex.Unlock()

	proxyServices, err := s.proxyRegistry.ListProxyServices(proxy)
	if err != nil {
		log.Warn().Err(err).Str(errcode.Kind, errcode.GetErrCodeWithMetric(errcode.ErrFetchingServiceList)).
			Msgf("Error looking up services for Sidecar with name=%s", proxy.GetName())
		return
	}

	cataloger := s.catalog
	pipyConf := new(PipyConf)

	probes(proxy, pipyConf)
	features(s, proxy, pipyConf)
	certs(s, proxy, pipyConf, proxyServices)
	pluginSetV := plugin(cataloger, s, pipyConf, proxy)
	inbound(cataloger, proxy.Identity, s, pipyConf, proxyServices)
	outbound(cataloger, proxy.Identity, s, pipyConf, proxy)
	egress(cataloger, proxy.Identity, s, pipyConf, proxy)
	forward(cataloger, proxy.Identity, s, pipyConf, proxy)
	balance(pipyConf)
	reorder(pipyConf)
	endpoints(pipyConf, s)
	job.publishSidecarConf(s.repoClient, proxy, pipyConf, pluginSetV)
}

func endpoints(pipyConf *PipyConf, s *Server) {
	ready := pipyConf.copyAllowedEndpoints(s.kubeController, s.proxyRegistry)
	if !ready {
		if s.retryProxiesJob != nil {
			s.retryProxiesJob()
		}
	}
}

func balance(pipyConf *PipyConf) {
	pipyConf.rebalancedOutboundClusters()
	pipyConf.rebalancedForwardClusters()
}

func reorder(pipyConf *PipyConf) {
	if pipyConf.Outbound != nil && pipyConf.Outbound.TrafficMatches != nil {
		for _, trafficMatches := range pipyConf.Outbound.TrafficMatches {
			for _, trafficMatch := range trafficMatches {
				for _, routeRules := range trafficMatch.HTTPServiceRouteRules {
					routeRules.RouteRules.sort()
				}
			}
		}
		pipyConf.Outbound.TrafficMatches.Sort()
	}

	if pipyConf.Inbound != nil && pipyConf.Inbound.TrafficMatches != nil {
		for _, trafficMatches := range pipyConf.Inbound.TrafficMatches {
			for _, routeRules := range trafficMatches.HTTPServiceRouteRules {
				routeRules.sort()
			}
		}
	}
}

func egress(cataloger catalog.MeshCataloger, serviceIdentity identity.ServiceIdentity, s *Server, pipyConf *PipyConf, proxy *pipy.Proxy) bool {
	egressTrafficPolicy, egressErr := cataloger.GetEgressTrafficPolicy(serviceIdentity)
	if egressErr != nil {
		if s.retryProxiesJob != nil {
			s.retryProxiesJob()
		}
		return false
	}

	if egressTrafficPolicy != nil {
		egressDependClusters := generatePipyEgressTrafficRoutePolicy(cataloger, serviceIdentity, pipyConf,
			egressTrafficPolicy)
		if len(egressDependClusters) > 0 {
			if ready := generatePipyEgressTrafficBalancePolicy(cataloger, proxy, serviceIdentity, pipyConf,
				egressTrafficPolicy, egressDependClusters); !ready {
				if s.retryProxiesJob != nil {
					s.retryProxiesJob()
				}
				return false
			}
		}
	}
	return true
}

func forward(cataloger catalog.MeshCataloger, serviceIdentity identity.ServiceIdentity, s *Server, pipyConf *PipyConf, _ *pipy.Proxy) bool {
	egressGatewayPolicy, egressErr := cataloger.GetEgressGatewayPolicy()
	if egressErr != nil {
		if s.retryProxiesJob != nil {
			s.retryProxiesJob()
		}
		return false
	}
	if egressGatewayPolicy != nil {
		if ready := generatePipyEgressTrafficForwardPolicy(cataloger, serviceIdentity, pipyConf,
			egressGatewayPolicy); !ready {
			if s.retryProxiesJob != nil {
				s.retryProxiesJob()
			}
			return false
		}
	}
	return true
}

func outbound(cataloger catalog.MeshCataloger, serviceIdentity identity.ServiceIdentity, s *Server, pipyConf *PipyConf, proxy *pipy.Proxy) bool {
	outboundTrafficPolicy := cataloger.GetOutboundMeshTrafficPolicy(serviceIdentity)
	if len(outboundTrafficPolicy.ServicesResolvableSet) > 0 {
		pipyConf.DNSResolveDB = outboundTrafficPolicy.ServicesResolvableSet
	}
	outboundDependClusters := generatePipyOutboundTrafficRoutePolicy(cataloger, serviceIdentity, pipyConf,
		outboundTrafficPolicy)
	if len(outboundDependClusters) > 0 {
		if ready := generatePipyOutboundTrafficBalancePolicy(cataloger, proxy, serviceIdentity, pipyConf,
			outboundTrafficPolicy, outboundDependClusters); !ready {
			if s.retryProxiesJob != nil {
				s.retryProxiesJob()
			}
			return false
		}
	}
	return true
}

func inbound(cataloger catalog.MeshCataloger, serviceIdentity identity.ServiceIdentity, s *Server, pipyConf *PipyConf, proxyServices []service.MeshService) {
	// Build inbound mesh route configurations. These route configurations allow
	// the services associated with this proxy to accept traffic from downstream
	// clients on allowed routes.
	inboundTrafficPolicy := cataloger.GetInboundMeshTrafficPolicy(serviceIdentity, proxyServices)
	generatePipyInboundTrafficPolicy(cataloger, serviceIdentity, pipyConf, inboundTrafficPolicy, s.certManager.GetTrustDomain())
	if len(proxyServices) > 0 {
		for _, svc := range proxyServices {
			if ingressTrafficPolicy, ingressErr := cataloger.GetIngressTrafficPolicy(svc); ingressErr == nil {
				if ingressTrafficPolicy != nil {
					generatePipyIngressTrafficRoutePolicy(cataloger, serviceIdentity, pipyConf, ingressTrafficPolicy)
				}
			}
			if aclTrafficPolicy, aclErr := cataloger.GetAccessControlTrafficPolicy(svc); aclErr == nil {
				if aclTrafficPolicy != nil {
					generatePipyAccessControlTrafficRoutePolicy(cataloger, serviceIdentity, pipyConf, aclTrafficPolicy)
				}
			}
			if expTrafficPolicy, expErr := cataloger.GetExportTrafficPolicy(svc); expErr == nil {
				if expTrafficPolicy != nil {
					generatePipyServiceExportTrafficRoutePolicy(cataloger, serviceIdentity, pipyConf, expTrafficPolicy)
				}
			}
		}
	}
}

func plugin(cataloger catalog.MeshCataloger, s *Server, pipyConf *PipyConf, proxy *pipy.Proxy) (pluginSetVersion string) {
	pipyConf.Chains = nil

	defer func() {
		if pipyConf.Chains == nil {
			setSidecarChain(s.cfg, pipyConf, nil, nil)
		}
	}()

	if !s.cfg.GetFeatureFlags().EnablePluginPolicy {
		return
	}

	pluginChains := cataloger.GetPluginChains()
	if len(pluginChains) == 0 {
		return
	}

	pod, err := s.kubeController.GetPodForProxy(proxy)
	if err != nil {
		log.Warn().Str("proxy", proxy.String()).Msg("Could not find pod for connecting proxy.")
		return
	}

	ns := s.kubeController.GetNamespace(pod.Namespace)
	if ns == nil {
		log.Warn().Str("proxy", proxy.String()).Str("namespace", pod.Namespace).Msg("Could not find namespace for connecting proxy.")
	}

	pluginSet, pluginPri := s.updatePlugins()
	plugin2MountPoint2Config, mountPoint2Plugins := walkPluginChain(pluginChains, ns, pod, pluginSet, s, proxy)
	meshSvc2Plugin2MountPoint2Config := walkPluginConfig(cataloger, plugin2MountPoint2Config)

	pipyConf.pluginPolicies = meshSvc2Plugin2MountPoint2Config
	setSidecarChain(s.cfg, pipyConf, pluginPri, mountPoint2Plugins)

	pluginSetVersion = s.pluginSetVersion
	return
}

func certs(s *Server, proxy *pipy.Proxy, pipyConf *PipyConf, proxyServices []service.MeshService) {
	if mc, ok := s.catalog.(*catalog.MeshCatalog); ok {
		meshConf := mc.GetConfigurator()
		if !(*meshConf).GetSidecarDisabledMTLS() {
			cnPrefix := proxy.Identity.String()
			if proxy.SidecarCert == nil {
				pipyConf.Certificate = nil
				sidecarCert := s.certManager.GetCertificate(cnPrefix)
				if sidecarCert == nil {
					proxy.SidecarCert = nil
				} else {
					proxy.SidecarCert = sidecarCert
				}
			}
			if proxy.SidecarCert == nil || s.certManager.ShouldRotate(proxy.SidecarCert) {
				pipyConf.Certificate = nil
				now := time.Now()
				certValidityPeriod := s.cfg.GetServiceCertValidityPeriod()
				certExpiration := now.Add(certValidityPeriod)
				certValidityPeriod = certExpiration.Sub(now)

				var sans []string
				if len(proxyServices) > 0 {
					for _, proxySvc := range proxyServices {
						sans = append(sans, k8s.GetHostnamesForService(proxySvc, true)...)
					}
				}

				sidecarCert, certErr := s.certManager.IssueCertificate(cnPrefix, certificate.Service,
					certificate.SubjectAlternativeNames(sans...),
					certificate.ValidityDurationProvided(&certValidityPeriod))
				if certErr != nil {
					proxy.SidecarCert = nil
				} else {
					sidecarCert.Expiration = certExpiration
					proxy.SidecarCert = sidecarCert
				}
			}
		} else {
			proxy.SidecarCert = nil
		}
	}
}

func features(s *Server, proxy *pipy.Proxy, pipyConf *PipyConf) {
	if mc, ok := s.catalog.(*catalog.MeshCatalog); ok {
		meshConf := mc.GetConfigurator()
		proxy.MeshConf = meshConf
		pipyConf.setSidecarLogLevel((*meshConf).GetMeshConfig().Spec.Sidecar.LogLevel)
		pipyConf.setEnableSidecarActiveHealthChecks((*meshConf).GetFeatureFlags().EnableSidecarActiveHealthChecks)
		pipyConf.setEnableEgress((*meshConf).IsEgressEnabled())
		pipyConf.setEnablePermissiveTrafficPolicyMode((*meshConf).IsPermissiveTrafficPolicyMode())
		pipyConf.setLocalDNSProxy((*meshConf).IsLocalDNSProxyEnabled(), (*meshConf).GetLocalDNSProxyPrimaryUpstream(), (*meshConf).GetLocalDNSProxySecondaryUpstream())
		clusterProps := (*meshConf).GetMeshConfig().Spec.ClusterSet.Properties
		if len(clusterProps) > 0 {
			pipyConf.Spec.ClusterSet = make(map[string]string)
			for _, prop := range clusterProps {
				pipyConf.Spec.ClusterSet[prop.Name] = prop.Value
			}
		}
	}
}

func probes(proxy *pipy.Proxy, pipyConf *PipyConf) {
	if proxy.PodMetadata != nil {
		if len(proxy.PodMetadata.StartupProbes) > 0 {
			for idx := range proxy.PodMetadata.StartupProbes {
				pipyConf.Spec.Probes.StartupProbes = append(pipyConf.Spec.Probes.StartupProbes, *proxy.PodMetadata.StartupProbes[idx])
			}
		}
		if len(proxy.PodMetadata.LivenessProbes) > 0 {
			for idx := range proxy.PodMetadata.LivenessProbes {
				pipyConf.Spec.Probes.LivenessProbes = append(pipyConf.Spec.Probes.LivenessProbes, *proxy.PodMetadata.LivenessProbes[idx])
			}
		}
		if len(proxy.PodMetadata.ReadinessProbes) > 0 {
			for idx := range proxy.PodMetadata.ReadinessProbes {
				pipyConf.Spec.Probes.ReadinessProbes = append(pipyConf.Spec.Probes.ReadinessProbes, *proxy.PodMetadata.ReadinessProbes[idx])
			}
		}
	}
}

func (job *PipyConfGeneratorJob) publishSidecarConf(repoClient *client.PipyRepoClient, proxy *pipy.Proxy, pipyConf *PipyConf, pluginSetV string) {
	pipyConf.Ts = nil
	pipyConf.Version = nil
	pipyConf.Certificate = nil
	if proxy.SidecarCert != nil {
		pipyConf.Certificate = &Certificate{
			Expiration: proxy.SidecarCert.Expiration.Format("2006-01-02 15:04:05"),
		}
	}
	bytes, jsonErr := json.Marshal(pipyConf)

	if jsonErr == nil {
		codebasePreV := proxy.ETag
		bytes = append(bytes, []byte(pluginSetV)...)
		codebaseCurV := hash(bytes)
		if codebaseCurV != codebasePreV {
			log.Log().Str("Proxy", proxy.GetCNPrefix()).
				Str("ID", fmt.Sprintf("%d", proxy.ID)).
				Str("codebasePreV", fmt.Sprintf("%d", codebasePreV)).
				Str("codebaseCurV", fmt.Sprintf("%d", codebaseCurV)).
				Msg("config.json")
			codebase := fmt.Sprintf("%s/%s", osmSidecarCodebase, proxy.GetCNPrefix())
			success, err := repoClient.DeriveCodebase(codebase, osmCodebaseRepo, codebaseCurV-2)
			if success {
				ts := time.Now()
				pipyConf.Ts = &ts
				version := fmt.Sprintf("%d", codebaseCurV)
				pipyConf.Version = &version
				if proxy.SidecarCert != nil {
					pipyConf.Certificate.CommonName = &proxy.SidecarCert.CommonName
					pipyConf.Certificate.CertChain = string(proxy.SidecarCert.CertChain)
					pipyConf.Certificate.PrivateKey = string(proxy.SidecarCert.PrivateKey)
					pipyConf.Certificate.IssuingCA = string(proxy.SidecarCert.IssuingCA)
				}
				bytes, _ = json.MarshalIndent(pipyConf, "", " ")
				_, err = repoClient.Batch(fmt.Sprintf("%d", codebaseCurV-1), []client.Batch{
					{
						Basepath: codebase,
						Items: []client.BatchItem{
							{
								Filename: osmCodebaseConfig,
								Content:  bytes,
							},
						},
					},
				})
			}
			if err != nil {
				log.Error().Err(err)
				_, _ = repoClient.Delete(codebase)
			} else {
				proxy.ETag = codebaseCurV
			}
		}
	}
}

// JobName implementation for this job, for logging purposes
func (job *PipyConfGeneratorJob) JobName() string {
	return fmt.Sprintf("pipyJob-%s", job.proxy.GetName())
}
