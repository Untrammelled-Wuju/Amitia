import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';

const _centerDomain = 'center';
const _localDomain = 'local';

String _domainLabel(String domain) => domain == _centerDomain ? '统筹中心' : '本机';

class RoleDomainSettings extends ConsumerStatefulWidget {
  const RoleDomainSettings({super.key});

  @override
  ConsumerState<RoleDomainSettings> createState() => _RoleDomainSettingsState();
}

class _RoleDomainSettingsState extends ConsumerState<RoleDomainSettings> {
  Future<List<Map<String, dynamic>>>? _rolesFuture;
  String _rolesDomain = '';

  Future<List<Map<String, dynamic>>> _roles(String domain) {
    if (_rolesDomain != domain || _rolesFuture == null) {
      _rolesDomain = domain;
      _rolesFuture = ref.read(deviceMeshServiceProvider).domainRoles(domain);
    }
    return _rolesFuture!;
  }

  @override
  Widget build(BuildContext context) {
    final coordination = ref.watch(deviceCoordinationProvider);
    final policy = coordination.asData?.value['policy'];
    if (policy is! Map) {
      return _tile(
        context,
        subtitle: coordination.hasError
            ? '默认角色读取失败：${coordination.error}'
            : '正在读取默认角色',
        value: '—',
        onTap: null,
      );
    }
    final coordinated = policy['coordinated'] == true;
    final domain = coordinated ? _centerDomain : _localDomain;
    final roleId = (policy[coordinated ? 'centerRole' : 'localRole'] ?? '')
        .toString()
        .trim();
    return FutureBuilder<List<Map<String, dynamic>>>(
      future: _roles(domain),
      builder: (context, snapshot) {
        final roles = snapshot.data ?? const <Map<String, dynamic>>[];
        final name = roles
            .where((role) => role['id']?.toString() == roleId)
            .map((role) => (role['name'] ?? '').toString().trim())
            .firstWhere((value) => value.isNotEmpty, orElse: () => '');
        return _tile(
          context,
          subtitle: roleId.isEmpty
              ? '调用或创建时按当时的角色来源再选择'
              : '当前使用${_domainLabel(domain)}角色',
          value: roleId.isEmpty ? '未指定' : (name.isEmpty ? roleId : name),
          onTap: () => _open(context, policy, domain, roleId),
        );
      },
    );
  }

  Future<void> _open(
    BuildContext context,
    Map policy,
    String domain,
    String roleId,
  ) async {
    final deviceId = (policy['deviceId'] ?? '').toString();
    final revision = policy['modeRevision'];
    if (deviceId.isEmpty || revision is! int) return;
    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => _RoleDomainSheet(
        deviceId: deviceId,
        expectedRevision: revision,
        initialDomain: domain,
        initialRole: roleId,
      ),
    );
    if (saved == true) {
      ref.invalidate(deviceCoordinationProvider);
      _rolesFuture = null;
      _rolesDomain = '';
    }
  }
}

class _RoleDomainSheet extends ConsumerStatefulWidget {
  final String deviceId;
  final int expectedRevision;
  final String initialDomain;
  final String initialRole;

  const _RoleDomainSheet({
    required this.deviceId,
    required this.expectedRevision,
    required this.initialDomain,
    required this.initialRole,
  });

  @override
  ConsumerState<_RoleDomainSheet> createState() => _RoleDomainSheetState();
}

class _RoleDomainSheetState extends ConsumerState<_RoleDomainSheet> {
  late String _domain = widget.initialDomain;
  late String _role = widget.initialRole;
  final _roleFutures = <String, Future<List<Map<String, dynamic>>>>{};
  bool _saving = false;
  String _error = '';

  Future<List<Map<String, dynamic>>> _roles(String domain) =>
      _roleFutures.putIfAbsent(
        domain,
        () => ref.read(deviceMeshServiceProvider).domainRoles(domain),
      );

