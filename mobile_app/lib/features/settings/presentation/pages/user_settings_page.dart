import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/space_profile_service.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/profile_avatar.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class UserSettingsPage extends ConsumerStatefulWidget {
  const UserSettingsPage({super.key});

  @override
  ConsumerState<UserSettingsPage> createState() => _UserSettingsPageState();
}

class _UserSettingsPageState extends ConsumerState<UserSettingsPage> {
  SpaceProfile? _profile;
  bool _loading = true;
  bool _avatarUpdating = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final profile = await ref.read(spaceProfileServiceProvider).fetch();
      if (!mounted) return;
      setState(() {
        _profile = profile;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() => _loading = false);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('个人空间资料加载失败：$error')));
    }
  }

  Future<void> _save({
    String? displayName,
    String? userLabel,
    String? bio,
  }) async {
    final current = _profile;
    if (current == null) return;
    final updated = await ref
        .read(spaceProfileServiceProvider)
        .update(
          displayName: displayName ?? current.displayName,
          userLabel: userLabel ?? current.userLabel,
          bio: bio ?? current.bio,
          avatar: current.avatar,
          preferences: current.preferences,
        );
    if (!mounted) return;
    setState(() => _profile = updated);
    ref.invalidate(currentSpaceProfileProvider);
  }

  Future<void> _changeAvatar() async {
    if (_avatarUpdating) return;
    setState(() => _avatarUpdating = true);
    try {
      final avatar = await pickProfileAvatar(context);
      if (avatar == null) return;
      final updated = await ref
          .read(spaceProfileServiceProvider)
          .updateAvatar(avatar);
      if (!mounted) return;
      setState(() => _profile = updated);
      ref.invalidate(currentSpaceProfileProvider);
      ScaffoldMessenger.of(context)
        ..clearSnackBars()
        ..showSnackBar(const SnackBar(content: Text('头像已更新')));
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
        ..clearSnackBars()
        ..showSnackBar(SnackBar(content: Text('头像更新失败：$error')));
    } finally {
      if (mounted) setState(() => _avatarUpdating = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final profile = _profile;
    final displayName = (profile?.displayName ?? '').trim().isEmpty
        ? '我的空间'
        : profile!.displayName.trim();
    final initial = displayName.characters.first;

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '个人空间',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
              children: [
                SizedBox(height: AppSpacing.lg),
                Center(
                  child: Semantics(
                    button: true,
                    label: '更换头像',
                    child: GestureDetector(
                      onTap: _avatarUpdating ? null : _changeAvatar,
                      child: ProfileAvatar(
                        avatar: profile?.avatar ?? '',
                        initial: initial,
                        size: 80,
                        showEditBadge: true,
                        loading: _avatarUpdating,
                      ),
                    ),
                  ),
                ),
                SizedBox(height: AppSpacing.md),
                Center(
                  child: Text(
                    displayName,
                    style: AppTypography.sectionTitle(context),
                  ),
                ),
                const SizedBox(height: 4),
                Center(
                  child: Text(
                    profile?.spaceId.isNotEmpty == true
                        ? profile!.spaceId
                        : '本地个人空间',
                    style: AppTypography.caption(context),
                  ),
                ),
                SizedBox(height: AppSpacing.sectionGap),
                const _SectionLabel(text: '本地资料'),
                SizedBox(height: AppSpacing.sm),
                _buildCard([
                  _buildEditTile(
                    '显示名称',
                    displayName,
                    () => _showEditSheet(
                      '显示名称',
                      displayName,
                      (value) => _save(displayName: value),
                    ),
                  ),
                  _divider(),
                  _buildEditTile(
                    'AI 对你的称呼',
                    (profile?.userLabel ?? '').trim().isEmpty
                        ? '未设置'
                        : profile!.userLabel,
                    () => _showEditSheet(
                      'AI 对你的称呼',
                      profile?.userLabel ?? '',
                      (value) => _save(userLabel: value),
                      allowEmpty: true,
                    ),
                  ),
                  _divider(),
                  _buildEditTile(
                    '个人简介',
                    (profile?.bio ?? '').trim().isEmpty ? '未设置' : profile!.bio,
                    () => _showEditSheet(
                      '个人简介',
                      profile?.bio ?? '',
                      (value) => _save(bio: value),
                      maxLines: 3,
                      allowEmpty: true,
                    ),
                  ),
                ]),
                SizedBox(height: AppSpacing.sectionGap),
                const _SectionLabel(text: '身份与设备'),
                SizedBox(height: AppSpacing.sm),
                _buildCard([
                  _buildInfoTile('Space ID', profile?.spaceId ?? ''),
                  _divider(),
                  _buildInfoTile('Instance ID', profile?.instanceId ?? ''),
                  _divider(),
                  _buildNavTile(
                    icon: Icons.devices_outlined,
                    title: '设备管理',
                    subtitle: '可信设备、云端配对与撤销',
                    onTap: () => context.push(AppRoutes.settingsDevices),
                  ),
                  _divider(),
                  _buildNavTile(
                    icon: Icons.hub_outlined,
                    title: '运行模式',
                    subtitle: '本地 Core / Cloud Core',
                    onTap: () => context.push(AppRoutes.settingsRuntimeMode),
                  ),
                ]),
                SizedBox(height: AppSpacing.md),
                Padding(
                  padding: EdgeInsets.symmetric(
                    horizontal: AppSpacing.pagePadding,
                  ),
                  child: Text(
                    'Amitia 不使用产品账号。Space ID 只表示数据归属；设备身份由 Device ID + Device Credential 管理。',
                    style: AppTypography.caption(context),
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
    return const SizedBox.shrink();
  }

  Widget _buildEditTile(String title, String value, VoidCallback onTap) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Flexible(
              child: Text(
                value,
                style: AppTypography.caption(context),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoTile(String title, String value) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Row(
        children: [
          Expanded(child: Text(title, style: AppTypography.body(context))),
          Flexible(
            child: Text(
              value.trim().isEmpty ? '未获取' : value,
              style: AppTypography.caption(context),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildNavTile({
    required IconData icon,
    required String title,
    required String subtitle,
    required VoidCallback onTap,
  }) {
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
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.body(context)),
                  const SizedBox(height: 2),
                  Text(subtitle, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showEditSheet(
    String title,
    String current,
    Future<void> Function(String) onSave, {
    int maxLines = 1,
    bool allowEmpty = false,
  }) {
    final controller = TextEditingController(text: current);
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.lg,
            AppSpacing.lg,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('编辑$title', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              AmitiaTextField(
                hintText: '请输入$title',
                controller: controller,
                maxLines: maxLines,
              ),
              SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '保存',
                isFullWidth: true,
                onPressed: () async {
                  final value = controller.text.trim();
                  if (!allowEmpty && value.isEmpty) return;
                  try {
                    await onSave(value);
                    if (!sheetContext.mounted || !mounted) return;
                    Navigator.pop(sheetContext);
                    ScaffoldMessenger.of(
                      context,
                    ).showSnackBar(SnackBar(content: Text('$title已更新')));
                  } catch (error) {
                    if (!mounted) return;
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('$title保存失败：$error')),
                    );
                  }
                },
              ),
            ],
          ),
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
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
