import { Envelope } from './protocol';

export interface Transport {
  send(message: Envelope): Promise<void>;
  receive(): Promise<Envelope>;
  close(): Promise<void>;
}
