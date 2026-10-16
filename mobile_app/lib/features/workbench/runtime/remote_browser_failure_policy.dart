bool remoteBrowserFailureIsTerminal(int? statusCode, {String? errorCode, bool? retryable}) {
  if (const {400, 401, 403, 404, 410, 501}.contains(statusCode)) return true;
  if (const {'capability_revoked', 'access_denied', 'invalid_request', 'unsupported_operation'}.contains(errorCode)) return true;
  return retryable == false && errorCode != null && errorCode.isNotEmpty;
}
