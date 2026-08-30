import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class UserSettingsPage extends ConsumerWidget {
  const UserSettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final userAsync = ref.watch(currentUserProvider);
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '账号',
        navigation: AmitiaAppBarNavigation.back,
        fallbackRoute: AppRoutes.settings,
      ),
      body: userAsync.when(
        loading: () => const AmitiaLoadingState(message: '正在读取账号信息'),
        error: (error, _) => AmitiaErrorState(
          message: '账号信息加载失败：${_message(error)}',
          onRetry: () => ref.invalidate(currentUserProvider),
        ),
        data: (user) {
          if (user == null) {
            return AmitiaEmptyState(
              icon: Icons.person_off_outlined,
              title: '当前未登录',
              subtitle: '账号资料来自真实登录会话；未登录时不展示模拟用户信息。',
              actionText: '前往登录',
              onAction: () => context.go(AppRoutes.login),
            );
          }
          return ListView(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.md,
              AppSpacing.pagePadding,
              AppSpacing.xl,
            ),
            children: <Widget>[
              _AccountHeader(username: user.username, role: user.role),
              SizedBox(height: AppSpacing.sectionGap),
              Text('账号信息', style: AppTypography.caption(context)),
              SizedBox(height: AppSpacing.sm),
              _AccountInfoCard(userId: user.id, username: user.username, role: user.role),
              SizedBox(height: AppSpacing.sectionGap),
              Text('账号安全', style: AppTypography.caption(context)),
              SizedBox(height: AppSpacing.sm),
              _ActionCard(
                children: <Widget>[
                  _AccountAction(
                    icon: Icons.lock_outline,
                    title: '修改密码',
                    subtitle: '调用当前账号的真实密码修改接口',
                    onTap: () => _showPasswordSheet(context, ref),
                  ),
                  Divider(height: 1, color: context.borderSecondary),
                  _AccountAction(
                    icon: Icons.devices_outlined,
                    title: '登录设备管理',
                    subtitle: '查看并撤销后端记录的账号会话',
                    onTap: () => _showSessionsSheet(context, ref),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.sectionGap),
              AmitiaButton(
                label: '退出登录',
                icon: Icons.logout,
                isDestructive: true,
                isFullWidth: true,
                onPressed: () => _confirmLogout(context, ref),
              ),
            ],
          );
        },
      ),
    );
  }

  static Future<void> _showPasswordSheet(BuildContext context, WidgetRef ref) async {
    final oldController = TextEditingController();
    final newController = TextEditingController();
    final confirmController = TextEditingController();
    var saving = false;

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.lg,
              AppSpacing.lg,
              AppSpacing.lg,
              MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
            ),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text('修改密码', style: AppTypography.sectionTitle(context)),
                  SizedBox(height: AppSpacing.lg),
                  _PasswordField(label: '当前密码', controller: oldController),
                  SizedBox(height: AppSpacing.md),
                  _PasswordField(label: '新密码', controller: newController, hint: '至少 6 位'),
                  SizedBox(height: AppSpacing.md),
                  _PasswordField(label: '确认密码', controller: confirmController),
                  SizedBox(height: AppSpacing.lg),
                  AmitiaButton(
                    label: saving ? '修改中…' : '确认修改',
                    isFullWidth: true,
                    onPressed: saving
                        ? null
                        : () async {
                            if (newController.text.length < 6) {
                              _snack(context, '新密码至少 6 位', error: true);
                              return;
                            }
                            if (newController.text != confirmController.text) {
                              _snack(context, '两次输入的新密码不一致', error: true);
                              return;
                            }
                            setSheetState(() => saving = true);
                            try {
                              await ref.read(authServiceProvider).changePassword(oldController.text, newController.text);
                              ref.invalidate(currentUserProvider);
                              if (!sheetContext.mounted) return;
                              Navigator.pop(sheetContext);
                              if (context.mounted) _snack(context, '密码已修改');
                            } catch (error) {
                              if (context.mounted) _snack(context, '修改失败：${_message(error)}', error: true);
                            } finally {
                              if (sheetContext.mounted) setSheetState(() => saving = false);
                            }
                          },
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );

    oldController.dispose();
    newController.dispose();
    confirmController.dispose();
  }

  static Future<void> _showSessionsSheet(BuildContext context, WidgetRef ref) async {
    List<Map<String, dynamic>> sessions;
    try {
      sessions = await ref.read(authServiceProvider).sessions();
    } catch (error) {
      if (context.mounted) _snack(context, '加载登录会话失败：${_message(error)}', error: true);
      return;
    }
    if (!context.mounted) return;

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (sheetContext) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(sheetContext).height * 0.70,
          child: Column(
            children: <Widget>[
              Padding(
                padding: EdgeInsets.all(AppSpacing.lg),
                child: Row(
                  children: <Widget>[
                    Expanded(child: Text('登录设备', style: AppTypography.sectionTitle(sheetContext))),
                    TextButton(
                      onPressed: sessions.length <= 1
                          ? null
                          : () async {
                              try {
                                final count = await ref.read(authServiceProvider).revokeOtherSessions();
                                if (!sheetContext.mounted) return;
                                Navigator.pop(sheetContext);
                                if (context.mounted) _snack(context, '已退出 $count 个其他登录会话');
                              } catch (error) {
                                if (context.mounted) _snack(context, '操作失败：${_message(error)}', error: true);
                              }
                            },
                      child: const Text('退出其他设备'),
                    ),
                  ],
                ),
              ),
              Expanded(
                child: sessions.isEmpty
                    ? const AmitiaEmptyState(icon: Icons.devices_other_outlined, title: '暂无活跃登录会话')
                    : ListView.separated(
                        padding: EdgeInsets.fromLTRB(AppSpacing.lg, 0, AppSpacing.lg, AppSpacing.lg),
                        itemCount: sessions.length,
                        separatorBuilder: (_, __) => Divider(height: 1, color: sheetContext.borderSecondary),
                        itemBuilder: (_, index) {
                          final session = sessions[index];
                          final current = session['current'] == true;
                          final sessionId = (session['sessionId'] ?? '').toString();
                          final device = (session['deviceName'] ?? '').toString().trim();
                          final ip = (session['ipAddress'] ?? '').toString().trim();
                          final agent = (session['userAgent'] ?? '').toString().trim();
                          final lastActive = (session['lastActiveAt'] ?? session['createdAt'] ?? '').toString().trim();
                          final subtitle = <String>[if (ip.isNotEmpty) ip, if (agent.isNotEmpty) agent, if (lastActive.isNotEmpty) lastActive].join(' · ');
                          return AmitiaListTile(
                            leading: Icon(current ? Icons.devices : Icons.devices_other_outlined, color: current ? sheetContext.accentPrimary : sheetContext.textTertiary),
                            title: device.isEmpty ? (current ? '当前设备' : '登录会话') : device,
                            subtitle: subtitle.isEmpty ? null : subtitle,
                            trailing: current
                                ? AmitiaStatusBadge(label: '当前', type: BadgeType.success)
                                : IconButton(
                                    tooltip: '退出该会话',
                                    icon: const Icon(Icons.logout, size: 19),
                                    onPressed: sessionId.isEmpty
                                        ? null
                                        : () async {
                                            try {
                                              await ref.read(authServiceProvider).revokeSession(sessionId);
                                              if (!sheetContext.mounted) return;
                                              Navigator.pop(sheetContext);
                                              if (context.mounted) _showSessionsSheet(context, ref);
                                            } catch (error) {
                                              if (context.mounted) _snack(context, '退出失败：${_message(error)}', error: true);
                                            }
                                          },
                                  ),
                          );
                        },
                      ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  static Future<void> _confirmLogout(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('退出登录'),
        content: const Text('退出后会清除当前设备保存的账号会话。'),
        actions: <Widget>[
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text('退出', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await ref.read(authServiceProvider).logout();
    ref.invalidate(currentUserProvider);
    if (context.mounted) context.go(AppRoutes.login);
  }

  static void _snack(BuildContext context, String message, {bool error = false}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: error ? context.error : null),
    );
  }

  static String _message(Object error) => error.toString().replaceFirst('Exception: ', '').replaceFirst('Bad state: ', '');
}

class _AccountHeader extends StatelessWidget {
  final String username;
  final String role;

  const _AccountHeader({required this.username, required this.role});

  @override
  Widget build(BuildContext context) {
    final initial = username.isEmpty ? '?' : username.characters.first.toUpperCase();
    return Row(
      children: <Widget>[
        Container(
          width: 58,
          height: 58,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(19),
            gradient: LinearGradient(colors: <Color>[context.accentPrimary, context.accentSecondary]),
          ),
          child: Center(child: Text(initial, style: const TextStyle(color: Colors.white, fontSize: 20, fontWeight: FontWeight.w700))),
        ),
        const SizedBox(width: 13),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(username, style: AppTypography.sectionTitle(context)),
              const SizedBox(height: 4),
              Text(_roleLabel(role), style: AppTypography.caption(context)),
            ],
          ),
        ),
      ],
    );
  }

  static String _roleLabel(String role) {
    switch (role.toLowerCase()) {
      case 'admin':
        return '管理员账号';
      case 'owner':
        return '所有者账号';
      default:
        return '用户账号';
    }
  }
}