  Future<void> _save() async {
    if (_saving) return;
    setState(() {
      _saving = true;
      _error = '';
    });
    try {
      await ref
          .read(deviceMeshServiceProvider)
          .selectRoleDomain(
            widget.deviceId,
            domain: _domain,
            role: _role,
            expectedRevision: widget.expectedRevision,
          );
      if (mounted) Navigator.of(context).pop(true);
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = '保存失败：$error';
          _saving = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        AppSpacing.lg + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('默认角色', style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.sm),
            Text(
              '数据归属跟随角色来源：使用统筹中心角色时数据保存在统筹中心，使用本机角色时数据保存在本机，历史数据不会迁移。',
              style: AppTypography.caption(context),
            ),
            SizedBox(height: AppSpacing.md),
            Text('角色来源', style: AppTypography.caption(context)),
            for (final domain in const [_localDomain, _centerDomain])
              RadioListTile<String>(
                value: domain,
                groupValue: _domain,
                onChanged: _saving
                    ? null
                    : (value) => setState(() {
                        _domain = value ?? _domain;
                        _role = '';
                      }),
                contentPadding: EdgeInsets.zero,
                title: Text(
                  _domainLabel(domain),
                  style: AppTypography.body(context),
                ),
                subtitle: Text(
                  domain == _localDomain
                      ? '使用这台设备的角色，同时关闭统筹中心管理员授权'
                      : '使用统筹中心角色，调用与创建的数据归统筹中心',
                  style: AppTypography.caption(context),
                ),
              ),
            SizedBox(height: AppSpacing.sm),
            Text('默认角色', style: AppTypography.caption(context)),
            FutureBuilder<List<Map<String, dynamic>>>(
              future: _roles(_domain),
              builder: (context, snapshot) {
                if (snapshot.connectionState != ConnectionState.done) {
                  return const LinearProgressIndicator();
                }
                if (snapshot.hasError) {
                  return Text(
                    '角色列表读取失败：${snapshot.error}',
                    style: AppTypography.caption(
                      context,
                    ).copyWith(color: Theme.of(context).colorScheme.error),
                  );
                }
                final roles = snapshot.data ?? const <Map<String, dynamic>>[];
                return Column(
                  children: [
                    RadioListTile<String>(
                      value: '',
                      groupValue: _role,
                      onChanged: _saving
                          ? null
                          : (value) => setState(() => _role = ''),
                      contentPadding: EdgeInsets.zero,
                      title: Text('不指定', style: AppTypography.body(context)),
                      subtitle: Text(
                        '每次调用或创建时再选择角色',
                        style: AppTypography.caption(context),
                      ),
                    ),
                    for (final role in roles)
                      RadioListTile<String>(
                        value: (role['id'] ?? '').toString(),
                        groupValue: _role,
                        onChanged: _saving
                            ? null
                            : (value) => setState(() => _role = value ?? ''),
                        contentPadding: EdgeInsets.zero,
                        title: Text(
                          (role['name'] ?? role['id'] ?? '').toString(),
                          style: AppTypography.body(context),
                        ),
                      ),
                  ],
                );
              },
            ),
            if (_error.isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              Text(
                _error,
                style: AppTypography.caption(
                  context,
                ).copyWith(color: Theme.of(context).colorScheme.error),
              ),
            ],
            SizedBox(height: AppSpacing.lg),
            AmitiaButton(
              label: _saving ? '保存中…' : '保存',
              isFullWidth: true,
              onPressed: _saving ? null : _save,
            ),
          ],
        ),
      ),
    );
  }
}

Widget _tile(
  BuildContext context, {
  required String subtitle,
  required String value,
  required VoidCallback? onTap,
}) {
  return InkWell(
    onTap: onTap,
    child: Padding(
      padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 12),
      child: Row(
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: BorderRadius.circular(11),
            ),
            child: Icon(
              Icons.badge_outlined,
              size: 17,
              color: context.textSecondary,
            ),
          ),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('默认角色', style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(
                  subtitle,
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Text(
            value,
            style: AppTypography.caption(
              context,
            ).copyWith(color: context.textTertiary),
          ),
          if (onTap != null)
            Icon(Icons.chevron_right, size: 19, color: context.textTertiary),
        ],
      ),
    ),
  );
}
