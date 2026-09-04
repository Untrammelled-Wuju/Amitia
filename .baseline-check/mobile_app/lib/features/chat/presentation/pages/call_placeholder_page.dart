import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';

enum PlaceholderCallMode { video, screen }

Future<void> showPlaceholderCallPage(
  BuildContext context, {
  required PlaceholderCallMode mode,
  required String characterName,
}) {
  return Navigator.of(context).push<void>(
    PageRouteBuilder<void>(
      opaque: true,
      transitionDuration: const Duration(milliseconds: 240),
      reverseTransitionDuration: const Duration(milliseconds: 220),
      pageBuilder: (_, __, ___) => _PlaceholderCallPage(
        mode: mode,
        characterName: characterName,
      ),
      transitionsBuilder: (_, animation, __, child) {
        final offset = Tween<Offset>(
          begin: const Offset(0, 1),
          end: Offset.zero,
        ).animate(CurvedAnimation(parent: animation, curve: Curves.easeOutCubic));
        return SlideTransition(position: offset, child: child);
      },
    ),
  );
}

class _PlaceholderCallPage extends StatelessWidget {
  const _PlaceholderCallPage({
    required this.mode,
    required this.characterName,
  });

  final PlaceholderCallMode mode;
  final String characterName;

  bool get _isVideo => mode == PlaceholderCallMode.video;

