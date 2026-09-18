import type { Envelope, RequestHandler } from '@amitia/game-plugin-sdk';

interface EchoPayload {
  message?: string;
}

interface IncrementPayload {
  delta?: number;
}

interface GeneratePayload {
  prompt?: string;
}

interface SetStatePayload {
  stateId?: string;
  value?: unknown;
}

interface GetStatePayload {
  stateId?: string;
}

interface StorePayload {
  binaryId?: string;
  data?: string;
}

interface StreamAppendPayload {
  streamId?: string;
  data?: string;
}

interface ChannelSendPayload {
  channelId?: string;
  message?: unknown;
}

export class CoreService {
  private counter: number = 0;

  readonly handleEcho: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as EchoPayload) || {};
    const message = payload.message || '';
    this.counter++;
    return { message, count: this.counter };
  };

  readonly handleIncrement: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as IncrementPayload) || {};
    const delta = payload.delta || 1;
    this.counter += delta;
    return { counter: this.counter };
  };

  readonly handleGenerate: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as GeneratePayload) || {};
    const prompt = payload.prompt || '';
    return { result: `echo: ${prompt}`, tokens: prompt.length };
  };
}

export class StateService {
  private states: Map<string, unknown> = new Map();

  readonly handleSetState: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as SetStatePayload) || {};
    const stateId = payload.stateId || 'default';
    this.states.set(stateId, payload.value);
    return { acked: true, stateId };
  };

  readonly handleGetState: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as GetStatePayload) || {};
    const stateId = payload.stateId || 'default';
    const value = this.states.get(stateId);
    return { stateId, found: value !== undefined, value };
  };
}

export class DataService {
  private binaryCount: number = 0;

  readonly handleStoreBinary: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as StorePayload) || {};
    this.binaryCount++;
    return {
      stored: true,
      binaryId: payload.binaryId,
      size: (payload.data || '').length,
      totalBinaries: this.binaryCount,
    };
  };

  readonly handleStreamAppend: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as StreamAppendPayload) || {};
    return {
      appended: true,
      streamId: payload.streamId,
      sequence: Date.now(),
    };
  };

  readonly handleChannelSend: RequestHandler = async (request: Envelope): Promise<unknown> => {
    const payload = (request.payload as ChannelSendPayload) || {};
    return {
      sent: true,
      channelId: payload.channelId,
    };
  };
}
