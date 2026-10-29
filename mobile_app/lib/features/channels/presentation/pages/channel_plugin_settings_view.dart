import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/channel_sandbox_service.dart';
import '../../../../core/services/providers.dart';

class ChannelPluginSettingsView extends ConsumerStatefulWidget {
  final String channelId;
  final String title;

  const ChannelPluginSettingsView({
    super.key,
    required this.channelId,
    required this.title,
  });

  @override
  ConsumerState<ChannelPluginSettingsView> createState() =>
      _ChannelPluginSettingsViewState();
}

class _ChannelPluginSettingsViewState
    extends ConsumerState<ChannelPluginSettingsView> {
  ChannelSandboxPolicy? _policy;
  bool _loading = true;
  bool _saving = false;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant ChannelPluginSettingsView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.channelId != widget.channelId) _load();
  }

  Future<void> _load() async {
    if (widget.channelId.trim().isEmpty) return;
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final policy = await ref
          .read(channelSandboxServiceProvider)
          .getPolicy(widget.channelId);
      if (!mounted) return;
      setState(() => _policy = policy);
    } catch (error) {
      if (!mounted) return;
      setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _save({bool? enabled, bool? allowNetwork}) async {
    final current = _policy;
    if (current == null || _saving) return;
    setState(() {
      _saving = true;
      _error = '';
    });
    try {
      final policy = await ref
          .read(channelSandboxServiceProvider)
          .savePolicy(
            channelId: widget.channelId,
            enabled: enabled ?? current.enabled,
            allowNetwork: allowNetwork ?? current.allowNetwork,
          );
      if (!mounted) return;
      setState(() => _policy = policy);
    } catch (error) {
      if (!mounted) return;
      setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    final policy = _policy;
    if (policy == null) {
      return Center(child: Text(_error.isEmpty ? '读取渠道设置失败' : _error));
    }
    return ListView(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.xl,
      ),
      children: [
        Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    widget.title,
                    style: AppTypography.sectionTitle(context),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '控制该渠道插件及其 Agent 能力的运行范围。',
                    style: AppTypography.caption(context),
                  ),
                ],
              ),
            ),
            IconButton(
              onPressed: _saving ? null : _load,
              tooltip: '刷新',
              icon: const Icon(Icons.refresh),
            ),
          ],
        ),
        if (_error.isNotEmpty) ...[
          SizedBox(height: AppSpacing.sm),
          Text(
            _error,
            style: AppTypography.bodySmall(
              context,
            ).copyWith(color: context.error),
          ),
        ],
        SizedBox(height: AppSpacing.md),
        _SettingsCard(
          children: [
            SwitchListTile(
              value: policy.enabled,
              onChanged: _saving ? null : (value) => _save(enabled: value),
              title: const Text('沙箱隔离'),
              subtitle: const Text('限制插件和 Agent 只能访问该渠道的沙箱目录'),
              contentPadding: EdgeInsets.zero,
            ),
            SwitchListTile(
              value: policy.allowNetwork,
              onChanged: !policy.enabled || _saving
                  ? null
                  : (value) => _save(allowNetwork: value),
              title: const Text('允许联网'),
              subtitle: const Text('关闭后仅允许访问本机回环地址'),
              contentPadding: EdgeInsets.zero,
            ),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        _SettingsCard(
          children: [
            _FactRow(
              label: '沙箱目录',
              value: policy.sandboxRoot.isEmpty ? '未创建' : policy.sandboxRoot,
            ),
            _FactRow(
              label: '工作区标识',
              value: policy.workspaceId.isEmpty ? '—' : policy.workspaceId,
            ),
            _FactRow(
              label: '实际状态',
              value: policy.enabled ? '已隔离，插件重启后完全生效' : '未隔离，可访问本机文件',
            ),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        Text(
          '开启后，此后从该渠道新进入的会话会把工作目录固定到沙箱内；已有会话保持原状。',
          style: AppTypography.caption(context),
        ),
      ],
    );
  }
}

class _SettingsCard extends StatelessWidget {
  final List<Widget> children;

  const _SettingsCard({required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(children: children),
    );
  }
}

class _FactRow extends StatelessWidget {
  final String label;
  final String value;

  const _FactRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 92,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          Expanded(
            child: Text(
              value,
              textAlign: TextAlign.end,
              style: AppTypography.bodySmall(context),
            ),
          ),
        ],
      ),
    );
  }
}
