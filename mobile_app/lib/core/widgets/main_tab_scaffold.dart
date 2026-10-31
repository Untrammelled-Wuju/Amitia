import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/app_routes.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_typography.dart';
import '../services/providers.dart';
import 'amitia_icons.dart';

class MainTabScaffold extends ConsumerStatefulWidget {
  const MainTabScaffold({super.key, required this.navigationShell});

  final StatefulNavigationShell navigationShell;

  @override
  ConsumerState<MainTabScaffold> createState() => _MainTabScaffoldState();
}

class _MainTabScaffoldState extends ConsumerState<MainTabScaffold> {
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

  Future<bool> _handleBackButton() async {
    final router = GoRouter.of(context);
    final location = router.routerDelegate.currentConfiguration.fullPath;
    final path = Uri.tryParse(location)?.path ?? location;
    final mainTabPaths = <String>{
      AppRoutes.conversations,
      AppRoutes.tasks,
      AppRoutes.memoryHub,
      AppRoutes.extensions,
      AppRoutes.me,
    };
    if (!mainTabPaths.contains(path) || router.canPop()) return false;

    final backHandler = ref.read(conversationBackHandlerProvider);
    if (widget.navigationShell.currentIndex == 0 &&
        backHandler.onBack?.call() == true) {
      backHandler.resetExitWindow();
      return true;
    }
    if (widget.navigationShell.currentIndex != 0) {
      backHandler.resetExitWindow();
      _switchToTab(0);
      return true;
    }
    if (backHandler.shouldExit(
      DateTime.now(),
      const Duration(seconds: 2),
    )) {
      await SystemNavigator.pop();
      return true;
    }
    ScaffoldMessenger.of(context)
      ..clearSnackBars()
      ..showSnackBar(
        const SnackBar(
          content: Text('再按一次退出应用'),
          duration: Duration(seconds: 2),
          behavior: SnackBarBehavior.floating,
        ),
      );
    return true;
  }

  @override
  Widget build(BuildContext context) {
    return BackButtonListener(
      onBackButtonPressed: _handleBackButton,
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
