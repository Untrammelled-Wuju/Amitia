import 'package:amitia_app/core/ui_runtime/plugin_surface_resolver.dart';
import 'package:amitia_app/core/ui_runtime/mobile_web_runtime_layout.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('parses mobile_web_runtime provider entries', () {
    final entry = UIProviderEntry.fromJson(<String, dynamic>{
      'contributionId': 'dsh-mobile-ui',
      'type': 'mobile_web_runtime',
      'path': 'modules/dsh-compatible-ui/mobile/index.mjs',
      'contentHash': 'sha256:module',
      'moduleToken': 'module-token',
      'stylePath': 'modules/dsh-compatible-ui/mobile/index.css',
      'styleHash': 'sha256:style',
    });
    expect(entry.type, UIProviderEntryType.mobileWebRuntime);
    expect(entry.path, 'modules/dsh-compatible-ui/mobile/index.mjs');
    expect(entry.contentHash, 'sha256:module');
    expect(entry.moduleToken, 'module-token');
    expect(entry.stylePath, 'modules/dsh-compatible-ui/mobile/index.css');
    expect(entry.styleHash, 'sha256:style');
  });

  test('resolves mobile_web_runtime as a web surface', () {
    expect(
      resolveMobilePluginSurface(
        kind: 'web_page',
        entryType: 'mobile_web_runtime',
      ),
      MobilePluginSurfaceKind.web,
    );
  });

  test('uses compact height for inline and badge web runtimes', () {
    expect(
      mobileWebRuntimeInitialHeight(<String, dynamic>{
        'slotLayout': 'inline',
        'slotId': 'extension.center.card.badge',
      }),
      1,
    );
    expect(
      mobileWebRuntimeInitialHeight(<String, dynamic>{
        'slotKind': 'status_item',
        'slotId': 'system.status.item',
      }),
      1,
    );
    expect(
      mobileWebRuntimeInitialHeight(<String, dynamic>{
        'slotLayout': 'stack',
        'slotId': 'extension.settings.page',
      }),
      320,
    );
  });
}
