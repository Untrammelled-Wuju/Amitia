import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/services/providers.dart';

class UserSettingsPage extends ConsumerStatefulWidget {
  const UserSettingsPage({super.key});

  @override
  ConsumerState<UserSettingsPage> createState() => _UserSettingsPageState();
}

class _UserSettingsPageState extends ConsumerState<UserSettingsPage> {
  late String _username;
  late String _nickname;
  late String _userLabel;
  late String _bio;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _username = '';
    _nickname = '';
    _userLabel = '';
    _bio = '';
    _loadUser();
  }

  Future<void> _loadUser() async {
    final auth = ref.read(authServiceProvider);
    final user = await auth.currentUser;
    if (user != null && mounted) {
      setState(() {
        _username = user.username;
        _nickname = user.username;
        _userLabel = user.username;
        _loading = false;
      });
    } else if (mounted) {
      setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '用户设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
              children: [
                SizedBox(height: AppSpacing.lg),
                Center(
                  child: Container(
                    width: 80,
                    height: 80,
                    decoration: BoxDecoration(
                      color: context.accentPrimary,
                      shape: BoxShape.circle,
                    ),
                    child: Center(
                      child: Text(
                        _nickname.isNotEmpty ? _nickname.substring(0, 1) : 'U',
                        style: const TextStyle(color: Colors.white, fontSize: 32, fontWeight: FontWeight.w600),
                      ),
                    ),
                  ),
                ),
                SizedBox(height: AppSpacing.md),
                Center(child: Text(_nickname, style: AppTypography.sectionTitle(context))),
                const SizedBox(height: 4),
                Center(child: Text('@$_username', style: AppTypography.caption(context))),
                SizedBox(height: AppSpacing.sectionGap),
                _SectionLabel(text: '基础资料'),
                SizedBox(height: AppSpacing.sm),
                _buildCard([
                  _buildEditTile('昵称', _nickname, () => _showEditSheet('昵称', _nickname, (v) => setState(() => _nickname = v))),
                  _divider(),
                  _buildEditTile('用户名', _username, null),
                  _divider(),
                  _buildEditTile('用户称呼', _userLabel, () => _showEditSheet('用户称呼', _userLabel, (v) => setState(() => _userLabel = v))),
                  _divider(),
                  _buildEditTile('个人简介', _bio.isEmpty ? '未设置' : _bio, () => _showEditSheet('个人简介', _bio, (v) => setState(() => _bio = v), maxLines: 3)),
                ]),
                SizedBox(height: AppSpacing.sectionGap),
                _SectionLabel(text: '账号安全'),
                SizedBox(height: AppSpacing.sm),
                _buildCard([
                  _buildNavTile(icon: Icons.lock_outline, title: '修改密码', onTap: _showPasswordSheet),
                  _divider(),
                  _buildNavTile(icon: Icons.devices_outlined, title: '登录设备管理', onTap: _showSessionsSheet),
                ]),
                SizedBox(height: AppSpacing.sectionGap),
                Padding(
                  padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  child: AmitiaButton(
                    label: '退出登录',
                    icon: Icons.logout,
                    isDestructive: true,
                    isFullWidth: true,
                    onPressed: _confirmLogout,
                  ),
                ),
                SizedBox(height: AppSpacing.xl),
              ],
            ),
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

  Widget _buildEditTile(String title, String value, VoidCallback? onTap) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Text(value, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
            if (onTap != null) ...[
              const SizedBox(width: 4),
              Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildNavTile({required IconData icon, required String title, required VoidCallback onTap}) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
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
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showEditSheet(String title, String current, ValueChanged<String> onSave, {int maxLines = 1}) {
    final ctrl = TextEditingController(text: current);
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: EdgeInsets.fromLTRB(AppSpacing.lg, AppSpacing.lg, AppSpacing.lg, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('编辑$title', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              AmitiaTextField(hintText: '请输入$title', controller: ctrl, maxLines: maxLines),
              SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '保存',
                isFullWidth: true,
                onPressed: () {
                  if (ctrl.text.isEmpty) return;
                  onSave(ctrl.text);
                  Navigator.pop(ctx);
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('$title已更新'), duration: const Duration(seconds: 1)),
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showPasswordSheet() {
    final oldCtrl = TextEditingController();
    final newCtrl = TextEditingController();
    final confirmCtrl = TextEditingController();
    bool saving = false;
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(AppSpacing.lg, AppSpacing.lg, AppSpacing.lg, MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('修改密码', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.lg),
                Text('当前密码', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                AmitiaTextField(hintText: '输入当前密码', controller: oldCtrl, obscureText: true),
                SizedBox(height: AppSpacing.md),
                Text('新密码', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                AmitiaTextField(hintText: '至少 6 位', controller: newCtrl, obscureText: true),
                SizedBox(height: AppSpacing.md),
                Text('确认密码', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                AmitiaTextField(hintText: '再次输入新密码', controller: confirmCtrl, obscureText: true),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: saving ? '修改中...' : '确认修改',
                  isFullWidth: true,
                  onPressed: saving
                      ? null
                      : () async {
                          if (newCtrl.text.length < 6) {
                            ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('新密码至少 6 位')));
                            return;
                          }
                          if (newCtrl.text != confirmCtrl.text) {
                            ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('两次输入的新密码不一致')));
                            return;
                          }
                          setSheetState(() => saving = true);
                          try {
                            await ref.read(authServiceProvider).changePassword(oldCtrl.text, newCtrl.text);
                            if (!mounted) return;
                            Navigator.pop(sheetContext);
                            ref.invalidate(currentUserProvider);
                            ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('密码已修改，其他登录会话已按安全策略处理')));
                          } catch (e) {
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('修改失败：$e')));
                            }
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
    );
  }

  Future<void> _showSessionsSheet() async {
    List<Map<String, dynamic>> sessions;
    try {
      sessions = await ref.read(authServiceProvider).sessions();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('加载登录会话失败：$e')));
      return;
    }
    if (!mounted) return;
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(sheetContext).height * 0.68,
          child: Column(
            children: [
              Padding(
                padding: EdgeInsets.all(AppSpacing.lg),
                child: Row(
                  children: [
                    Expanded(child: Text('登录设备管理', style: AppTypography.sectionTitle(sheetContext))),
                    TextButton(
                      onPressed: sessions.length <= 1
                          ? null
                          : () async {
                              try {
                                final count = await ref.read(authServiceProvider).revokeOtherSessions();
                                if (!sheetContext.mounted) return;
                                Navigator.pop(sheetContext);
                                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已退出 $count 个其他登录会话')));
                              } catch (e) {
                                if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e')));
                              }
                            },
                      child: const Text('退出其他设备'),
                    ),
                  ],
                ),
              ),
              Expanded(
                child: sessions.isEmpty
                    ? Center(child: Text('暂无活跃登录会话', style: AppTypography.caption(sheetContext)))
                    : ListView.separated(
                        padding: EdgeInsets.fromLTRB(AppSpacing.lg, 0, AppSpacing.lg, AppSpacing.lg),
                        itemCount: sessions.length,
                        separatorBuilder: (_, __) => const Divider(height: 1),
                        itemBuilder: (_, index) {
                          final session = sessions[index];
                          final current = session['current'] == true;
                          final sessionId = (session['sessionId'] ?? '').toString();
                          final device = (session['deviceName'] ?? '').toString();
                          final agent = (session['userAgent'] ?? '').toString();
                          final ip = (session['ipAddress'] ?? '').toString();
                          final lastActive = (session['lastActiveAt'] ?? session['createdAt'] ?? '').toString();
                          return ListTile(
                            contentPadding: EdgeInsets.zero,
                            leading: Icon(current ? Icons.devices : Icons.devices_other_outlined, color: current ? sheetContext.accentPrimary : sheetContext.textTertiary),
                            title: Text(device.isEmpty ? (current ? '当前设备' : '登录会话') : device),
                            subtitle: Text([if (ip.isNotEmpty) ip, if (agent.isNotEmpty) agent, if (lastActive.isNotEmpty) lastActive].join(' · '), maxLines: 2, overflow: TextOverflow.ellipsis),
                            trailing: current
                                ? const Text('当前')
                                : IconButton(
                                    tooltip: '退出该会话',
                                    icon: const Icon(Icons.logout),
                                    onPressed: sessionId.isEmpty
                                        ? null
                                        : () async {
                                            try {
                                              await ref.read(authServiceProvider).revokeSession(sessionId);
                                              if (!sheetContext.mounted) return;
                                              Navigator.pop(sheetContext);
                                              _showSessionsSheet();
                                            } catch (e) {
                                              if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('退出失败：$e')));
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

  void _confirmLogout() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('退出登录', style: AppTypography.cardTitle(context)),
        content: Text('确定要退出登录吗？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              final auth = ref.read(authServiceProvider);
              auth.logout();
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('已退出登录'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('退出', style: TextStyle(color: context.error)),
          ),
        ],
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
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
