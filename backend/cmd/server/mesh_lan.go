package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/spaceidentity"
)

func startMeshLAN(ctx context.Context, services *AppServices, handler http.Handler) error {
	if services == nil || services.DeviceMesh == nil || services.DeviceMesh.LocalHandler == nil {
		return nil
	}
	ip := net.ParseIP(config.AppCfg.Server.Host)
	if ip == nil || !ip.IsLoopback() {
		return nil
	}
	addresses, err := lan.PrivateAddresses()
	if err != nil {
		return err
	}
	signer, err := agent.NewIdentityStore(config.AppCfg.Storage.DataDir).CryptoSigner()
	if err != nil {
		return err
	}
	var server *lan.Server
	if len(addresses) > 0 {
		server, err = lan.Start(handler, signer, spaceidentity.DefaultSpaceID(), config.AppCfg.Server.Port, addresses)
		if err != nil {
			return err
		}
		services.DeviceMesh.LocalHandler.SetLANEndpoints(server.Endpoints)
	}
	addressStamp := func(addresses []net.IP) string {
		values := make([]string, len(addresses))
		for i, address := range addresses {
			values[i] = address.String()
		}
		return strings.Join(values, ",")
	}
	stamp := addressStamp(addresses)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		refreshed := time.Now()
		closeServer := func() {
			services.DeviceMesh.LocalHandler.SetLANEndpoints(nil)
			if server != nil {
				shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = server.Close(shutdown)
				cancel()
				server = nil
			}
		}
		defer closeServer()
		for {
			var failures <-chan error
			if server != nil {
				failures = server.Errors()
			}
			select {
			case <-ctx.Done():
				return
			case err := <-failures:
				log.Printf("device mesh LAN listener stopped: %v", err)
				closeServer()
			case <-ticker.C:
			}
			addresses, err := lan.PrivateAddresses()
			if err != nil {
				log.Printf("device mesh LAN addresses unavailable: %v", err)
				continue
			}
			next := addressStamp(addresses)
			if next == stamp && server != nil && time.Since(refreshed) < 24*time.Hour {
				continue
			}
			closeServer()
			stamp = next
			if len(addresses) == 0 {
				continue
			}
			server, err = lan.Start(handler, signer, spaceidentity.DefaultSpaceID(), config.AppCfg.Server.Port, addresses)
			if err != nil {
				log.Printf("device mesh LAN listener restart failed: %v", err)
				continue
			}
			refreshed = time.Now()
			services.DeviceMesh.LocalHandler.SetLANEndpoints(server.Endpoints)
		}
	}()
	return nil
}