  @override
  Widget build(BuildContext context) {
    final name = characterName.trim().isEmpty ? 'Amitia' : characterName.trim();
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final background = isDark ? const Color(0xFF111214) : const Color(0xFFF4F1ED);
    final panel = isDark ? const Color(0xFF191A1C) : const Color(0xFFFFFDF9);
    final foreground = isDark ? const Color(0xFFF1EDE8) : const Color(0xFF24221F);
    final secondary = isDark ? const Color(0xFFA7A19A) : const Color(0xFF67615A);

    return Scaffold(
      backgroundColor: background,
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(14, 8, 14, 0),
              child: Row(
                children: [
                  _PlainCallButton(
                    icon: Icons.keyboard_arrow_down_rounded,
                    foreground: foreground,
                    background: panel,
                    border: isDark ? const Color(0xFF303236) : const Color(0xFFE5DED5),
                    tooltip: '关闭占位通话页',
                    onTap: () => Navigator.of(context).pop(),
                  ),
                  const Spacer(),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
                    decoration: BoxDecoration(
                      color: panel,
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(
                        color: isDark ? const Color(0xFF303236) : const Color(0xFFE5DED5),
                      ),
                    ),
                    child: Text(
                      'UI 占位',
                      style: AppTypography.label(context).copyWith(
                        color: secondary,
                        fontSize: 10,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            Expanded(
              child: _isVideo
                  ? _VideoPlaceholderBody(
                      name: name,
                      panel: panel,
                      foreground: foreground,
                      secondary: secondary,
                    )
                  : _ScreenPlaceholderBody(
                      panel: panel,
                      foreground: foreground,
                      secondary: secondary,
                    ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(18, 12, 18, 22),
              child: _CallControls(
                isVideo: _isVideo,
                panel: panel,
                secondary: secondary,
                onEnd: () => Navigator.of(context).pop(),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _VideoPlaceholderBody extends StatelessWidget {
  const _VideoPlaceholderBody({
    required this.name,
    required this.panel,
    required this.foreground,
    required this.secondary,
  });

  final String name;
  final Color panel;
  final Color foreground;
  final Color secondary;

  @override
  Widget build(BuildContext context) {
    final initial = name.characters.first;
    return Padding(
      padding: const EdgeInsets.fromLTRB(18, 22, 18, 0),
      child: Column(
        children: [
          Align(
            alignment: Alignment.centerLeft,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: AppTypography.cardTitle(context).copyWith(color: foreground)),
                const SizedBox(height: 4),
                Text(
                  '视频通话',
                  style: AppTypography.label(context).copyWith(color: secondary),
                ),
              ],
            ),
          ),
          const Spacer(),
          Container(
            width: 104,
            height: 104,
            decoration: BoxDecoration(
              color: context.accentPrimary,
              borderRadius: BorderRadius.circular(32),
            ),
            alignment: Alignment.center,
            child: Text(
              initial,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 34,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          const SizedBox(height: 18),
          Text(
            '视频通话界面',
            style: AppTypography.pageTitle(context).copyWith(color: foreground),
          ),
          const SizedBox(height: 8),
          Text(
            '视频媒体链路尚未接入，当前仅展示通话 UI。',
            textAlign: TextAlign.center,
            style: AppTypography.bodySmall(context).copyWith(color: secondary, height: 1.55),
          ),
          const Spacer(),
          Align(
            alignment: Alignment.centerRight,
            child: Container(
              width: 92,
              height: 128,
              decoration: BoxDecoration(
                color: panel,
                borderRadius: BorderRadius.circular(20),
                border: Border.all(color: context.borderPrimary),
              ),
              alignment: Alignment.center,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.person_outline, color: secondary, size: 28),
                  const SizedBox(height: 7),
                  Text(
                    '我的画面',
                    style: AppTypography.label(context).copyWith(color: secondary),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ScreenPlaceholderBody extends StatelessWidget {
  const _ScreenPlaceholderBody({
    required this.panel,
    required this.foreground,
    required this.secondary,
  });

  final Color panel;
  final Color foreground;
  final Color secondary;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 26),
      child: Column(
        children: [
          const Spacer(),
          Container(
            width: 88,
            height: 88,
            decoration: BoxDecoration(
              color: panel,
              borderRadius: BorderRadius.circular(28),
              border: Border.all(color: context.borderPrimary),
            ),
            alignment: Alignment.center,
            child: Icon(
              Icons.screen_share_outlined,
              size: 38,
              color: context.accentPrimary,
            ),
          ),
          const SizedBox(height: 20),
          Text(
            '屏幕共享',
            style: AppTypography.pageTitle(context).copyWith(color: foreground),
          ),
          const SizedBox(height: 8),
          Text(
            '屏幕共享链路接入后，AI 可以看到你的屏幕。\n当前页面仅作为通话 UI 占位。',
            textAlign: TextAlign.center,
            style: AppTypography.bodySmall(context).copyWith(color: secondary, height: 1.6),
          ),
          const Spacer(),
        ],
      ),
    );
  }
}

class _CallControls extends StatelessWidget {
  const _CallControls({
    required this.isVideo,
    required this.panel,
    required this.secondary,
    required this.onEnd,
  });

  final bool isVideo;
  final Color panel;
  final Color secondary;
  final VoidCallback onEnd;

  @override
  Widget build(BuildContext context) {
    final items = isVideo
        ? const <_ControlSpec>[
            _ControlSpec(Icons.mic_none_outlined, '静音'),
            _ControlSpec(Icons.cameraswitch_outlined, '翻转'),
            _ControlSpec(Icons.videocam_outlined, '摄像头'),
          ]
        : const <_ControlSpec>[
            _ControlSpec(Icons.mic_none_outlined, '静音'),
            _ControlSpec(Icons.screen_share_outlined, '屏幕共享'),
          ];

    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceEvenly,
      children: [
        for (final item in items)
          _DisabledCallControl(
            spec: item,
            panel: panel,
            secondary: secondary,
          ),
        _EnabledEndControl(onTap: onEnd),
      ],
    );
  }
}

class _ControlSpec {
  const _ControlSpec(this.icon, this.label);
  final IconData icon;
  final String label;
}

class _DisabledCallControl extends StatelessWidget {
  const _DisabledCallControl({
    required this.spec,
    required this.panel,
    required this.secondary,
  });

  final _ControlSpec spec;
  final Color panel;
  final Color secondary;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: '媒体链路接入后可用',
      child: SizedBox(
        width: 70,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 54,
              height: 54,
              decoration: BoxDecoration(
                color: panel,
                shape: BoxShape.circle,
                border: Border.all(color: context.borderPrimary),
              ),
              alignment: Alignment.center,
              child: Icon(spec.icon, size: 23, color: secondary),
            ),
            const SizedBox(height: 7),
            Text(
              spec.label,
              maxLines: 1,
              style: AppTypography.label(context).copyWith(color: secondary),
            ),
          ],
        ),
      ),
    );
  }
}

class _EnabledEndControl extends StatelessWidget {
  const _EnabledEndControl({required this.onTap});
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: SizedBox(
        width: 70,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 54,
              height: 54,
              decoration: BoxDecoration(
                color: context.error,
                shape: BoxShape.circle,
              ),
              alignment: Alignment.center,
              child: const Icon(Icons.call_end, size: 24, color: Colors.white),
            ),
            const SizedBox(height: 7),
            Text('结束', style: AppTypography.label(context).copyWith(color: context.error)),
          ],
        ),
      ),
    );
  }
}

class _PlainCallButton extends StatelessWidget {
  const _PlainCallButton({
    required this.icon,
    required this.foreground,
    required this.background,
    required this.border,
    required this.tooltip,
    required this.onTap,
  });

  final IconData icon;
  final Color foreground;
  final Color background;
  final Color border;
  final String tooltip;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: background,
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: border),
          ),
          alignment: Alignment.center,
          child: Icon(icon, size: 22, color: foreground),
        ),
      ),
    );
  }
}
