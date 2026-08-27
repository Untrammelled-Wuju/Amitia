import { Client, MessageOption } from './client';
import { HOST_API_STATUS_SUCCESS, invokeHostAPI } from './hostapi';

export const HOST_NETWORK_METHOD = {
  REQUEST: 'host.network.request',
  TCP_OPEN: 'host.network.tcp.open',
  TCP_READ: 'host.network.tcp.read',
  TCP_WRITE: 'host.network.tcp.write',
  TCP_CLOSE: 'host.network.tcp.close',
  UDP_OPEN: 'host.network.udp.open',
  UDP_RECEIVE: 'host.network.udp.receive',
  UDP_SEND: 'host.network.udp.send',
  UDP_CLOSE: 'host.network.udp.close',
  WEBSOCKET_OPEN: 'host.network.websocket.open',
  WEBSOCKET_RECEIVE: 'host.network.websocket.receive',
  WEBSOCKET_SEND: 'host.network.websocket.send',
  WEBSOCKET_CLOSE: 'host.network.websocket.close',
} as const;

export interface NetworkRequestInput {
  method?: string;
  url: string;
  headers?: Record<string, string>;
  bodyBase64?: string;
  timeoutMs?: number;
  maxResponseBytes?: number;
}

export interface NetworkRequestOutput {
  statusCode: number;
  headers: Record<string, string[]>;
  bodyBase64: string;
  finalUrl: string;
}

export interface NetworkSocketOpenInput {
  target: string;
  port: number;
  timeoutMs?: number;
}

export interface NetworkSocketOpenOutput {
  handleId: string;
  transport: string;
  localAddress?: string;
  remoteAddress?: string;
}

export interface NetworkSocketReadInput {
  handleId: string;
  maxBytes?: number;
  timeoutMs?: number;
}

export interface NetworkSocketReadOutput {
  dataBase64: string;
  bytesRead: number;
  eof?: boolean;
}

export interface NetworkSocketWriteInput {
  handleId: string;
  dataBase64: string;
  timeoutMs?: number;
}

export interface NetworkSocketWriteOutput {
  bytesWritten: number;
}

export interface NetworkSocketCloseInput {
  handleId: string;
}

export interface NetworkSocketCloseOutput {
  closed: boolean;
}

export interface NetworkWebSocketOpenInput {
  url: string;
  headers?: Record<string, string>;
  subprotocols?: string[];
  timeoutMs?: number;
}

export interface NetworkWebSocketOpenOutput {
  handleId: string;
  subprotocol?: string;
  remoteAddress?: string;
}

export interface NetworkWebSocketSendInput {
  handleId: string;
  messageType?: 'text' | 'binary';
  dataBase64: string;
  timeoutMs?: number;
}

export interface NetworkWebSocketSendOutput {
  bytesWritten: number;
}

export interface NetworkWebSocketReceiveInput {
  handleId: string;
  timeoutMs?: number;
}

export interface NetworkWebSocketReceiveOutput {
  messageType: 'text' | 'binary';
  dataBase64: string;
  bytesRead: number;
}

async function invokeNetwork<T>(
  client: Client,
  method: string,
  input: unknown,
  opts: MessageOption[] = [],
): Promise<T> {
  const result = await invokeHostAPI(client, { method, version: 1, input }, opts);
  if (result.status !== HOST_API_STATUS_SUCCESS) {
    throw new Error(`Game Plugin SDK: ${method} returned host API status '${result.status}'`);
  }
  return result.output as T;
}

export function networkRequest(client: Client, input: NetworkRequestInput, opts: MessageOption[] = []): Promise<NetworkRequestOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.REQUEST, input, opts);
}

export function networkTCPOpen(client: Client, input: NetworkSocketOpenInput, opts: MessageOption[] = []): Promise<NetworkSocketOpenOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.TCP_OPEN, input, opts);
}

export function networkTCPRead(client: Client, input: NetworkSocketReadInput, opts: MessageOption[] = []): Promise<NetworkSocketReadOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.TCP_READ, input, opts);
}

export function networkTCPWrite(client: Client, input: NetworkSocketWriteInput, opts: MessageOption[] = []): Promise<NetworkSocketWriteOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.TCP_WRITE, input, opts);
}

export function networkTCPClose(client: Client, input: NetworkSocketCloseInput, opts: MessageOption[] = []): Promise<NetworkSocketCloseOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.TCP_CLOSE, input, opts);
}

export function networkUDPOpen(client: Client, input: NetworkSocketOpenInput, opts: MessageOption[] = []): Promise<NetworkSocketOpenOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.UDP_OPEN, input, opts);
}

export function networkUDPReceive(client: Client, input: NetworkSocketReadInput, opts: MessageOption[] = []): Promise<NetworkSocketReadOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.UDP_RECEIVE, input, opts);
}

export function networkUDPSend(client: Client, input: NetworkSocketWriteInput, opts: MessageOption[] = []): Promise<NetworkSocketWriteOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.UDP_SEND, input, opts);
}

export function networkUDPClose(client: Client, input: NetworkSocketCloseInput, opts: MessageOption[] = []): Promise<NetworkSocketCloseOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.UDP_CLOSE, input, opts);
}

export function networkWebSocketOpen(client: Client, input: NetworkWebSocketOpenInput, opts: MessageOption[] = []): Promise<NetworkWebSocketOpenOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.WEBSOCKET_OPEN, input, opts);
}

export function networkWebSocketReceive(client: Client, input: NetworkWebSocketReceiveInput, opts: MessageOption[] = []): Promise<NetworkWebSocketReceiveOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.WEBSOCKET_RECEIVE, input, opts);
}

export function networkWebSocketSend(client: Client, input: NetworkWebSocketSendInput, opts: MessageOption[] = []): Promise<NetworkWebSocketSendOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.WEBSOCKET_SEND, input, opts);
}

export function networkWebSocketClose(client: Client, input: NetworkSocketCloseInput, opts: MessageOption[] = []): Promise<NetworkSocketCloseOutput> {
  return invokeNetwork(client, HOST_NETWORK_METHOD.WEBSOCKET_CLOSE, input, opts);
}
