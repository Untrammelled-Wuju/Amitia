import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/backend_transport/backend_service_api.dart';
import '../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../core/services/owned_conversation_reference.dart';

class WorkbenchTaskReadFailure extends StateError {
  WorkbenchTaskReadFailure(super.message, {this.invalidatesSnapshot = false});
  final bool invalidatesSnapshot;
  @override
  String toString() => message.toString();
}

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

  Future<Map<String, dynamic>> readBudget(
    String conversationId,
    String characterId, {
    required String coordinationId,
    required String parentInteractionId,
    required String kind,
    Map<String, dynamic> cursor = const {},
  }) => read(conversationId, characterId, coordinationId: coordinationId,
      budgetQuery: {'parentInteractionId': parentInteractionId, 'kind': kind, ...cursor});

  Future<Map<String, dynamic>> registerBudgetEvidence(
    String conversationId,
    String characterId, {
    required String coordinationId,
    required String parentInteractionId,
    required String kind,
    required String recordId,
    required String evidenceRef,
  }) => read(conversationId, characterId, coordinationId: coordinationId,
      budgetQuery: {'parentInteractionId': parentInteractionId},
      registration: {'kind': kind, 'recordId': recordId, 'evidenceRef': evidenceRef});

  Future<Map<String, dynamic>> read(
    String conversationId,
    String characterId, {
    String? coordinationId,
    Map<String, dynamic>? budgetQuery,
    Map<String, dynamic>? registration,
  }) async {
    try {
      return await _read(
        conversationId,
        characterId,
        coordinationId: coordinationId,
        budgetQuery: budgetQuery,
        registration: registration,
      );
    } on StateError catch (error) {
      throw WorkbenchTaskReadFailure(error.message.toString(), invalidatesSnapshot: true);
    } on ServiceApiException catch (error) {
      throw WorkbenchTaskReadFailure(
        error.message,
        invalidatesSnapshot: const {401, 403, 404, 409}.contains(error.code),
      );
    }
  }

  Future<Map<String, dynamic>> _read(
    String conversationId,
    String characterId, {
    String? coordinationId,
    Map<String, dynamic>? budgetQuery,
    Map<String, dynamic>? registration,
  }) async {
    Map<String, String>? origin;
    try {
      origin = parseConversationReference(conversationId);
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
throw StateError(error.toString());
    }
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
    final initialPolicy = state?['policy'];
    if (!workbenchPolicyIsValid(initialPolicy)) {
      throw StateError('任务执行权限不可确认，请重新加载');
    }
    final initialAuthority = workbenchPolicyAuthority(initialPolicy);
    final initialSelectedRole = (initialPolicy as Map)['selectedRole'];
    final parameters = <String, dynamic>{
      'conversationId': origin['id'],
      'conversationOwnerId': origin['ownerId'],
      'characterId': characterId,
      'expectedCoreId': coreId,
    };
    if (coordinationId != null) parameters['coordinationId'] = coordinationId;
    if (budgetQuery != null) parameters.addAll(budgetQuery);
    final path = '/api/workbench/tasks${budgetQuery != null ? '/budget-reconciliation${registration != null ? '/review' : ''}' : coordinationId == null ? '' : '/checkpoints'}';
    final response = registration == null
        ? await api.get<Map<String, dynamic>>(path, queryParameters: parameters)
        : await api.post<Map<String, dynamic>>(path, queryParameters: parameters, data: registration);
    if (!isCurrent()) throw StateError('执行 Core 已变化，请重新加载');
    final latest = await api.get<Map<String, dynamic>>('/api/device-mesh/v1/coordination/me');
    if (!isCurrent() || latest?['coreId'] != coreId) throw StateError('执行 Core 已变化，请重新加载');
    final context = response?['queryContext'];
    final scope = context is Map ? context['executionScope'] : null;
    final source = context is Map ? context['conversationOrigin'] : null;
    final policy = latest?['policy'];
    if (!workbenchPolicyIsValid(policy) ||
        workbenchPolicyAuthority(policy) != initialAuthority ||
        (policy as Map)['selectedRole'] != initialSelectedRole ||
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


bool workbenchPolicyIsValid(Object? input) {
  if (input is! Map) return false;
  for (final name in const ['providerEpoch', 'modeRevision', 'permissionRevision']) {
    final number = input[name];
    if (number is! int || number < 0) return false;
  }
  return input['coordinated'] is bool;
}

String workbenchPolicyAuthority(Object? input) {
  if (!workbenchPolicyIsValid(input)) throw StateError('任务执行权限不可确认，请重新加载');
  final policy = input as Map;
  return '${policy['providerEpoch']}:${policy['modeRevision']}:${policy['permissionRevision']}:${policy['coordinated']}';
}
