import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../app/app_routes.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../app/theme/design_tokens.dart';

enum AmitiaAppBarNavigation {
  drawer,
  back,
  none,
}

class ShellDrawerScope extends InheritedWidget {
  final VoidCallback openDrawer;
  const ShellDrawerScope({super.key, required this.openDrawer, required super.child});
  static ShellDrawerScope? of(BuildContext context) {
    return context.dependOnInheritedWidgetOfExactType<ShellDrawerScope>();
  }
  @override
  bool updateShouldNotify(ShellDrawerScope oldWidget) => openDrawer != oldWidget.openDrawer;
}

class AmitiaScaffold extends StatefulWidget {
  final Widget? body;
  final PreferredSizeWidget? appBar;
  final Widget? floatingActionButton;
  final Widget? bottomNavigationBar;
  final Widget? drawer;
  final Color? backgroundColor;
  final bool resizeToAvoidBottomInset;

  const AmitiaScaffold({
    super.key,
    this.body,
    this.appBar,
    this.floatingActionButton,
    this.bottomNavigationBar,
    this.drawer,
    this.backgroundColor,
    this.resizeToAvoidBottomInset = true,
  });

  @override
  State<AmitiaScaffold> createState() => _AmitiaScaffoldState();
}

class _AmitiaScaffoldState extends State<AmitiaScaffold> {
  DateTime? _lastBackPress;

  void _showToast(String msg) {
    ScaffoldMessenger.of(context).clearSnackBars();
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(msg),
        duration: const Duration(seconds: 2),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final router = GoRouter.of(context);
    final currentLocation = router.routerDelegate.currentConfiguration.fullPath;
    final isChatPage = currentLocation == AppRoutes.chat;
    final isTopLevelPage = currentLocation == '/onboarding' || currentLocation == '/login' || currentLocation == '/privacy';
    final canPop = router.canPop();

    return PopScope(
      canPop: canPop,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        if (canPop) return;
        if (isChatPage || isTopLevelPage) {
          final now = DateTime.now();
          if (_lastBackPress != null && now.difference(_lastBackPress!) < const Duration(seconds: 2)) {
            _lastBackPress = null;
            Navigator.of(context).maybePop();
          } else {
            _lastBackPress = now;
            _showToast('再按一次退出应用');
          }
        } else if (currentLocation != '/') {
          router.go(AppRoutes.chat);
        } else {
          Navigator.of(context).maybePop();
        }
      },
      child: Scaffold(
        body: widget.body,
        appBar: widget.appBar,
        floatingActionButton: widget.floatingActionButton,
        bottomNavigationBar: widget.bottomNavigationBar,
        drawer: widget.drawer,
        backgroundColor: widget.backgroundColor ?? context.backgroundPrimary,
        resizeToAvoidBottomInset: widget.resizeToAvoidBottomInset,
      ),
    );
  }
}

class AmitiaAppBar extends StatelessWidget implements PreferredSizeWidget {
  final String? title;
  final Widget? titleWidget;
  final List<Widget>? actions;
  final Widget? leading;
  final bool showBackButton;
  final AmitiaAppBarNavigation navigation;
  final bool centerTitle;
  final double elevation;
  final String fallbackRoute;
  final PreferredSizeWidget? bottom;

  const AmitiaAppBar({
    super.key,
    this.title,
    this.titleWidget,
    this.actions,
    this.leading,
    this.showBackButton = false,
    this.navigation = AmitiaAppBarNavigation.none,
    this.centerTitle = false,
    this.elevation = 0,
    this.fallbackRoute = AppRoutes.chat,
    this.bottom,
  });

  @override
  Size get preferredSize => Size.fromHeight(
        DesignTokenRuntime.components.toolbarHeight + (bottom?.preferredSize.height ?? 0));

