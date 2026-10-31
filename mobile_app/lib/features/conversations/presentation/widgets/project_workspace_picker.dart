import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/workspace_service.dart';

Future<WorkspaceMountDto?> pickAndRegisterProjectWorkspace(
  WidgetRef ref, {
  String dialogTitle = '选择工作目录',
}) async {
  if (defaultTargetPlatform == TargetPlatform.android) {
    final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
    final response = await dispatcher.execute(<String, dynamic>{
      'protocolVersion': 1,
      'requestId': 'project-workspace-${DateTime.now().microsecondsSinceEpoch}',
      'platform': 'android',
      'operation': 'workspace.saf.pick_tree',
      'payload': const <String, dynamic>{},
    });
    if ((response['status'] ?? '').toString() != 'success') {
      final rawError = response['error'];
      final error = rawError is Map
          ? Map<String, dynamic>.from(rawError)
          : const <String, dynamic>{};
      throw StateError((error['message'] ?? '系统目录授权失败').toString());
    }
    final rawResult = response['result'];
    final result = rawResult is Map
        ? Map<String, dynamic>.from(rawResult)
        : const <String, dynamic>{};
    if (result['cancelled'] == true) return null;
    final grantId = (result['grantId'] ?? '').toString().trim();
    if (grantId.isEmpty) throw StateError('系统目录授权未返回 grantId');
    return ref
        .read(workspaceServiceProvider)
        .registerSaf(
          name: (result['name'] ?? '工作目录').toString(),
          grantId: grantId,
          readOnly: result['readOnly'] == true,
        );
  }

  final path = await FilePicker.platform.getDirectoryPath(
    dialogTitle: dialogTitle,
  );
  final localRoot = path?.trim() ?? '';
  if (localRoot.isEmpty) return null;
  final normalized = localRoot.replaceAll('\\', '/');
  final segments = normalized
      .split('/')
      .where((segment) => segment.trim().isNotEmpty)
      .toList(growable: false);
  final name = segments.isEmpty ? '工作目录' : segments.last;
  return ref
      .read(workspaceServiceProvider)
      .registerLocal(name: name, localRoot: localRoot);
}