class _AccountInfoCard extends StatelessWidget {
  final String userId;
  final String username;
  final String role;

  const _AccountInfoCard({required this.userId, required this.username, required this.role});

  @override
  Widget build(BuildContext context) {
    return _ActionCard(
      children: <Widget>[
        _InfoRow(label: '用户名', value: username),
        Divider(height: 1, color: context.borderSecondary),
        _InfoRow(label: '账号 ID', value: userId),
        Divider(height: 1, color: context.borderSecondary),
        _InfoRow(label: '角色', value: role),
      ],
    );
  }
}

class _ActionCard extends StatelessWidget {
  final List<Widget> children;
  const _ActionCard({required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(children: children),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 12),
      child: Row(
        children: <Widget>[
          Expanded(child: Text(label, style: AppTypography.body(context))),
          Flexible(child: Text(value, style: AppTypography.caption(context), textAlign: TextAlign.right, overflow: TextOverflow.ellipsis)),
        ],
      ),
    );
  }
}

class _AccountAction extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _AccountAction({required this.icon, required this.title, required this.subtitle, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return AmitiaListTile(
      leading: Container(
        width: 32,
        height: 32,
        decoration: BoxDecoration(color: context.accentSoft, borderRadius: BorderRadius.circular(11)),
        child: Icon(icon, size: 17, color: context.accentPrimary),
      ),
      title: title,
      subtitle: subtitle,
      trailing: Icon(Icons.chevron_right, size: 19, color: context.textTertiary),
      onTap: onTap,
    );
  }
}

class _PasswordField extends StatelessWidget {
  final String label;
  final String hint;
  final TextEditingController controller;

  const _PasswordField({required this.label, required this.controller, this.hint = '请输入密码'});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text(label, style: AppTypography.label(context)),
        const SizedBox(height: 5),
        AmitiaTextField(controller: controller, hintText: hint, obscureText: true),
      ],
    );
  }
}
