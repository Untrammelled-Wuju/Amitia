import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';

class McpInteractionGuard extends ConsumerStatefulWidget {
  const McpInteractionGuard({super.key});

  @override
  ConsumerState<McpInteractionGuard> createState() => _McpInteractionGuardState();
}

class _McpInteractionGuardState extends ConsumerState<McpInteractionGuard> {
  Timer? _timer;
  bool _loading = false;
  bool _dialogOpen = false;
  bool _available = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _poll();
      _timer = Timer.periodic(const Duration(seconds: 2), (_) => _poll());
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _poll() async {
    if (!mounted || !_available || _loading || _dialogOpen) return;
    _loading = true;
    try {
      final loggedIn = await ref.read(authServiceProvider).isLoggedIn;
      if (!loggedIn || !mounted) return;
      final items = await ref.read(mcpServiceProvider).interactions();
      if (!mounted || items.isEmpty || _dialogOpen) return;
      _dialogOpen = true;
      await _showInteraction(items.first);
    } catch (error) {
      if (error.toString().contains('404')) {
        _available = false;
        _timer?.cancel();
        _timer = null;
      }
    } finally {
      _loading = false;
      _dialogOpen = false;
    }
  }

  Future<void> _showInteraction(Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty || !mounted) return;
    final kind = (item['kind'] ?? '').toString();
    final request = _asMap(item['request']);
    final serverName = (item['serverName'] ?? item['serverId'] ?? '').toString();
    if (kind == 'elicitation') {
      await _showElicitation(id, serverName, request);
    } else {
      await _showSampling(id, serverName, kind, request);
    }
  }

  Future<void> _showSampling(
    String id,
    String serverName,
    String kind,
    Map<String, dynamic> request,
  ) async {
    final isResult = kind == 'sampling_result';
    final action = await showDialog<String>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => AlertDialog(
        title: Text(isResult ? '审核 Sampling 结果' : '批准模型 Sampling 请求'),
        content: SizedBox(
          width: 560,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (serverName.isNotEmpty) ...[
                  Text(serverName, style: AppTypography.caption(dialogContext)),
                  SizedBox(height: AppSpacing.sm),
                ],
                Text(
                  isResult
                      ? '模型结果只有在你批准后才会返回给该 MCP 服务。'
                      : '该 MCP 服务请求 Amitia 执行一次受控模型采样，本次批准不会向服务提供模型密钥、角色记忆或会话历史。',
                  style: AppTypography.bodySmall(dialogContext),
                ),
                SizedBox(height: AppSpacing.md),
                SelectableText(_prettyJson(request), style: AppTypography.caption(dialogContext)),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, 'cancel'), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, 'decline'),
            child: Text('拒绝', style: TextStyle(color: dialogContext.error)),
          ),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, 'accept'), child: const Text('批准')),
        ],
      ),
    );
    if (action != null) await _resolve(id, action, const {});
  }

  Future<void> _showElicitation(
    String id,
    String serverName,
    Map<String, dynamic> request,
  ) async {
    if ((request['mode'] ?? 'form').toString() == 'url') {
      await _showUrlElicitation(id, serverName, request);
      return;
    }

    final schema = _asMap(request['requestedSchema'] ?? request['schema']);
    final properties = _asMap(schema['properties']);
    final required = schema['required'] is List
        ? (schema['required'] as List).map((value) => value.toString()).toSet()
        : <String>{};
    final controllers = <String, TextEditingController>{};
    final boolValues = <String, bool>{};
    final enumValues = <String, dynamic>{};
    for (final entry in properties.entries.take(50)) {
      final definition = _asMap(entry.value);
      final type = (definition['type'] ?? 'string').toString();
      if (type == 'boolean') {
        boolValues[entry.key] = false;
      } else if (definition['enum'] is List) {
        enumValues[entry.key] = null;
      } else {
        controllers[entry.key] = TextEditingController();
      }
    }

    try {
      final result = await showDialog<Map<String, dynamic>>(
        context: context,
        barrierDismissible: false,
        builder: (dialogContext) => StatefulBuilder(
          builder: (dialogContext, setDialogState) {
            final reason = (request['message'] ?? request['reason'] ?? '').toString();
            return AlertDialog(
              title: const Text('处理 Elicitation 请求'),
              content: SizedBox(
                width: 560,
                child: SingleChildScrollView(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (serverName.isNotEmpty) ...[
                        Text(serverName, style: AppTypography.caption(dialogContext)),
                        SizedBox(height: AppSpacing.sm),
                      ],
                      if (reason.isNotEmpty) ...[
                        Text(reason, style: AppTypography.bodySmall(dialogContext)),
                        SizedBox(height: AppSpacing.md),
                      ],
                      ...properties.entries.take(50).expand((entry) {
                        final definition = _asMap(entry.value);
                        final type = (definition['type'] ?? 'string').toString();
                        final title = (definition['title'] ?? entry.key).toString();
                        final description = (definition['description'] ?? '').toString();
                        final label = required.contains(entry.key) ? '$title *' : title;
                        Widget field;
                        if (definition['enum'] is List) {
                          final options = (definition['enum'] as List).take(100).toList();
                          field = DropdownButtonFormField<dynamic>(
                            value: enumValues[entry.key],
                            decoration: InputDecoration(
                              labelText: label,
                              helperText: description.isEmpty ? null : description,
                            ),
                            items: options
                                .map((option) => DropdownMenuItem<dynamic>(value: option, child: Text(option.toString())))
                                .toList(growable: false),
                            onChanged: (value) => setDialogState(() => enumValues[entry.key] = value),
                          );
                        } else if (type == 'boolean') {
                          field = SwitchListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text(label),
                            subtitle: description.isEmpty ? null : Text(description),
                            value: boolValues[entry.key] ?? false,
                            onChanged: (value) => setDialogState(() => boolValues[entry.key] = value),
                          );
                        } else {
                          field = Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(label, style: AppTypography.caption(dialogContext)),
                              const SizedBox(height: 6),
                              AmitiaTextField(
                                controller: controllers[entry.key],
                                hintText: description.isEmpty ? null : description,
                                keyboardType: type == 'number' || type == 'integer'
                                    ? const TextInputType.numberWithOptions(decimal: true, signed: true)
                                    : TextInputType.text,
                                maxLines: definition['format'] == 'textarea' ? 4 : 1,
                              ),
                            ],
                          );
                        }
                        return <Widget>[field, SizedBox(height: AppSpacing.md)];
                      }),
                    ],
                  ),
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(dialogContext, {'action': 'cancel'}),
                  child: const Text('取消'),
                ),
                TextButton(
                  onPressed: () => Navigator.pop(dialogContext, {'action': 'decline'}),
                  child: Text('拒绝', style: TextStyle(color: dialogContext.error)),
                ),
                FilledButton(
                  onPressed: () {
                    final content = <String, dynamic>{};
                    for (final entry in properties.entries.take(50)) {
                      final definition = _asMap(entry.value);
                      final type = (definition['type'] ?? 'string').toString();
                      dynamic value;
                      if (definition['enum'] is List) {
                        value = enumValues[entry.key];
                      } else if (type == 'boolean') {
                        value = boolValues[entry.key] ?? false;
                      } else {
                        final text = controllers[entry.key]?.text ?? '';
                        if (type == 'number') {
                          value = double.tryParse(text);
                        } else if (type == 'integer') {
                          value = int.tryParse(text);
                        } else {
                          value = text;
                        }
                      }
                      if (required.contains(entry.key) && (value == null || value == '')) {
                        ScaffoldMessenger.of(dialogContext).showSnackBar(
                          SnackBar(content: Text('请填写“${definition['title'] ?? entry.key}”')),
                        );
                        return;
                      }
                      if (value != null) content[entry.key] = value;
                    }
                    Navigator.pop(dialogContext, {'action': 'accept', 'content': content});
                  },
                  child: const Text('批准'),
                ),
              ],
            );
          },
        ),
      );
      if (result == null) return;
      await _resolve(id, (result['action'] ?? 'cancel').toString(), _asMap(result['content']));
    } finally {
      for (final controller in controllers.values) {
        controller.dispose();
      }
    }
  }

  Future<void> _showUrlElicitation(
    String id,
    String serverName,
    Map<String, dynamic> request,
  ) async {
    final url = (request['url'] ?? '').toString();
    final reason = (request['message'] ?? request['reason'] ?? '').toString();
    final action = await showDialog<String>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => AlertDialog(
        title: const Text('处理外部 URL 请求'),
        content: SizedBox(
          width: 560,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (serverName.isNotEmpty) Text(serverName, style: AppTypography.caption(dialogContext)),
              if (reason.isNotEmpty) ...[
                SizedBox(height: AppSpacing.sm),
                Text(reason, style: AppTypography.bodySmall(dialogContext)),
              ],
              SizedBox(height: AppSpacing.md),
              SelectableText(url, style: AppTypography.caption(dialogContext)),
              SizedBox(height: AppSpacing.sm),
              Text('批准后链接会复制到剪贴板，可由系统浏览器打开。', style: AppTypography.caption(dialogContext)),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, 'cancel'), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, 'decline'),
            child: Text('拒绝', style: TextStyle(color: dialogContext.error)),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, 'accept'),
            child: const Text('批准并复制链接'),
          ),
        ],
      ),
    );
    if (action == null) return;
    await _resolve(id, action, const {});
    if (action == 'accept' && url.isNotEmpty) {
      await Clipboard.setData(ClipboardData(text: url));
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('链接已复制到剪贴板')));
      }
    }
  }

  Future<void> _resolve(String id, String action, Map<String, dynamic> content) async {
    try {
      await ref.read(mcpServiceProvider).resolveInteraction(id, {
        'action': action,
        'content': action == 'accept' ? content : <String, dynamic>{},
      });
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('MCP 请求处理失败：$error')));
    }
  }

  Map<String, dynamic> _asMap(dynamic value) {
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    if (value is String && value.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(value);
        if (decoded is Map) return Map<String, dynamic>.from(decoded);
      } catch (_) {}
    }
    return <String, dynamic>{};
  }

  String _prettyJson(dynamic value) {
    try {
      final text = const JsonEncoder.withIndent('  ').convert(value);
      return text.length > 10000 ? '${text.substring(0, 10000)}…' : text;
    } catch (_) {
      final text = value.toString();
      return text.length > 10000 ? '${text.substring(0, 10000)}…' : text;
    }
  }

  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
