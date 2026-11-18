Uri buildMobileWebRuntimeResourceUri({
  required Uri base,
  required String providerId,
  required String path,
  required int generation,
  required String contentHash,
  required String moduleToken,
}) {
  final provider = providerId.trim();
  final resourcePath = path.trim();
  final hash = contentHash.trim();
  final token = moduleToken.trim();
  if (provider.isEmpty) {
    throw StateError('mobile_web_runtime providerId is required');
  }
  if (resourcePath.isEmpty) {
    throw StateError('mobile_web_runtime path is required');
  }
  if (hash.isEmpty) {
    throw StateError('mobile_web_runtime contentHash is required');
  }
  if (token.isEmpty) {
    throw StateError('mobile_web_runtime moduleToken is required');
  }
  final encodedPath = resourcePath
      .split('/')
      .where((segment) => segment.isNotEmpty)
      .map(Uri.encodeComponent)
      .join('/');
  return base.resolve(
    '/api/public/extensions/ui/provider-module/'
    '${Uri.encodeComponent(provider)}/$encodedPath'
    '?generation=$generation'
    '&integrity=${Uri.encodeQueryComponent(hash)}'
    '&accessToken=${Uri.encodeQueryComponent(token)}',
  );
}

Uri buildMobileWebRuntimeContributionUri({
  required Uri base,
  required String extensionId,
  required String contributionId,
  required String path,
  required int generation,
  required String contentHash,
  required String moduleToken,
}) {
  final extension = extensionId.trim();
  final contribution = contributionId.trim();
  final resourcePath = path.trim();
  final hash = contentHash.trim();
  final token = moduleToken.trim();
  if (extension.isEmpty) {
    throw StateError('mobile_web_runtime extensionId is required');
  }
  if (contribution.isEmpty) {
    throw StateError('mobile_web_runtime contributionId is required');
  }
  if (resourcePath.isEmpty) {
    throw StateError('mobile_web_runtime path is required');
  }
  if (hash.isEmpty) {
    throw StateError('mobile_web_runtime contentHash is required');
  }
  if (token.isEmpty) {
    throw StateError('mobile_web_runtime moduleToken is required');
  }
  final encodedPath = resourcePath
      .split('/')
      .where((segment) => segment.isNotEmpty)
      .map(Uri.encodeComponent)
      .join('/');
  return base.resolve(
    '/api/public/extensions/ui/contribution-module/'
    '${Uri.encodeComponent(extension)}/'
    '${Uri.encodeComponent(contribution)}/$encodedPath'
    '?generation=$generation'
    '&integrity=${Uri.encodeQueryComponent(hash)}'
    '&accessToken=${Uri.encodeQueryComponent(token)}',
  );
}
