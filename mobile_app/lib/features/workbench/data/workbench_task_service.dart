import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/backend_transport/backend_service_api.dart';
import '../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../core/services/owned_conversation_reference.dart';

final workbenchTaskServiceProvider = Provider<WorkbenchTaskService>((ref) {
  final api = ref.watch(rawBackendServiceApiProvider);
  if (api == null) throw StateError('任务执行 Core 尚未就绪');
  var current = true;
  ref.onDispose(() {
    current = false;
  });
  return WorkbenchTaskService(
    api,
    isCurrent: () =>
        current && identical(ref.read(rawBackendServiceApiProvider), api),
  );
});

class WorkbenchTaskService {
  WorkbenchTaskService(this.api, {required this.isCurrent});
  final BackendServiceApi api;
  final bool Function() isCurrent;

  Future<Map<String, dynamic>> read(
    String conversationId,
    String characterId, {
    String? coordinationId,
  }) async {
    try {
      return await _read(
        conversationId,
        characterId,
        coordinationId: coordinationId,
      );
    } on ServiceApiException catch (error) {
      throw StateError(switch (error.code) {
        401 || 403 => '没有当前会话任务的访问权限，请检查设备授权',
        404 => '当前执行 Core 下没有此会话或任务，请确认原执行 Core',
        409 => '会话归属或执行 Core 已变化，请重新加载',
        _ => '任务所属执行服务或数据所有者暂不可用，请检查连接',
      });
    }
  }

  Future<Map<String, dynamic>> _read(
    String conversationId,
    String characterId, {
    String? coordinationId,
  }) async {
    final origin = parseConversationReference(conversationId);
    if (origin == null || characterId.isEmpty) {
      throw StateError('无法确认任务所属会话和角色，请重新打开对话');
    }
    if (!isCurrent()) throw StateError('执行 Core 已变化，请重新加载');
    final state = await api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    final coreId = state?['coreId'];
    if (!isCurrent() || coreId is! String || coreId.isEmpty) {
      throw StateError('任务执行 Core 尚未就绪或已变化');
    }
    final parameters = <String, dynamic>{
      'conversationId': origin['id'],
      'conversationOwnerId': origin['ownerId'],
      'characterId': characterId,
      'expectedCoreId': coreId,
    };
    if (coordinationId != null) parameters['coordinationId'] = coordinationId;
    final response = await api.get<Map<String, dynamic>>(
      '/api/workbench/tasks${coordinationId == null ? '' : '/checkpoints'}',
      queryParameters: parameters,
    );
    final context = response?['queryContext'];
    final scope = context is Map ? context['executionScope'] : null;
    final source = context is Map ? context['conversationOrigin'] : null;
    final policy = state?['policy'];
    if (policy is! Map ||
        scope is! Map ||
        scope['providerEpoch'] != policy['providerEpoch'] ||
        scope['modeRevision'] != policy['modeRevision'] ||
        scope['permissionRevision'] != policy['permissionRevision'] ||
        scope['coordinated'] != policy['coordinated']) {
      throw StateError('任务响应权限版本已变化，请重新加载');
    }
    if (!isCurrent() ||
        scope['coreId'] != coreId ||
        scope['roleId'] != characterId ||
        scope['spaceId'] is! String ||
        scope['resourceOwnerId'] is! String ||
        source is! Map ||
        source['ownerId'] != origin['ownerId'] ||
        source['id'] != origin['id'] ||
        context['conversationId'] is! String ||
        response?['conversationId'] != context['conversationId']) {
      throw StateError('任务响应归属已变化，请重新加载');
    }
    return {...response!, 'conversationId': conversationId};
  }
}
