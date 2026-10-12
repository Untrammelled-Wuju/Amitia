import 'package:amitia_app/core/ui_runtime/mobile_style_runtime.dart';
import 'package:amitia_app/features/settings/presentation/pages/device_pair_scan_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('registers page and container style targets', () {
    registerDevicePairScanStyleTargets();

    for (final target in <String>[
      'device.pair.scan.page',
      'device.pair.scan.viewfinder',
      'device.pair.scan.hint',
    ]) {
      expect(AmitiaStyleTargetRegistry.descriptor(target), isNotNull);
    }
    expect(
      AmitiaStyleTargetRegistry.descriptor(
        'device.pair.scan.hint',
      )!.properties.containsKey('backgroundColor'),
      isTrue,
    );
  });

  testWidgets('device pair scan hint applies structured styles', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'scan-hint',
          scope: const AmitiaStyleScopeData(
            platform: 'mobile',
            page: 'settings.devices',
          ),
          target: 'device.pair.scan.hint',
          properties: const <String, dynamic>{
            'backgroundColor': '#123456',
            'foregroundColor': '#abcdef',
            'borderRadius': 18,
          },
        ),
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: AmitiaStyleScopeHost(
          scope: const AmitiaStyleScopeData(
            platform: 'mobile',
            page: 'settings.devices',
          ),
          child: AmitiaStyleDocumentHost(
            document: document,
            child: const DevicePairScanPage(),
          ),
        ),
      ),
    );
    await tester.pump();

    final card = tester.widget<Card>(find.byType(Card));
    expect(card.color, const Color(0xFF123456));
    final hint = tester.widget<Text>(
      find.text('对准另一台设备的 Amitia 配对二维码。识别后会校验服务地址和设备身份。'),
    );
    expect(hint.style?.color, const Color(0xFFABCDEF));
    final shape = card.shape;
    expect(shape, isA<RoundedRectangleBorder>());
    expect(
      (shape as RoundedRectangleBorder).borderRadius,
      BorderRadius.circular(18),
    );
  });
}