  @override
  Widget build(BuildContext context) {
    Widget? effectiveLeading = leading;
    if (effectiveLeading == null) {
      if (navigation == AmitiaAppBarNavigation.back || showBackButton) {
        effectiveLeading = IconButton(
          icon: const Icon(Icons.arrow_back_ios_new, size: 18),
          constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          style: IconButton.styleFrom(
            backgroundColor: Colors.transparent,
            side: BorderSide.none,
            foregroundColor: context.textPrimary,
          ),
          onPressed: () {
            final router = GoRouter.of(context);
            if (router.canPop()) {
              router.pop();
            } else {
              router.go(fallbackRoute);
            }
          },
        );
      } else if (navigation == AmitiaAppBarNavigation.drawer ||
          (navigation == AmitiaAppBarNavigation.none &&
              !showBackButton &&
              ShellDrawerScope.of(context) != null)) {
        effectiveLeading = IconButton(
          icon: const Icon(Icons.menu, size: 20),
          constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          style: IconButton.styleFrom(
            backgroundColor: Colors.transparent,
            side: BorderSide.none,
            foregroundColor: context.textPrimary,
          ),
          onPressed: () {
            final scope = ShellDrawerScope.of(context);
            if (scope != null) {
              scope.openDrawer();
            } else {
              Scaffold.of(context).openDrawer();
            }
          },
        );
      }
    }
    return AppBar(
      toolbarHeight: context.uiComponents.toolbarHeight,
      iconTheme: IconThemeData(size: context.uiIcons.navigation),
      title: titleWidget ?? (title != null ? Text(title!, style: AppTypography.pageTitle(context)) : null),
      actions: actions,
      leading: effectiveLeading,
      centerTitle: centerTitle,
      elevation: elevation,
      scrolledUnderElevation: 0,
      surfaceTintColor: Colors.transparent,
      bottom: bottom,
    );
  }
}

class AmitiaCard extends StatelessWidget {
  final Widget child;
  final EdgeInsets? padding;
  final EdgeInsets? margin;
  final VoidCallback? onTap;
  final Color? backgroundColor;
  final Border? border;
  final BorderRadius? borderRadius;

  const AmitiaCard({
    super.key,
    required this.child,
    this.padding,
    this.margin,
    this.onTap,
    this.backgroundColor,
    this.border,
    this.borderRadius,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('card');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final paddingX = number('paddingX', AppSpacing.cardPadding);
    final paddingY = number('paddingY', AppSpacing.cardPadding);
    final radius = number('radius', 20);
    final borderWidth = number('borderWidth', context.uiComponents.borderWidth);
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: margin,
        padding: padding ?? EdgeInsets.symmetric(horizontal: paddingX, vertical: paddingY),
        decoration: BoxDecoration(
          color: backgroundColor ?? context.surfacePrimary,
          borderRadius: borderRadius ?? BorderRadius.circular(radius),
          border: border ?? Border.all(color: context.borderPrimary, width: borderWidth),
        ),
        child: child,
      ),
    );
  }
}

class AmitiaSectionHeader extends StatelessWidget {
  final String title;
  final String? actionText;
  final VoidCallback? onAction;

  const AmitiaSectionHeader({
    super.key,
    required this.title,
    this.actionText,
    this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            title,
            style: AppTypography.sectionTitle(context).copyWith(
              color: context.textTertiary,
              fontSize: 11,
              fontWeight: FontWeight.w700,
              letterSpacing: .15,
            ),
          ),
          if (actionText != null)
            GestureDetector(
              onTap: onAction,
              child: Text(actionText!, style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500)),
            ),
        ],
      ),
    );
  }
}

class AmitiaListTile extends StatelessWidget {
  final Widget? leading;
  final String title;
  final String? subtitle;
  final Widget? trailing;
  final VoidCallback? onTap;
  final bool isSelected;

  const AmitiaListTile({
    super.key,
    this.leading,
    required this.title,
    this.subtitle,
    this.trailing,
    this.onTap,
    this.isSelected = false,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('listTile');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Container(
        constraints: BoxConstraints(minHeight: number('minHeight', context.uiLayout.listItemMinHeight)),
        padding: EdgeInsets.symmetric(
          horizontal: number('paddingX', 13),
          vertical: number('paddingY', 12),
        ),
        decoration: BoxDecoration(
          color: isSelected ? context.accentSoft : Colors.transparent,
          borderRadius: BorderRadius.circular(number('radius', 14)),
        ),
        child: Row(
          children: [
            if (leading != null) ...[
              leading!,
              const SizedBox(width: 12),
            ],
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.body(context)),
                  if (subtitle != null)
                    Padding(
                      padding: EdgeInsets.only(top: 2),
                      child: Text(subtitle!, style: AppTypography.caption(context)),
                    ),
                ],
              ),
            ),
            ?trailing,
          ],
        ),
      ),
    );
  }
}
