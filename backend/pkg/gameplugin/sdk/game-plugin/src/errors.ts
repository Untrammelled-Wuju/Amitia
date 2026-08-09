export type SDKErrorCode =
  | 'protocol_error'
  | 'transport_error'
  | 'encode_error'
  | 'decode_error'
  | 'validation_error';

export class SDKError extends Error {
  code: SDKErrorCode;
  cause?: Error;

  constructor(code: SDKErrorCode, message: string, cause?: Error) {
    super(`sdk.${code}: ${message}`);
    this.name = 'SDKError';
    this.code = code;
    this.cause = cause;
  }
}

export function createProtocolError(message: string): SDKError {
  return new SDKError('protocol_error', message);
}

export function createTransportError(message: string): SDKError {
  return new SDKError('transport_error', message);
}

export function createEncodeError(message: string): SDKError {
  return new SDKError('encode_error', message);
}

export function createDecodeError(message: string): SDKError {
  return new SDKError('decode_error', message);
}

export function createValidationError(message: string): SDKError {
  return new SDKError('validation_error', message);
}
