enum BackendHttpMethod {
  get('GET'),
  post('POST'),
  put('PUT'),
  patch('PATCH'),
  delete('DELETE'),
  head('HEAD');

  final String value;
  const BackendHttpMethod(this.value);
}
