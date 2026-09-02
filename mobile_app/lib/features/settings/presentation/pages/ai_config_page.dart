import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/model_config.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final _aiAppConfigProvider = FutureProvider.autoDispose<Map<String, dynamic>?>((ref) {
  return ref.read(systemServiceProvider).config();
});

class AiConfigPage extends ConsumerWidget {
  const AiConfigPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final modelsAsync = ref.watch(modelConfigListProvider);
    final charactersAsync = ref.watch(characterListProvider);
    final appConfigAsync = ref.watch(_aiAppConfigProvider);

    Object? error;
    if (modelsAsync.hasError) error = modelsAsync.error;
    if (charactersAsync.hasError) error ??= charactersAsync.error;
    if (appConfigAsync.hasError) error ??= appConfigAsync.error;
    final loading = modelsAsync.isLoading || charactersAsync.isLoading || appConfigAsync.isLoading;

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'AI 配置',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: () {
              ref.invalidate(modelConfigListProvider);
              ref.invalidate(characterListProvider);
              ref.invalidate(_aiAppConfigProvider);
            },
          ),
        ],
      ),
      body: loading
          ? const Center(child: CircularProgressIndicator())
          : error != null
              ? _LoadError(
                  error: error,
                  onRetry: () {
                    ref.invalidate(modelConfigListProvider);
                    ref.invalidate(characterListProvider);
                    ref.invalidate(_aiAppConfigProvider);
                  },
                )
              : _AiConfigContent(
                  configs: modelsAsync.value ?? const <ModelConfigDto>[],
                  characters: charactersAsync.value ?? const <CharacterDto>[],
                  appConfig: appConfigAsync.value,
                  onSaved: () {
                    ref.invalidate(modelConfigListProvider);
                    ref.invalidate(characterListProvider);
                    ref.invalidate(_aiAppConfigProvider);
                  },
                ),
    );
  }
}

class _LoadError extends StatelessWidget {
  final Object error;
  final VoidCallback onRetry;

  const _LoadError({required this.error, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.textSecondary),
            const SizedBox(height: 16),
            Text(
              '加载失败: ${error.toString().replaceFirst('Exception: ', '')}',
              style: AppTypography.body(context).copyWith(color: context.error),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            AmitiaButton(label: '重试', onPressed: onRetry),
          ],
        ),
      ),
    );
  }
}

class _AiConfigContent extends ConsumerStatefulWidget {
  final List<ModelConfigDto> configs;
  final List<CharacterDto> characters;
  final Map<String, dynamic>? appConfig;
  final VoidCallback onSaved;

  const _AiConfigContent({
    required this.configs,
    required this.characters,
    required this.appConfig,
    required this.onSaved,
  });

  @override
  ConsumerState<_AiConfigContent> createState() => _AiConfigContentState();
}

class _AiConfigContentState extends ConsumerState<_AiConfigContent> {
  static const _strategies = ['滑动窗口', '摘要压缩', '全量上下文', '向量检索'];
  static const _fallbackOptions = ['简单回复', '重试', '切换模型', '静默失败'];

