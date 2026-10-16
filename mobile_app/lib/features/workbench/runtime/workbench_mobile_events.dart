import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../core/device_mesh/mobile_cloud_device_credential_store.dart';

import '../../chat/runtime/agent_event_reducer.dart';

String? resolveMobileWorkbenchBrowserTarget({
  required String originDeviceId,
  required String explicitTarget,
  required bool cloudMode,
  required String localDeviceId,
}) {
  if (explicitTarget.isNotEmpty) {
    return RegExp(r'^[a-zA-Z0-9_:.-]{1,140}$').hasMatch(explicitTarget) ? explicitTarget : null;
  }
  if (originDeviceId.isEmpty) return '';
  if (!RegExp(r'^[a-zA-Z0-9_:.-]{1,140}$').hasMatch(originDeviceId)) return null;
  if (cloudMode) {
    if (localDeviceId.isEmpty) return null;
    return originDeviceId == localDeviceId ? '' : originDeviceId;
  }
  return '';
}

class WorkbenchMobileEvents {
  static final ValueNotifier<OwnedWorkbenchTaskChange?> taskChanges = ValueNotifier<OwnedWorkbenchTaskChange?>(null);

  static void taskChanged(Map<String, dynamic> event, {required String coreId, required String characterId, required String conversationId}) {
    final scope = event['executionScope'];
    final task = event['task'];
    if (event['type'] != 'task' || scope is! Map || scope['coreId'] != coreId || scope['roleId'] != characterId || event['conversationId'] != conversationId || conversationId.isEmpty || task is! Map || task['coordinationId'] is! String || event['sourceApi'] == null) return;
    taskChanges.value = OwnedWorkbenchTaskChange(coreId: coreId, characterId: characterId, conversationId: conversationId, coordinationId: task['coordinationId'] as String, sourceApi: event['sourceApi'] as Object);
  }
  static final ValueNotifier<int> revision = ValueNotifier<int>(0);
  static String? lastChangedConversation;
  static Future<void> _writes = Future<void>.value();

  static String keyFor(String conversationId) => 'amitia:workbench:mobile:v1:${conversationId.isEmpty ? '__draft__' : conversationId}';

  static Future<String?> read(String conversationId) async {
    await _writes;
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(keyFor(conversationId));
  }

  static Future<void> save(String conversationId, List<Map<String, dynamic>> tabs, int index) {
    _writes = _writes.then((_) async {
      try {
        final prefs = await SharedPreferences.getInstance();
        final raw = prefs.getString(keyFor(conversationId));
        Map<String, dynamic> state = <String, dynamic>{};
        if (raw != null) {
          final parsed = jsonDecode(raw);
          if (parsed is Map) state = Map<String, dynamic>.from(parsed);
        }
        state['tabs'] = tabs;
        state['index'] = index;
        await prefs.setString(keyFor(conversationId), jsonEncode(state));
      } catch (_) {}
    });
    return _writes;
  }

