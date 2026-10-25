import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_icons.dart';

class MainTabScaffold extends StatefulWidget {
  const MainTabScaffold({super.key, required this.navigationShell});

  final StatefulNavigationShell navigationShell;

  @override
  State<MainTabScaffold> createState() => _MainTabScaffoldState();
}

class _MainTabScaffoldState extends State<MainTabScaffold> {
  DateTime? _lastBackPress;

  static const _destinations = <_MainTabDestination>[
    _MainTabDestination(label: '消息', icon: AmitiaIconGlyph.messages),
    _MainTabDestination(label: '任务', icon: AmitiaIconGlyph.tasks),
    _MainTabDestination(label: '记忆', icon: AmitiaIconGlyph.memory),
    _MainTabDestination(label: '插件', icon: AmitiaIconGlyph.plugins),
    _MainTabDestination(label: '我的', icon: AmitiaIconGlyph.profile),
  ];

  void _switchToTab(int index) {
    widget.navigationShell.goBranch(
      index,
      initialLocation: widget.navigationShell.currentIndex == index,
    );
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        if (widget.navigationShell.currentIndex != 0) {
          _lastBackPress = null;
          _switchToTab(0);
          return;
        }
        final now = DateTime.now();
        if (_lastBackPress != null &&
            now.difference(_lastBackPress!) < const Duration(seconds: 2)) {
          _lastBackPress = null;
          SystemNavigator.pop();
          return;
        }
        _lastBackPress = now;
        ScaffoldMessenger.of(context)
          ..clearSnackBars()
          ..showSnackBar(
            const SnackBar(
              content: Text('再按一次退出应用'),
              duration: Duration(seconds: 2),
              behavior: SnackBarBehavior.floating,
            ),
          );
      },
      child: Scaffold(
        body: widget.navigationShell,
        bottomNavigationBar: DecoratedBox(
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            border: Border(
              top: BorderSide(color: context.borderPrimary, width: 0.5),
            ),
          ),
          child: SafeArea(
            top: false,
            child: SizedBox(
              height: 56,
              child: Row(
                children: [
                  for (var index = 0; index < _destinations.length; index++)
                    Expanded(
                      child: _MainTabButton(
                        destination: _destinations[index],
                        selected: widget.navigationShell.currentIndex == index,
                        onTap: () => _switchToTab(index),
                      ),
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _MainTabDestination {
  const _MainTabDestination({required this.label, required this.icon});

  final String label;
  final AmitiaIconGlyph icon;
}

class _MainTabButton extends StatelessWidget {
  const _MainTabButton({
    required this.destination,
    required this.selected,
    required this.onTap,
  });

  final _MainTabDestination destination;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final color = selected ? context.textPrimary : context.textTertiary;
    return Semantics(
      key: ValueKey('main-tab-${destination.label}'),
      button: true,
      selected: selected,
      label: destination.label,
      child: InkWell(
        onTap: onTap,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            AmitiaIcon(destination.icon, size: 21, color: color),
            const SizedBox(height: 2),
            Text(
              destination.label,
              style: AppTypography.label(context).copyWith(
                fontSize: 10,
                color: color,
                fontWeight: selected ? FontWeight.w500 : FontWeight.w400,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
