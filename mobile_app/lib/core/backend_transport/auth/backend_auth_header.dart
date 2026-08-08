class BackendAuthHeader {
  static const String localToken = 'X-Amitia-Local-Token';
  static const String authorization = 'Authorization';
  static const String cookie = 'Cookie';
  static const String host = 'Host';
  static const String connection = 'Connection';
  static const String contentLength = 'Content-Length';

  static const Set<String> protectedHeaders = {
    localToken,
    authorization,
    cookie,
    host,
    connection,
    contentLength,
  };
}

class BackendBaseHeaders {
  static const String userAgent = 'User-Agent';
  static const String accept = 'Accept';
  static const String contentType = 'Content-Type';

  static const String userAgentValue = 'Amitia-Mobile';
  static const String acceptJsonValue = 'application/json';
  static const String contentTypeJsonValue = 'application/json';
}