  static Future<void> ingest(AgentUIEvent event) {
    if (event.type != 'workbench.open' || event.version != 1) return Future<void>.value();
    final conversation = event.conversationId.trim();
    final kind = (event.payload['kind'] ?? '').toString();
    final uri = (event.payload['resourceUri'] ?? '').toString().trim();
    final workspace = (event.payload['workspaceUri'] ?? '').toString().trim();
    final explicitTarget = (event.payload['targetDeviceId'] ?? '').toString().trim();
    final originDevice = (event.payload['originDeviceId'] ?? '').toString().trim();
    final taskId = kind == 'terminal' ? (event.payload['taskId'] ?? '').toString().trim() : '';
    if (taskId.isNotEmpty && !RegExp(r'^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$').hasMatch(taskId)) return Future<void>.value();
    final browserRef = kind == 'browser' ? (event.payload['browserRef'] ?? '').toString().trim() : '';
    if (browserRef.isNotEmpty && !RegExp(r'^[A-Za-z0-9_-]{32}$').hasMatch(browserRef)) return Future<void>.value();
    if (explicitTarget.isNotEmpty && !RegExp(r'^[a-zA-Z0-9_:.-]{1,140}$').hasMatch(explicitTarget)) return Future<void>.value();
    final operationId = (event.payload['operationId'] ?? event.eventId).toString();
    if (conversation.isEmpty || conversation.length > 160 || operationId.length > 140 || !{'files', 'file', 'git', 'diff', 'browser', 'terminal', 'tasks'}.contains(kind) || uri.length > 4096 || workspace.length > 4096 || explicitTarget.length > 140 || originDevice.length > 140) return Future<void>.value();
    if ((kind == 'file' || kind == 'diff') && (uri.isEmpty || workspace.isEmpty)) return Future<void>.value();
    if (kind == 'tasks' && (uri.isNotEmpty || workspace.isNotEmpty || explicitTarget.isNotEmpty || originDevice.isNotEmpty || browserRef.isNotEmpty || taskId.isNotEmpty)) return Future<void>.value();
    if (kind == 'browser') {
      final parsed = Uri.tryParse(uri);
      if (parsed == null || !{'http', 'https'}.contains(parsed.scheme) || parsed.host.isEmpty || parsed.userInfo.isNotEmpty) return Future<void>.value();
    }
    _writes = _writes.then((_) async {
      try {
        final prefs = await SharedPreferences.getInstance();
        String targetDevice = explicitTarget;
        if (kind == 'browser' && browserRef.isNotEmpty) {
          final mode = prefs.getString('deployment_mode');
          final cloudMode = {'cloud', 'remote', 'hybrid'}.contains(mode);
          String localDeviceId = '';
          if (cloudMode && originDevice.isNotEmpty && explicitTarget.isEmpty) {
            final credential = await const MobileCloudDeviceCredentialStore().load();
            if (credential == null || credential.isExpired) return;
            localDeviceId = credential.deviceId;
          }
          final resolved = resolveMobileWorkbenchBrowserTarget(
            originDeviceId: originDevice,
            explicitTarget: explicitTarget,
            cloudMode: cloudMode,
            localDeviceId: localDeviceId,
          );
          if (resolved == null) return;
          targetDevice = resolved;
        }
        final raw = prefs.getString(keyFor(conversation));
        Map<String, dynamic> state = <String, dynamic>{};
        if (raw != null) {
          final parsed = jsonDecode(raw);
          if (parsed is Map) state = Map<String, dynamic>.from(parsed);
        }
        final seen = (state['operations'] is List ? state['operations'] as List : <dynamic>[]).map((value) => value.toString()).toList();
        if (operationId.isNotEmpty && seen.contains(operationId)) return;
        final tabs = (state['tabs'] is List ? state['tabs'] as List : <dynamic>[]).whereType<Map>().map((tab) => Map<String, dynamic>.from(tab)).toList();
        final forceNew = event.payload['forceNew'] == true;
        int index = forceNew ? -1 : tabs.indexWhere((tab) => tab['type'] == kind && tab['uri'] == uri && tab['workspaceUri'] == workspace && tab['targetDeviceId'] == targetDevice && (tab['browserRef'] ?? '') == browserRef && (tab['taskId'] ?? '') == taskId);
        if (index < 0) {
          if (tabs.length >= 25) return;
          final title = (event.payload['title'] ?? '').toString().trim();
          final id = 'agent-${DateTime.now().microsecondsSinceEpoch}-${tabs.length}';
          tabs.add({
            'id': id,
            'type': kind,
            'title': title.isEmpty ? kind : title.substring(0, title.length > 140 ? 140 : title.length),
            'uri': uri,
            'workspaceUri': workspace,
            'targetDeviceId': targetDevice,
            'browserRef': browserRef,
            'taskId': taskId,
            'pinned': false,
          });
          index = tabs.length - 1;
        }
        final current = state['index'];
        state['tabs'] = tabs;
        state['index'] = event.payload['focus'] == 'activate' ? index : (current is int && current >= 0 && current < tabs.length ? current : 0);
        if (operationId.isNotEmpty) {
          seen.add(operationId);
          state['operations'] = seen.length > 200 ? seen.sublist(seen.length - 200) : seen;
        }
        await prefs.setString(keyFor(conversation), jsonEncode(state));
        lastChangedConversation = conversation;
        revision.value += 1;
      } catch (_) {}
    });
    return _writes;
  }
}

class OwnedWorkbenchTaskChange {
  const OwnedWorkbenchTaskChange({required this.coreId, required this.characterId, required this.conversationId, required this.coordinationId, required this.sourceApi});
  final String coreId;
  final String characterId;
  final String conversationId;
  final String coordinationId;
  final Object sourceApi;

  bool matches({required String conversationId, required String characterId, required Object? api}) => this.conversationId == conversationId && this.characterId == characterId && identical(sourceApi, api);
}
