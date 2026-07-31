import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../app/app_routes.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';

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

class AmitiaScaffold extends StatelessWidget {
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
  Widget build(BuildContext context) {
    return Scaffold(
      body: body,
      appBar: appBar,
      floatingActionButton: floatingActionButton,
      bottomNavigationBar: bottomNavigationBar,
      drawer: drawer,
      backgroundColor: backgroundColor ?? context.backgroundPrimary,
      resizeToAvoidBottomInset: resizeToAvoidBottomInset,
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
  });

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight);

  @override
  Widget build(BuildContext context) {
    Widget? effectiveLeading = leading;
    if (effectiveLeading == null) {
      if (navigation == AmitiaAppBarNavigation.back || showBackButton) {
        effectiveLeading = IconButton(
          icon: const Icon(Icons.arrow_back_ios_new, size: 20),
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go(fallbackRoute);
            }
          },
        );
      } else if (navigation == AmitiaAppBarNavigation.drawer) {
        effectiveLeading = IconButton(
          icon: const Icon(Icons.menu, size: 22),
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
      title: titleWidget ?? (title != null ? Text(title!, style: AppTypography.pageTitle(context)) : null),
      actions: actions,
      leading: effectiveLeading,
      centerTitle: centerTitle,
      elevation: elevation,
      scrolledUnderElevation: 0,
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
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: margin,
        padding: padding ?? const EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: backgroundColor ?? context.surfacePrimary,
          borderRadius: borderRadius ?? AppRadius.brMedium,
          border: border ?? Border.all(color: context.borderPrimary, width: 0.5),
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
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(title, style: AppTypography.sectionTitle(context)),
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
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 14),
        decoration: BoxDecoration(
          color: isSelected ? context.accentSoft : Colors.transparent,
          borderRadius: AppRadius.brSmall,
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
                      padding: const EdgeInsets.only(top: 2),
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
