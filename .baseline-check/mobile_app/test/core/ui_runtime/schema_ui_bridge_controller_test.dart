import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/ui_runtime/schema_ui_bridge_controller.dart';
import 'package:amitia_app/features/extensions/schema_ui/engine/action_dispatcher.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeBackendServiceApi extends Fake implements BackendServiceApi {
  @override
  int get generation => 1;
}

class _FakeExtensionService extends ExtensionService {
  _FakeExtensionService() : super(_FakeBackendServiceApi());

  int createCount = 0;
  int invokeCount = 0;
  final List<String> revoked = <String>[];
  final List<String> conversations = <String>[];
  final List<String> invokedActions = <String>[];

  @override
  Future<Map<String, dynamic>> createSchemaUISession({
    required String contributionId,
    String characterId = '',
    String conversationId = '',
    String surface = 'mobile',
  }) async {
    createCount++;
    conversations.add(conversationId);
    return <String, dynamic>{
      'sessionId': 'session-$createCount',
      'origin': 'amitia://schema-ui',
      'contractVersion': 3,
    };
  }

  @override
  Future<Map<String, dynamic>> invokeSchemaUIBridge(
    String sessionId, {
    required String contributionId,
    required String origin,
    required int contractVersion,
    required String actionId,
    Map<String, dynamic> input = const <String, dynamic>{},
  }) async {
    invokeCount++;
    invokedActions.add('$sessionId:$actionId:$contractVersion');
    return <String, dynamic>{'ok': true, 'message': 'done'};
  }

  @override
  Future<void> revokeSchemaUISession(String sessionId) async {
    revoked.add(sessionId);
  }
}

ActionInvocation _invocation(String id) => ActionInvocation(
      actionId: id,
      target: 'invoke_capability',
      input: const <String, dynamic>{'value': 1},
      extensionId: 'ext.test',
      contributionId: 'contribution.test',
      ownerIdentity: const <String, dynamic>{},
    );

void main() {
  test('local host action bypasses extension bridge', () async {
    final service = _FakeExtensionService();
    final controller = SchemaUIBridgeController(service);
    var called = 0;

    final result = await controller.dispatch(
      _invocation('local.action'),
      contributionId: 'contribution.test',
      localActions: {
        'local.action': (input) {
          called++;
          return <String, dynamic>{'local': input};
        },
      },
    );

    expect(called, 1);
    expect(result, isA<Map>());
    expect(service.createCount, 0);
    expect(service.invokeCount, 0);
    await controller.dispose();
  });

  test('remote action creates and reuses a schema UI bridge session', () async {
    final service = _FakeExtensionService();
    final controller = SchemaUIBridgeController(service);

    await controller.dispatch(
      _invocation('remote.one'),
      contributionId: 'contribution.test',
      contractVersion: 1,
      conversationId: 'conversation-a',
    );
    await controller.dispatch(
      _invocation('remote.two'),
      contributionId: 'contribution.test',
      contractVersion: 1,
      conversationId: 'conversation-a',
    );

    expect(service.createCount, 1);
    expect(service.invokeCount, 2);
    expect(service.invokedActions, [
      'session-1:remote.one:3',
      'session-1:remote.two:3',
    ]);
    await controller.dispose();
    expect(service.revoked, contains('session-1'));
  });

  test('scope change revokes the old session before creating another', () async {
    final service = _FakeExtensionService();
    final controller = SchemaUIBridgeController(service);

    await controller.dispatch(
      _invocation('remote.one'),
      contributionId: 'contribution.test',
      conversationId: 'conversation-a',
    );
    await controller.dispatch(
      _invocation('remote.two'),
      contributionId: 'contribution.test',
      conversationId: 'conversation-b',
    );

    expect(service.createCount, 2);
    expect(service.conversations, ['conversation-a', 'conversation-b']);
    expect(service.revoked, contains('session-1'));
    await controller.dispose();
  });
  test('contract version change starts a fresh bridge session', () async {
    final service = _FakeExtensionService();
    final controller = SchemaUIBridgeController(service);

    await controller.dispatch(
      _invocation('remote.one'),
      contributionId: 'contribution.test',
      contractVersion: 1,
      conversationId: 'conversation-a',
    );
    await controller.dispatch(
      _invocation('remote.two'),
      contributionId: 'contribution.test',
      contractVersion: 2,
      conversationId: 'conversation-a',
    );

    expect(service.createCount, 2);
    expect(service.revoked, contains('session-1'));
    await controller.dispose();
  });

}
