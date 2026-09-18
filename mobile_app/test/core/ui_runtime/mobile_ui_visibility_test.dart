import 'package:amitia_app/core/ui_runtime/mobile_ui_visibility.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

UIContributionSnapshotEntry _entry(Map<String, dynamic> visibility) {
  return UIContributionSnapshotEntry(
    contributionId: 'entry.test',
    extensionId: 'extension.test',
    moduleId: 'module.test',
    kind: 'panel',
    slotId: 'test.slot',
    contractVersion: 1,
    entryType: 'schema_renderer',
    entryPath: '',
    permissions: const <String>[],
    dataContract: const <String, dynamic>{},
    visibility: visibility,
    ordering: 0,
    priority: 0,
  );
}

void main() {
  test('filters contributions by platform', () {
    final entry = _entry(const <String, dynamic>{
      'platforms': <String>['android'],
    });

    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{},
        platform: 'android',
      ),
      isTrue,
    );
    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{},
        platform: 'ios',
      ),
      isFalse,
    );
  });

  test('required context checks presence instead of non-null value', () {
    final entry = _entry(const <String, dynamic>{
      'required_context': <String>['message.payload'],
    });

    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{
          'message': <String, dynamic>{'payload': null},
        },
        platform: 'android',
      ),
      isTrue,
    );
    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{'message': <String, dynamic>{}},
        platform: 'android',
      ),
      isFalse,
    );
  });

  test('supports the desktop portable condition operators', () {
    final entry = _entry(const <String, dynamic>{
      'conditions': <Map<String, dynamic>>[
        <String, dynamic>{
          'field': 'route',
          'operator': 'not_in',
          'value': <String>['/blocked'],
        },
        <String, dynamic>{
          'field': 'message.text',
          'operator': 'contains',
          'value': 'Amitia',
        },
      ],
    });

    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{
          'route': '/chat',
          'message': <String, dynamic>{'text': 'Hello Amitia'},
        },
        platform: 'android',
      ),
      isTrue,
    );
    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{
          'route': '/blocked',
          'message': <String, dynamic>{'text': 'Hello Amitia'},
        },
        platform: 'android',
      ),
      isFalse,
    );
  });

  test('unknown visibility operators fail closed', () {
    final entry = _entry(const <String, dynamic>{
      'conditions': <Map<String, dynamic>>[
        <String, dynamic>{
          'field': 'route',
          'operator': 'unknown_operator',
          'value': '/chat',
        },
      ],
    });

    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{'route': '/chat'},
        platform: 'android',
      ),
      isFalse,
    );
  });

  test('message type accepts the same top-level context keys as desktop', () {
    final entry = _entry(const <String, dynamic>{
      'message_types': <String>['image'],
    });

    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{'messageType': 'image'},
        platform: 'android',
      ),
      isTrue,
    );
    expect(
      matchesMobileUIContributionVisibility(
        entry,
        const <String, dynamic>{'type': 'image'},
        platform: 'android',
      ),
      isTrue,
    );
  });
}
