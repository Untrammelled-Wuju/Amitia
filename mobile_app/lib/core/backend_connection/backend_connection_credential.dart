class BackendConnectionCredential {
  final String _secret;

  BackendConnectionCredential._(this._secret);

  String revealForTransport() => _secret;

  @override
  String toString() => 'BackendConnectionCredential([REDACTED])';

  static BackendConnectionCredential? tryCreate(String token) {
    final trimmed = token.trim();
    if (trimmed.length < 32) return null;
    if (trimmed.contains('\u0000')) return null;
    if (trimmed.contains('\r')) return null;
    if (trimmed.contains('\n')) return null;
    return BackendConnectionCredential._(trimmed);
  }
}
