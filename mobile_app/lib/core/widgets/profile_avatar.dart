import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';

const int _maxAvatarBytes = 3 * 1024 * 1024;

enum _ProfileAvatarAction { gallery, camera, remove }

Future<String?> pickProfileAvatar(BuildContext context) async {
  final action = await showModalBottomSheet<_ProfileAvatarAction>(
    context: context,
    backgroundColor: context.surfacePrimary,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
    ),
    builder: (sheetContext) => SafeArea(
      child: Padding(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.md,
          AppSpacing.md,
          AppSpacing.md,
          AppSpacing.lg,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8),
              child: Text(
                '更换头像',
                style: AppTypography.sectionTitle(sheetContext),
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            _AvatarOptionTile(
              icon: Icons.photo_library_outlined,
              label: '从相册选择',
              onTap: () =>
                  Navigator.pop(sheetContext, _ProfileAvatarAction.gallery),
            ),
            _AvatarOptionTile(
              icon: Icons.photo_camera_outlined,
              label: '拍照',
              onTap: () =>
                  Navigator.pop(sheetContext, _ProfileAvatarAction.camera),
            ),
            _AvatarOptionTile(
              icon: Icons.delete_outline,
              label: '恢复默认头像',
              isDestructive: true,
              onTap: () =>
                  Navigator.pop(sheetContext, _ProfileAvatarAction.remove),
            ),
          ],
        ),
      ),
    ),
  );
  if (action == null || !context.mounted) return null;
  if (action == _ProfileAvatarAction.remove) return '';
  final picked = await ImagePicker().pickImage(
    source: action == _ProfileAvatarAction.camera
        ? ImageSource.camera
        : ImageSource.gallery,
    maxWidth: 1024,
    maxHeight: 1024,
    imageQuality: 85,
  );
  if (picked == null) return null;
  final bytes = await picked.readAsBytes();
  if (bytes.isEmpty) throw StateError('无法读取所选头像');
  if (bytes.length > _maxAvatarBytes) {
    throw StateError('头像图片不能超过 3 MB');
  }
  return 'data:${_mimeTypeForPath(picked.path)};base64,${base64Encode(bytes)}';
}

String _mimeTypeForPath(String path) {
  final normalized = path.toLowerCase();
  if (normalized.endsWith('.png')) return 'image/png';
  if (normalized.endsWith('.webp')) return 'image/webp';
  if (normalized.endsWith('.gif')) return 'image/gif';
  if (normalized.endsWith('.bmp')) return 'image/bmp';
  return 'image/jpeg';
}

class ProfileAvatar extends StatelessWidget {
  final String avatar;
  final String initial;
  final double size;
  final bool showEditBadge;
  final bool loading;
  final BorderRadiusGeometry? borderRadius;

  const ProfileAvatar({
    super.key,
    required this.avatar,
    required this.initial,
    this.size = 48,
    this.showEditBadge = false,
    this.loading = false,
    this.borderRadius,
  });

  @override
  Widget build(BuildContext context) {
    final radius = borderRadius ?? BorderRadius.circular(size / 2);
    final badgeSize = size < 40 ? 16.0 : 24.0;
    return SizedBox(
      width: size,
      height: size,
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          Container(
            width: size,
            height: size,
            clipBehavior: Clip.antiAlias,
            decoration: BoxDecoration(
              color: context.accentPrimary,
              borderRadius: radius,
            ),
            child: _buildAvatarContent(context),
          ),
          if (loading)
            Positioned.fill(
              child: Container(
                decoration: BoxDecoration(
                  color: Colors.black.withValues(alpha: 0.32),
                  borderRadius: radius,
                ),
                child: const Center(
                  child: SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: Colors.white,
                    ),
                  ),
                ),
              ),
            ),
          if (showEditBadge && !loading)
            Positioned(
              right: -2,
              bottom: -2,
              child: Container(
                width: badgeSize,
                height: badgeSize,
                decoration: BoxDecoration(
                  color: context.accentPrimary,
                  shape: BoxShape.circle,
                  border: Border.all(color: context.surfacePrimary, width: 2),
                ),
                child: Icon(
                  Icons.photo_camera_outlined,
                  size: badgeSize * 0.58,
                  color: Colors.white,
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildAvatarContent(BuildContext context) {
    if (avatar.startsWith('data:image/')) {
      final comma = avatar.indexOf(',');
      if (comma > 0) {
        try {
          return Image.memory(
            base64Decode(avatar.substring(comma + 1)),
            fit: BoxFit.cover,
            errorBuilder: (_, __, ___) => _buildInitial(context),
          );
        } catch (_) {
          return _buildInitial(context);
        }
      }
    }
    final uri = Uri.tryParse(avatar);
    if (uri != null && (uri.scheme == 'http' || uri.scheme == 'https')) {
      return Image.network(
        avatar,
        fit: BoxFit.cover,
        errorBuilder: (_, __, ___) => _buildInitial(context),
        loadingBuilder: (context, child, progress) =>
            progress == null ? child : _buildInitial(context),
      );
    }
    return _buildInitial(context);
  }

  Widget _buildInitial(BuildContext context) {
    return Center(
      child: Text(
        initial.trim().isEmpty ? '?' : initial.trim().characters.first,
        style: TextStyle(
          color: Colors.white,
          fontSize: size * 0.42,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

class _AvatarOptionTile extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final bool isDestructive;

  const _AvatarOptionTile({
    required this.icon,
    required this.label,
    required this.onTap,
    this.isDestructive = false,
  });

  @override
  Widget build(BuildContext context) {
    final color = isDestructive ? context.error : context.textPrimary;
    return Material(
      color: Colors.transparent,
      borderRadius: AppRadius.brMedium,
      child: InkWell(
        borderRadius: AppRadius.brMedium,
        onTap: onTap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 52),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: isDestructive
                        ? context.error.withValues(alpha: 0.1)
                        : context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(
                    icon,
                    size: 20,
                    color: isDestructive
                        ? context.error
                        : context.accentPrimary,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    label,
                    style: TextStyle(
                      color: color,
                      fontSize: 15,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
