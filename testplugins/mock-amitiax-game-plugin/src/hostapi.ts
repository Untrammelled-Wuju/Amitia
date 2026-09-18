import type { Client, Envelope } from '@amitia/game-plugin-sdk';
import { checkPermission, getPermissionSnapshot, requestPermission } from '@amitia/game-plugin-sdk';
import { invokeHostAPI } from '@amitia/game-plugin-sdk';
import { acquireSecret, releaseSecret, querySecretLease } from '@amitia/game-plugin-sdk';
import { networkTCPOpen, networkTCPRead, networkTCPWrite, networkTCPClose, networkUDPOpen, networkUDPReceive, networkUDPSend, networkUDPClose, networkWebSocketOpen, networkWebSocketReceive, networkWebSocketSend, networkWebSocketClose } from '@amitia/game-plugin-sdk';

export class HostAPIHandler {
  private leaseId: string = '';
  private secretRef: string = 'secret://provider/worldgame_provider_token';

  hasActiveLease(): boolean {
    return this.leaseId !== '';
  }

  getLeaseId(): string {
    return this.leaseId;
  }

  getSecretRef(): string {
    return this.secretRef;
  }

  setLeaseId(leaseId: string): void {
    this.leaseId = leaseId;
  }

  async handlePermissionCheck(client: Client, request: Envelope): Promise<unknown> {
    const payload = (request.payload as { permissionId?: string }) || {};
    const permissionId = payload.permissionId || 'gamehost.control';
    return checkPermission(client, { permissionId });
  }

  async handlePermissionSnapshot(client: Client, request: Envelope): Promise<unknown> {
    void request;
    return getPermissionSnapshot(client);
  }

  async handlePermissionRequest(client: Client, request: Envelope): Promise<unknown> {
    const payload = (request.payload as { permissionId?: string }) || {};
    const permissionId = payload.permissionId || 'gamehost.control';
    return requestPermission(client, { permissionId });
  }

  async handleHostAPICheck(request: Envelope): Promise<unknown> {
    const payload = (request.payload as { method?: string; route?: string }) || {};
    return {
      method: payload.method || payload.route || 'unknown',
      registered: false,
      invokeResult: 'pending',
    };
  }

  async handleSecretProbe(): Promise<unknown> {
    return {
      leaseId: this.leaseId,
      hasLease: this.hasActiveLease(),
      leaseStatus: this.hasActiveLease() ? 'granted' : 'none',
      ref: this.secretRef,
    };
  }

  async handleReleaseLease(): Promise<unknown> {
    if (!this.hasActiveLease()) {
      return { released: false, reason: 'no_active_lease' };
    }
    this.leaseId = '';
    return { released: true, reason: 'ok' };
  }

  async handleAcquireLease(client: import('@amitia/game-plugin-sdk').Client): Promise<unknown> {
    const result = await acquireSecret(client, {
      ref: this.secretRef,
      purpose: 'startup',
    });
    if (result.granted && result.leaseId) {
      this.leaseId = result.leaseId;
    }
    return result;
  }

  async handleReleaseLeaseFull(client: import('@amitia/game-plugin-sdk').Client): Promise<unknown> {
    if (!this.hasActiveLease()) {
      return { released: false, reason: 'no_active_lease' };
    }
    const result = await releaseSecret(client, {
      leaseId: this.leaseId,
    });
    if (result.released) {
      this.leaseId = '';
    }
    return result;
  }

  async handleQueryLease(client: import('@amitia/game-plugin-sdk').Client): Promise<unknown> {
    if (!this.hasActiveLease()) {
      return { valid: false, granted: false, reason: 'no_active_lease' };
    }
    return querySecretLease(client, { leaseId: this.leaseId });
  }

  async handleInvokeHostAPI(client: import('@amitia/game-plugin-sdk').Client, request: Envelope): Promise<unknown> {
    const payload = (request.payload as { method?: string; input?: unknown }) || {};
    return invokeHostAPI(client, {
      method: payload.method || 'host.runtime.health',
      input: payload.input || {},
    });
  }


