import 'package:amitia_app/core/ui_runtime/mobile_dynamic_runtime.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

UIContributionSnapshotEntry _entry(
  String id, {
  int ordering = 0,
  int priority = 0,
  String kind = 'panel',
  String entryKey = '',
  String cellId = '',
  String? runtimePackageId,
  List<Map<String, dynamic>> matchRules = const <Map<String, dynamic>>[],
}) {
  return UIContributionSnapshotEntry(
    contributionId: id,
    extensionId: 'ext.$id',
    moduleId: 'module.$id',
    kind: kind,
    slotId: 'test.slot',
    contractVersion: 1,
    entryType: 'schema_renderer',
    entryPath: '',
    permissions: const <String>[],
    dataContract: const <String, dynamic>{},
    visibility: const <String, dynamic>{},
    ordering: ordering,
    priority: priority,
    entryKey: entryKey,
    cellId: cellId,
    runtimePackageId: runtimePackageId,
    matchRules: matchRules,
  );
}

UISlotSnapshotEntry _slot({
  String? kind,
  String multiplicity = 'ordered_multiple',
  List<String> supportedKinds = const <String>['panel'],
}) {
  return UISlotSnapshotEntry(
    slotId: 'test.slot',
    contractVersion: 1,
    supportedKinds: supportedKinds,
    kind: kind,
    multiplicity: multiplicity,
    layout: 'stack',
    fallbackPolicy: 'empty',
    declarationEpoch: 0,
    scope: 'root',
    dynamicSlot: false,
    contributions: const <UIContributionSnapshotEntry>[],
  );
}

void main() {
  group('UI snapshot dispatch metadata', () {
    test('parses kind, supportedKinds and contribution dispatch metadata', () {
      final slot = UISlotSnapshotEntry.fromJson({
        'slotId': 'test.slot',
        'contractVersion': 2,
        'supportedKinds': ['panel'],
        'kind': 'keyed',
        'multiplicity': 'ordered_multiple',
        'layout': 'stack',
        'fallbackPolicy': 'empty',
        'contributions': [
          {
            'contribution_id': 'entry.a',
            'extension_id': 'ext.a',
            'module_id': 'module.a',
            'kind': 'panel',
            'slot': {'slot_id': 'test.slot'},
            'contract_version': 2,
            'entry': {'type': 'schema_renderer', 'path': ''},
            'ordering': {'priority': 7, 'sort_key': 'fallback-key'},
            'dispatch': {
              'entry_key': 'alpha',
              'cell_id': 'cell-a',
              'matched': {'route': 'matched'},
            },
            'permissions': [],
          },
        ],
      });

      expect(slot.kind, 'keyed');
      expect(slot.supportedKinds, ['panel']);
      expect(slot.contributions.single.entryKey, 'alpha');
      expect(slot.contributions.single.cellId, 'cell-a');
      expect(slot.contributions.single.matched, {'route': 'matched'});
    });
  });

  group('MobileDynamicRuntime strict dispatch', () {
    test('single uses lower strict priority', () {
      final result = MobileDynamicRuntime.resolveSlot(
        slot: _slot(kind: 'single'),
        server: [_entry('high', priority: 9), _entry('low', priority: 1)],
        dynamic: const [],
      );
      expect(result.single.contributionId, 'low');
    });

    test('single rejects equal strict priorities', () {
      expect(
        () => MobileDynamicRuntime.resolveSlot(
          slot: _slot(kind: 'single'),
          server: [_entry('a', priority: 1), _entry('b', priority: 1)],
          dynamic: const [],
        ),
        throwsStateError,
      );
    });

    test('list shadows per cell and supports listOnly', () {
      final entries = [
        _entry('a-old', priority: 8, ordering: 1, cellId: 'a'),
        _entry('a-new', priority: 2, ordering: 9, cellId: 'a'),
        _entry('b', priority: 3, ordering: 3, cellId: 'b'),
      ];
      final all = MobileDynamicRuntime.resolveSlot(
        slot: _slot(kind: 'list'),
        server: entries,
        dynamic: const [],
      );
      expect(all.map((item) => item.contributionId), ['b', 'a-new']);

      final onlyA = MobileDynamicRuntime.resolveSlot(
        slot: _slot(kind: 'list'),
        server: entries,
        dynamic: const [],
        listOnly: 'a',
      );
      expect(onlyA.single.contributionId, 'a-new');
    });

    test('keyed requires a dispatch key and shadows within the key', () {
      final entries = [
        _entry('alpha-old', priority: 8, entryKey: 'alpha'),
        _entry('alpha-new', priority: 2, entryKey: 'alpha'),
        _entry('beta', priority: 1, entryKey: 'beta'),
      ];
      expect(
        MobileDynamicRuntime.resolveSlot(
          slot: _slot(kind: 'keyed'),
          server: entries,
          dynamic: const [],
        ),
        isEmpty,
      );
      final result = MobileDynamicRuntime.resolveSlot(
        slot: _slot(kind: 'keyed'),
        server: entries,
        dynamic: const [],
        dispatchKey: 'alpha',
      );
      expect(result.single.contributionId, 'alpha-new');
    });

    test('chain skips unmatched declarative client entries', () {
      final server = _entry('server', priority: 5);
      final dynamic = _entry(
        'dynamic',
        priority: 1,
        runtimePackageId: 'runtime.package',
        matchRules: const [
          {'field': 'message.type', 'operator': 'eq', 'value': 'image'},
        ],
      );

      final textResult = MobileDynamicRuntime.resolveSlot(
        slot: _slot(kind: 'chain'),
        server: [server],
        dynamic: [dynamic],
        owner: const {
          'message': {'type': 'text'},
        },
      );
      expect(textResult.single.contributionId, 'server');

      final imageResult = MobileDynamicRuntime.resolveSlot(
        slot: _slot(kind: 'chain'),
        server: [server],
        dynamic: [dynamic],
        owner: const {
          'message': {'type': 'image'},
        },
      );
      expect(imageResult.single.contributionId, 'dynamic');
    });

    test('filters contribution kinds not supported by the slot', () {
      final result = MobileDynamicRuntime.resolveSlot(
        slot: _slot(kind: 'single', supportedKinds: const ['panel']),
        server: [_entry('renderer', kind: 'renderer')],
        dynamic: [_entry('panel', kind: 'panel', runtimePackageId: 'pkg')],
      );
      expect(result.single.contributionId, 'panel');
    });
  });

  test('legacy replaceable_single keeps historical higher-priority behavior', () {
    final result = MobileDynamicRuntime.resolveSlot(
      slot: _slot(kind: null, multiplicity: 'replaceable_single'),
      server: [_entry('low', priority: 1), _entry('high', priority: 9)],
      dynamic: const [],
    );
    expect(result.single.contributionId, 'high');
  });
}
