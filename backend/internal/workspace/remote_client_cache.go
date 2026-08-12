package workspace

import (
	"context"
	"sync"
	"time"
)

type cachedClient struct {
	transport RemoteTransport
	lastUsed  int64
	configGen int64
	credGen   int64
	inUse     bool
}

type remoteClientCache struct {
	mu      sync.Mutex
	policy  RemotePolicy
	clients map[WorkspaceID][]*cachedClient
	counter int64
}

func newRemoteClientCache(policy RemotePolicy) *remoteClientCache {
	return &remoteClientCache{
		policy:  policy,
		clients: make(map[WorkspaceID][]*cachedClient),
	}
}

func (c *remoteClientCache) getOrCreate(ctx context.Context, mountID WorkspaceID, config RemoteMountConfig, cred *RemoteCredential) (RemoteTransport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.counter++
	now := c.counter

	configGen := computeConfigGen(config)
	credGen := computeCredGen(cred)

	existing, ok := c.clients[mountID]
	if ok {
		active := existing[:0]
		for _, cc := range existing {
			if cc.configGen == configGen && cc.credGen == credGen {
				if !cc.inUse {
					cc.inUse = true
					cc.lastUsed = now
					active = append(active, cc)
					c.clients[mountID] = active
					return cc.transport, nil
				}
				active = append(active, cc)
			} else {
				cc.transport.Close()
			}
		}
		c.clients[mountID] = active
	}

	if c.countLocked() >= c.policy.MaxGlobalConnections {
		c.evictIdleLocked(now)
	}

	if c.countLocked() >= c.policy.MaxGlobalConnections {
		return nil, ErrRemoteClientCacheExhausted
	}

	currentMountCount := 0
	for _, cc := range c.clients[mountID] {
		if cc.inUse {
			currentMountCount++
		}
	}
	if currentMountCount >= c.policy.MaxConnectionsPerMount {
		for _, cc := range c.clients[mountID] {
			if !cc.inUse {
				cc.transport.Close()
			}
		}
		var remaining []*cachedClient
		for _, cc := range c.clients[mountID] {
			if cc.inUse {
				remaining = append(remaining, cc)
			}
		}
		c.clients[mountID] = remaining
		if len(remaining) >= c.policy.MaxConnectionsPerMount {
			return nil, ErrRemoteClientCacheExhausted
		}
	}

	transport, err := c.createTransport(ctx, config, cred)
	if err != nil {
		return nil, err
	}

	cc := &cachedClient{
		transport: transport,
		lastUsed:  now,
		configGen: configGen,
		credGen:   credGen,
		inUse:     true,
	}
	c.clients[mountID] = append(c.clients[mountID], cc)

	return transport, nil
}

func (c *remoteClientCache) release(mountID WorkspaceID, transport RemoteTransport) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, ok := c.clients[mountID]
	if !ok {
		return
	}
	c.counter++
	for _, cc := range existing {
		if cc.transport == transport {
			cc.inUse = false
			cc.lastUsed = c.counter
			return
		}
	}
}

func (c *remoteClientCache) invalidate(mountID WorkspaceID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, ok := c.clients[mountID]
	if !ok {
		return
	}
	for _, cc := range existing {
		cc.transport.Close()
	}
	delete(c.clients, mountID)
}

func (c *remoteClientCache) invalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, clients := range c.clients {
		for _, cc := range clients {
			cc.transport.Close()
		}
	}
	c.clients = make(map[WorkspaceID][]*cachedClient)
}

func (c *remoteClientCache) countLocked() int {
	count := 0
	for _, clients := range c.clients {
		count += len(clients)
	}
	return count
}

func (c *remoteClientCache) evictIdleLocked(now int64) {
	idleThreshold := c.policy.IdleTTL.Nanoseconds()
	for mountID, clients := range c.clients {
		active := make([]*cachedClient, 0, len(clients))
		for _, cc := range clients {
			if !cc.inUse && now-cc.lastUsed > idleThreshold {
				cc.transport.Close()
				continue
			}
			active = append(active, cc)
		}
		if len(active) == 0 {
			delete(c.clients, mountID)
		} else {
			c.clients[mountID] = active
		}
	}
}

func (c *remoteClientCache) createTransport(ctx context.Context, config RemoteMountConfig, cred *RemoteCredential) (RemoteTransport, error) {
	switch config.Protocol {
	case RemoteProtocolSFTP:
		return newSFTPTransport(ctx, config, cred)
	case RemoteProtocolWebDAV:
		return newWebDAVTransport(ctx, config, cred)
	default:
		return nil, ErrRemoteProtocolUnsupported
	}
}

func computeConfigGen(config RemoteMountConfig) int64 {
	port := config.Port
	return int64(len(config.Host)*31 + port*17 + len(config.BasePath)*13 + len(string(config.Protocol))*7)
}

func computeCredGen(cred *RemoteCredential) int64 {
	if cred == nil {
		return 0
	}
	return int64(len(cred.Username)*11 + len(credentialFingerprint(cred)))
}

func credentialFingerprint(cred *RemoteCredential) []byte {
	switch cred.Type {
	case RemoteAuthTypePassword:
		return cred.Password
	case RemoteAuthTypePrivateKey:
		return cred.PrivateKey
	case RemoteAuthTypeBearer:
		return cred.BearerToken
	}
	return nil
}

var _ = time.Now
