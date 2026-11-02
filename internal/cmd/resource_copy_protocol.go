package cmd

import "fmt"

// copyHTTPRouter clones the HTTP router and its owned service, if present.
func copyHTTPRouter(src, dst *DynamicConfig, srcName, dstName, dstFile string, changes routeChanges) error {
	if err := changes.validate("http", dstFile); err != nil {
		return err
	}
	if dst.HTTP == nil {
		dst.HTTP = &HTTPConfig{}
	}
	if dst.HTTP.Routers == nil {
		dst.HTTP.Routers = map[string]*Router{}
	}
	if dst.HTTP.Services == nil {
		dst.HTTP.Services = map[string]*Service{}
	}
	if dst.HTTP.Routers[dstName] != nil {
		return fmt.Errorf("router '%s' already exists in destination", dstName)
	}
	router := *src.HTTP.Routers[srcName]
	if err := changes.applyHTTP(&router); err != nil {
		return err
	}
	switch {
	case changes.service != "":
		router.Service = changes.service
	case changes.address != "":
		router.Service = dstName + "-svc"
		if dst.HTTP.Services[router.Service] != nil {
			return fmt.Errorf("service %q already exists in destination", router.Service)
		}
		dst.HTTP.Services[router.Service] = &Service{LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://" + changes.address}}}}
	case src.HTTP.Services[router.Service] != nil:
		svc := *src.HTTP.Services[router.Service]
		if svc.LoadBalancer != nil {
			lb := *svc.LoadBalancer
			lb.Servers = append([]ServerURL{}, lb.Servers...)
			svc.LoadBalancer = &lb
		}
		router.Service = dstName + "-svc"
		if dst.HTTP.Services[router.Service] != nil {
			return fmt.Errorf("service %q already exists in destination", router.Service)
		}
		dst.HTTP.Services[router.Service] = &svc
	default:
		if err := validateExistingService(router.Service, "http", dstFile); err != nil {
			return err
		}
	}
	dst.HTTP.Routers[dstName] = &router
	return nil
}

// copyTCPRouter clones the TCP router and its owned service, if present.
func copyTCPRouter(src, dst *DynamicConfig, srcName, dstName, dstFile string, changes routeChanges) error {
	if err := changes.validate("tcp", dstFile); err != nil {
		return err
	}
	if dst.TCP == nil {
		dst.TCP = &TCPConfig{}
	}
	if dst.TCP.Routers == nil {
		dst.TCP.Routers = map[string]*TCPRouter{}
	}
	if dst.TCP.Services == nil {
		dst.TCP.Services = map[string]*TCPService{}
	}
	if dst.TCP.Routers[dstName] != nil {
		return fmt.Errorf("router '%s' already exists in destination", dstName)
	}
	router := *src.TCP.Routers[srcName]
	if router.TLS != nil {
		tls := *router.TLS
		router.TLS = &tls
	}
	if err := changes.applyTCP(&router); err != nil {
		return err
	}
	switch {
	case changes.service != "":
		router.Service = changes.service
	case changes.address != "":
		router.Service = dstName + "-svc"
		if dst.TCP.Services[router.Service] != nil {
			return fmt.Errorf("service %q already exists in destination", router.Service)
		}
		dst.TCP.Services[router.Service] = &TCPService{LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: changes.address}}}}
	case src.TCP.Services[router.Service] != nil:
		svc := *src.TCP.Services[router.Service]
		if svc.LoadBalancer != nil {
			lb := *svc.LoadBalancer
			lb.Servers = append([]ServerAddress{}, lb.Servers...)
			svc.LoadBalancer = &lb
		}
		router.Service = dstName + "-svc"
		if dst.TCP.Services[router.Service] != nil {
			return fmt.Errorf("service %q already exists in destination", router.Service)
		}
		dst.TCP.Services[router.Service] = &svc
	default:
		if err := validateExistingService(router.Service, "tcp", dstFile); err != nil {
			return err
		}
	}
	dst.TCP.Routers[dstName] = &router
	return nil
}
