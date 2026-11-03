import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('Agent OS and extension marketplace routes are real routes', () {
    expect(AppRoutes.agentOS, '/agent-os');
    expect(AppRoutes.extensionsMarket, '/extensions/market');
    expect(isProtectedProviderRoutePath(AppRoutes.agentOS), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.extensionsMarket), isTrue);
  });
}