  String _defaultCharacterId = '';
  String _defaultModelId = '';
  String _contextStrategy = _strategies.first;
  bool _streamingOutput = true;
  bool _messageSplitting = true;
  bool _toolCalls = true;
  String _errorFallback = _fallbackOptions.first;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _apply(widget.appConfig);
  }

  @override
  void didUpdateWidget(covariant _AiConfigContent oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!identical(widget.appConfig, oldWidget.appConfig) ||
        !identical(widget.configs, oldWidget.configs) ||
        !identical(widget.characters, oldWidget.characters)) {
      _apply(widget.appConfig);
    }
  }

  Map<String, dynamic> _settings(Map<String, dynamic>? config) {
    final raw = config?['settings'];
    return raw is Map ? Map<String, dynamic>.from(raw) : <String, dynamic>{};
  }

  void _apply(Map<String, dynamic>? config) {
    final settings = _settings(config);
    final storedCharacterId = (settings['ai_default_character_id'] ?? '').toString();
    final storedModelId = (settings['ai_default_model_id'] ?? '').toString();

    _defaultCharacterId = widget.characters.any((item) => item.id == storedCharacterId)
        ? storedCharacterId
        : _preferredCharacterId();
    _defaultModelId = widget.configs.any((item) => item.id == storedModelId)
        ? storedModelId
        : _preferredModelId();

    final strategy = (settings['ai_context_strategy'] ?? '').toString();
    _contextStrategy = _strategies.contains(strategy) ? strategy : _strategies.first;
    _streamingOutput = _parseBool(settings['ai_streaming_output'], fallback: true);
    _messageSplitting = _parseBool(settings['ai_message_splitting'], fallback: true);
    _toolCalls = _parseBool(settings['ai_tool_calls'], fallback: true);
    final fallback = (settings['ai_error_fallback'] ?? '').toString();
    _errorFallback = _fallbackOptions.contains(fallback) ? fallback : _fallbackOptions.first;
  }

  String _preferredCharacterId() {
    for (final item in widget.characters) {
      if (item.isDefault || item.isActive == 1) return item.id;
    }
    return widget.characters.isEmpty ? '' : widget.characters.first.id;
  }

  String _preferredModelId() {
    for (final item in widget.configs) {
      if (item.isActive == 1) return item.id;
    }
    return widget.configs.isEmpty ? '' : widget.configs.first.id;
  }

  bool _parseBool(dynamic value, {required bool fallback}) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    switch (value?.toString().trim().toLowerCase()) {
      case 'true':
      case '1':
      case 'yes':
      case 'on':
        return true;
      case 'false':
      case '0':
      case 'no':
      case 'off':
        return false;
      default:
        return fallback;
    }
  }

  Future<void> _save() async {
    if (_saving) return;
    setState(() => _saving = true);
    try {
      final operations = <Future<dynamic>>[];
      if (_defaultCharacterId.isNotEmpty) {
        operations.add(ref.read(characterServiceProvider).setDefault(_defaultCharacterId));
        operations.add(ref.read(characterServiceProvider).setActive(_defaultCharacterId));
      }
      if (_defaultModelId.isNotEmpty) {
        operations.add(ref.read(modelConfigServiceProvider).activate(_defaultModelId));
      }
      operations.add(
        ref.read(systemServiceProvider).updateConfig({
          'settings': <String, String>{
            'ai_default_character_id': _defaultCharacterId,
            'ai_default_model_id': _defaultModelId,
            'ai_context_strategy': _contextStrategy,
            'ai_streaming_output': _streamingOutput.toString(),
            'ai_message_splitting': _messageSplitting.toString(),
            'ai_tool_calls': _toolCalls.toString(),
            'ai_error_fallback': _errorFallback,
          },
        }),
      );
      await Future.wait(operations);
      if (!mounted) return;
      widget.onSaved();
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('AI 配置已写入后端并持久化')),
      );
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('AI 配置保存失败：${error.toString().replaceFirst('Exception: ', '')}')),
      );
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
      children: [
        const _SectionLabel(text: '全局 AI 行为'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          _buildDropdownTile(
            icon: Icons.person_outline,
            title: '默认角色',
            value: _defaultCharacterId,
            options: widget.characters.map((item) => item.id).toList(growable: false),
            labels: {for (final item in widget.characters) item.id: item.name},
            emptyLabel: '暂无角色',
            onChanged: (value) => setState(() => _defaultCharacterId = value),
          ),
          _divider(),
          _buildDropdownTile(
            icon: Icons.psychology_outlined,
            title: '默认模型',
            value: _defaultModelId,
            options: widget.configs.map((item) => item.id).toList(growable: false),
            labels: {for (final item in widget.configs) item.id: item.name.isEmpty ? item.model : item.name},
            emptyLabel: '暂无模型',
            onChanged: (value) => setState(() => _defaultModelId = value),
          ),
          _divider(),
          _buildDropdownTile(
            icon: Icons.memory,
            title: '上下文策略',
            value: _contextStrategy,
            options: _strategies,
            labels: const {},
            onChanged: (value) => setState(() => _contextStrategy = value),
          ),
        ]),
        SizedBox(height: AppSpacing.sectionGap),
        const _SectionLabel(text: '输出与调用'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          AmitiaSwitchTile(
            title: '流式输出',
            subtitle: '逐字显示回复内容',
            value: _streamingOutput,
            onChanged: (value) => setState(() => _streamingOutput = value),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '消息拆分',
            subtitle: '长文本自动分段发送',
            value: _messageSplitting,
            onChanged: (value) => setState(() => _messageSplitting = value),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '工具调用',
            subtitle: '允许 AI 调用扩展工具',
            value: _toolCalls,
            onChanged: (value) => setState(() => _toolCalls = value),
          ),
        ]),
        SizedBox(height: AppSpacing.sectionGap),
        const _SectionLabel(text: '异常处理'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          _buildDropdownTile(
            icon: Icons.error_outline,
            title: '错误回落',
            value: _errorFallback,
            options: _fallbackOptions,
            labels: const {},
            onChanged: (value) => setState(() => _errorFallback = value),
          ),
        ]),
        SizedBox(height: AppSpacing.xl),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: Column(
            children: [
              AmitiaButton(
                label: _saving ? '保存中...' : '保存 AI 配置',
                icon: Icons.save_outlined,
                isFullWidth: true,
                onPressed: _saving ? null : _save,
              ),
              SizedBox(height: AppSpacing.sm),
              AmitiaButtonOutline(
                label: '管理模型配置',
                icon: Icons.settings,
                isFullWidth: true,
                onPressed: () => context.push(AppRoutes.settingsModels),
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.xl),
      ],
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(children: children),
    );
  }

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildDropdownTile({
    required IconData icon,
    required String title,
    required String value,
    required List<String> options,
    required Map<String, String> labels,
    String emptyLabel = '未配置',
    required ValueChanged<String> onChanged,
  }) {
    final display = value.isEmpty ? emptyLabel : (labels[value] ?? value);
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: options.isEmpty ? null : () => _showOptionSheet(title, options, labels, value, onChanged),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
              child: Icon(icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Flexible(child: Text(display, overflow: TextOverflow.ellipsis, style: AppTypography.caption(context))),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showOptionSheet(
    String title,
    List<String> options,
    Map<String, String> labels,
    String current,
    ValueChanged<String> onChanged,
  ) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: EdgeInsets.all(AppSpacing.lg),
              child: Text(title, style: AppTypography.sectionTitle(context)),
            ),
            ...options.map((option) {
              final selected = option == current;
              return ListTile(
                leading: Icon(
                  selected ? Icons.radio_button_checked : Icons.radio_button_off,
                  size: 20,
                  color: selected ? context.accentPrimary : context.textTertiary,
                ),
                title: Text(labels[option] ?? option, style: AppTypography.body(context)),
                onTap: () {
                  onChanged(option);
                  Navigator.pop(sheetContext);
                },
              );
            }),
            SizedBox(height: AppSpacing.sm),
          ],
        ),
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Text(text, style: AppTypography.sectionTitle(context)),
    );
  }
}
