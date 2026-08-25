import { Envelope } from './protocol';

export interface Transport {
  send(message: Envelope): Promise<void>;
  receive(): Promise<Envelope>;
  close(): Promise<void>;
  // Optional fast path used by stdio transports. Other transports can omit it
  // and the SDK falls back to bounded JSON/base64 binary.write requests.
  sendBinaryFrame?(message: Envelope, objectId: string, offset: number, data: Uint8Array): Promise<void>;
}