  async handleMediatedSocketProbe(client: import('@amitia/game-plugin-sdk').Client): Promise<unknown> {
    let tcpRoundtrip = '';
    let udpRoundtrip = '';
    let websocketRoundtrip = '';
    let websocketMessageType = '';

    const tcp = await networkTCPOpen(client, { target: 'host-loopback', port: 18901, timeoutMs: 5000 });
    try {
      await networkTCPWrite(client, {
        handleId: tcp.handleId,
        dataBase64: Buffer.from('generic-game-tcp', 'utf8').toString('base64'),
        timeoutMs: 5000,
      });
      const received = await networkTCPRead(client, { handleId: tcp.handleId, maxBytes: 65536, timeoutMs: 5000 });
      tcpRoundtrip = Buffer.from(received.dataBase64, 'base64').toString('utf8');
    } finally {
      await networkTCPClose(client, { handleId: tcp.handleId }).catch(() => undefined);
    }

    const udp = await networkUDPOpen(client, { target: 'host-loopback', port: 18902, timeoutMs: 5000 });
    try {
      await networkUDPSend(client, {
        handleId: udp.handleId,
        dataBase64: Buffer.from('generic-game-udp', 'utf8').toString('base64'),
        timeoutMs: 5000,
      });
      const received = await networkUDPReceive(client, { handleId: udp.handleId, maxBytes: 65536, timeoutMs: 5000 });
      udpRoundtrip = Buffer.from(received.dataBase64, 'base64').toString('utf8');
    } finally {
      await networkUDPClose(client, { handleId: udp.handleId }).catch(() => undefined);
    }

    const ws = await networkWebSocketOpen(client, { url: 'ws://host-loopback:18903/echo', timeoutMs: 5000 });
    try {
      await networkWebSocketSend(client, {
        handleId: ws.handleId,
        messageType: 'text',
        dataBase64: Buffer.from('generic-game-websocket', 'utf8').toString('base64'),
        timeoutMs: 5000,
      });
      const received = await networkWebSocketReceive(client, { handleId: ws.handleId, timeoutMs: 5000 });
      websocketRoundtrip = Buffer.from(received.dataBase64, 'base64').toString('utf8');
      websocketMessageType = received.messageType;
    } finally {
      await networkWebSocketClose(client, { handleId: ws.handleId }).catch(() => undefined);
    }

    const blockedPort = await invokeHostAPI(client, {
      method: 'host.network.tcp.open',
      input: { target: 'host-loopback', port: 18904, timeoutMs: 1000 },
    });
    const blockedTarget = await invokeHostAPI(client, {
      method: 'host.network.tcp.open',
      input: { target: '127.0.0.2', port: 18901, timeoutMs: 1000 },
    });

    return {
      tcpRoundtrip,
      udpRoundtrip,
      websocketRoundtrip,
      websocketMessageType,
      blockedPortStatus: blockedPort.status,
      blockedTargetStatus: blockedTarget.status,
    };
  }

  async handleRestrictedNetworkProbe(client: import('@amitia/game-plugin-sdk').Client): Promise<unknown> {
    const allowedUrl = 'http://127.0.0.1:18899/api/public/health';
    let directSucceeded = false;
    let directError = '';
    try {
      const response = await fetch(allowedUrl, { signal: AbortSignal.timeout(5000) });
      directSucceeded = response.ok || response.status > 0;
      await response.body?.cancel();
    } catch (error) {
      directError = error instanceof Error ? error.message : String(error);
    }

    const mediated = await invokeHostAPI(client, {
      method: 'host.network.request',
      input: { method: 'GET', url: allowedUrl, timeoutMs: 10000, maxResponseBytes: 65536 },
    });
    const blockedIp = await invokeHostAPI(client, {
      method: 'host.network.request',
      input: { method: 'GET', url: 'http://127.0.0.2:18899/api/public/health', timeoutMs: 1000 },
    });
    const blockedPort = await invokeHostAPI(client, {
      method: 'host.network.request',
      input: { method: 'GET', url: 'http://127.0.0.1:18898/api/public/health', timeoutMs: 1000 },
    });

    return {
      directSucceeded,
      directError,
      mediatedStatus: mediated.status,
      mediatedOutput: mediated.output,
      blockedIpStatus: blockedIp.status,
      blockedPortStatus: blockedPort.status,
    };
  }

}

export { checkPermission, getPermissionSnapshot, requestPermission, invokeHostAPI };
