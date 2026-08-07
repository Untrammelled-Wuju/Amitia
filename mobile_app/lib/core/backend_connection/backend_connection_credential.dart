class BackendConnectionCredential {
  final String _localToken;

  BackendConnectionCredential._(this._localToken);

  String revealForTransport() => _localToken;

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
