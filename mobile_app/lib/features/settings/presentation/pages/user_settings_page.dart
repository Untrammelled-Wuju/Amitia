import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../shared/mock_data/mock_data.dart';

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

  @override
  void initState() {
    super.initState();
    final u = MockSettings.userSettings;
    _username = u.username;
    _nickname = u.nickname;
    _userLabel = u.userLabel;
    _bio = u.bio;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '用户设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          const SizedBox(height: AppSpacing.lg),
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
          const SizedBox(height: AppSpacing.md),
          Center(child: Text(_nickname, style: AppTypography.sectionTitle(context))),
          const SizedBox(height: 4),
          Center(child: Text('@$_username', style: AppTypography.caption(context))),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '基础资料'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildEditTile('昵称', _nickname, () => _showEditSheet('昵称', _nickname, (v) => setState(() => _nickname = v))),
            _divider(),
            _buildEditTile('用户名', _username, () => _showEditSheet('用户名', _username, (v) => setState(() => _username = v))),
            _divider(),
            _buildEditTile('用户称呼', _userLabel, () => _showEditSheet('用户称呼', _userLabel, (v) => setState(() => _userLabel = v))),
            _divider(),
            _buildEditTile('个人简介', _bio.isEmpty ? '未设置' : _bio, () => _showEditSheet('个人简介', _bio, (v) => setState(() => _bio = v), maxLines: 3)),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '账号安全'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildNavTile(icon: Icons.lock_outline, title: '修改密码', onTap: _showPasswordSheet),
            _divider(),
            _buildNavTile(icon: Icons.devices_outlined, title: '登录设备管理', onTap: () => _showTip('登录设备管理')),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaButton(
              label: '退出登录',
              icon: Icons.logout,
              isDestructive: true,
              isFullWidth: true,
              onPressed: _confirmLogout,
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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

  Widget _buildEditTile(String title, String value, VoidCallback onTap) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Text(value, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
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
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
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
              const SizedBox(height: AppSpacing.lg),
              AmitiaTextField(hintText: '请输入$title', controller: ctrl, maxLines: maxLines),
              const SizedBox(height: AppSpacing.lg),
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
              Text('修改密码', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('当前密码', style: AppTypography.label(context)),
              const SizedBox(height: 4),
              AmitiaTextField(hintText: '输入当前密码', controller: oldCtrl, obscureText: true),
              const SizedBox(height: AppSpacing.md),
              Text('新密码', style: AppTypography.label(context)),
              const SizedBox(height: 4),
              AmitiaTextField(hintText: '输入新密码', controller: newCtrl, obscureText: true),
              const SizedBox(height: AppSpacing.md),
              Text('确认密码', style: AppTypography.label(context)),
              const SizedBox(height: 4),
              AmitiaTextField(hintText: '再次输入新密码', controller: confirmCtrl, obscureText: true),
              const SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '确认修改',
                isFullWidth: true,
                onPressed: () {
                  Navigator.pop(ctx);
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('密码已修改'), duration: Duration(seconds: 1)),
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showTip(String title) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('$title · 即将开放'), duration: const Duration(seconds: 1)),
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
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
