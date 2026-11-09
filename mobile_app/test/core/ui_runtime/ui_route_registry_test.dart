import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('open built-in shells accept child provider routes', () {
    expect(isProtectedProviderRoutePath(AppRoutes.extensions), isTrue);
    expect(
      isProtectedProviderRoutePath('${AppRoutes.extensions}/market'),
      isFalse,
    );
    expect(
      isProtectedProviderRoutePath('${AppRoutes.extensions}/third-party'),
      isFalse,
    );
    expect(isProtectedProviderRoutePath('/decision-viz'), isFalse);
    expect(isProtectedProviderRoutePath('/long-running'), isFalse);
  });

  test('closed built-in namespaces remain protected', () {
    expect(isProtectedProviderRoutePath('${AppRoutes.settings}/plugin'), isTrue);
    expect(isProtectedProviderRoutePath('${AppRoutes.memoryHub}/plugin'), isTrue);
    expect(isProtectedProviderRoutePath('${AppRoutes.gameCenter}/plugin'), isTrue);
  });
}
