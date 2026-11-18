import 'package:amitia_app/core/ui_runtime/mobile_web_runtime_uri.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('builds module and style resource URIs from the package root', () {
    final base = Uri.parse('http://127.0.0.1:18100');
    final module = buildMobileWebRuntimeResourceUri(
      base: base,
      providerId: 'dsh-compatible-page-provider',
      path: 'modules/dsh-compatible-ui/mobile/index.mjs',
      generation: 7,
      contentHash: 'sha256-module',
      moduleToken: 'token with space',
    );
    final style = buildMobileWebRuntimeResourceUri(
      base: base,
      providerId: 'dsh-compatible-page-provider',
      path: 'modules/dsh-compatible-ui/mobile/index.css',
      generation: 7,
      contentHash: 'sha256-style',
      moduleToken: 'token with space',
    );

    expect(
      module.path,
      '/api/public/extensions/ui/provider-module/'
      'dsh-compatible-page-provider/'
      'modules/dsh-compatible-ui/mobile/index.mjs',
    );
    expect(
      style.path,
      '/api/public/extensions/ui/provider-module/'
      'dsh-compatible-page-provider/'
      'modules/dsh-compatible-ui/mobile/index.css',
    );
    expect(style.path.contains('/mobile/modules/'), isFalse);
    expect(module.queryParameters['generation'], '7');
    expect(module.queryParameters['integrity'], 'sha256-module');
    expect(module.queryParameters['accessToken'], 'token with space');
  });

  test('rejects incomplete mobile web runtime resources', () {
    expect(
      () => buildMobileWebRuntimeResourceUri(
        base: Uri.parse('http://127.0.0.1:18100'),
        providerId: 'provider',
        path: '',
        generation: 1,
        contentHash: 'hash',
        moduleToken: 'token',
      ),
      throwsStateError,
    );
    expect(
      () => buildMobileWebRuntimeResourceUri(
        base: Uri.parse('http://127.0.0.1:18100'),
        providerId: 'provider',
        path: 'index.mjs',
        generation: 1,
        contentHash: '',
        moduleToken: 'token',
      ),
      throwsStateError,
    );
  });

  test('builds slot contribution resources with an independent capability', () {
    final uri = buildMobileWebRuntimeContributionUri(
      base: Uri.parse('http://127.0.0.1:18100'),
      extensionId: 'demo/plugin',
      contributionId: 'slot.mobile',
      path: 'modules/mobile/index.mjs',
      generation: 3,
      contentHash: 'sha256-module',
      moduleToken: 'contribution-token',
    );
    expect(
      uri.path,
      '/api/public/extensions/ui/contribution-module/'
      'demo%2Fplugin/slot.mobile/modules/mobile/index.mjs',
    );
    expect(uri.queryParameters['integrity'], 'sha256-module');
    expect(uri.queryParameters['accessToken'], 'contribution-token');
  });
}
