import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/native_bridge/providers/native_bridge_relay_bootstrap_provider.dart';
import '../core/runtime/runtime_bootstrap_provider.dart';
import '../core/runtime/runtime_bootstrap_phase.dart';
import '../core/runtime/runtime_bootstrap_snapshot.dart';
import '../core/runtime/runtime_bridge_provider.dart';
import '../core/runtime/backend/mobile_backend_providers.dart';
import '../core/runtime/backend/mobile_deployment_mode.dart';
import '../core/debug/debug_log_overlay.dart';
import '../core/debug/debug_runtime_bridge.dart';
import '../core/ui_runtime/ui_provider.dart';
import '../core/ui_runtime/ui_runtime_controller.dart';
import '../core/ui_runtime/ui_theme.dart';
import '../core/settings/appearance_preferences.dart';
import '../features/desktop_pet/runtime/desktop_pet_mobile_runtime.dart';
import '../features/extensions/presentation/widgets/mcp_interaction_guard.dart';
import 'theme/app_theme.dart';
import 'theme/app_motion.dart';
import 'theme/design_tokens.dart';
import 'router.dart';

class AmitiaAppRoot extends ConsumerStatefulWidget {
  const AmitiaAppRoot({super.key});

  @override
  ConsumerState<AmitiaAppRoot> createState() => _AmitiaAppRootState();
}

class _AmitiaAppRootState extends ConsumerState<AmitiaAppRoot> {
  bool _bootstrapInitialized = false;

  @override
  void initState() {
    super.initState();
    _initializeBootstrap();
  }

  Future<void> _initializeBootstrap() async {
    await ref.read(appearancePreferencesProvider.notifier).init();
    final deploymentNotifier = ref.read(
      mobileDeploymentConfigProvider.notifier,
    );
    await deploymentNotifier.init();

    final runtimeBootstrap = ref.read(runtimeBootstrapProvider);
    await runtimeBootstrap.initialize();
    final bootstrapSnapshot = await runtimeBootstrap.snapshots.first;

    final config = ref.read(mobileDeploymentConfigProvider);
    final lifecycle = ref.read(mobileBackendLifecycleProvider);
    final localBootstrapBlocked =
        config.mode == MobileDeploymentMode.local &&
        (bootstrapSnapshot.phase == RuntimeBootstrapPhase.installRequired ||
            bootstrapSnapshot.phase == RuntimeBootstrapPhase.failed ||
            bootstrapSnapshot.phase == RuntimeBootstrapPhase.unavailable);
    if (!localBootstrapBlocked) {
      await lifecycle.reconcile(config);
    }

    if (mounted) {
      setState(() => _bootstrapInitialized = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(debugRuntimeLogBridgeProvider);
    return _BootstrapGate(bootstrapInitialized: _bootstrapInitialized);
  }
}

class _BootstrapGate extends ConsumerWidget {
  final bool bootstrapInitialized;

  const _BootstrapGate({required this.bootstrapInitialized});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    Widget content;
    if (!bootstrapInitialized) {
      content = const _BootstrapInitializingWidget();
    } else {
      final deployment = ref.watch(mobileDeploymentConfigProvider);

      if (deployment.mode == MobileDeploymentMode.cloud) {
        content = const AmitiaApp();
      } else {
        final bootstrapAsync = ref.watch(runtimeBootstrapSnapshotProvider);
        content = bootstrapAsync.when(
          data: (snapshot) => _buildAppForSnapshot(snapshot),
          loading: () => const _BootstrapInitializingWidget(),
          error: (_, __) => const _BootstrapErrorWidget(),
        );
      }
    }

    return Directionality(textDirection: TextDirection.ltr, child: content);
  }

  Widget _buildAppForSnapshot(RuntimeBootstrapSnapshot snapshot) {
    switch (snapshot.phase) {
      case RuntimeBootstrapPhase.ready:
      case RuntimeBootstrapPhase.stopped:
      case RuntimeBootstrapPhase.starting:
      case RuntimeBootstrapPhase.stopping:
        return const AmitiaApp();
      case RuntimeBootstrapPhase.installRequired:
        return const _BootstrapInstallRequiredWidget();
      case RuntimeBootstrapPhase.failed:
        return _BootstrapFailedWidget(message: snapshot.error?.message);
      case RuntimeBootstrapPhase.unavailable:
        return const _BootstrapUnavailableWidget();
      case RuntimeBootstrapPhase.initializing:
        return const _BootstrapInitializingWidget();
    }
  }
}

class _BootstrapInitializingWidget extends StatelessWidget {
  const _BootstrapInitializingWidget();

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      home: Stack(
        children: [
          const Scaffold(body: Center(child: CircularProgressIndicator())),
          const DebugLogOverlay(),
        ],
      ),
    );
  }
}

class _BootstrapInstallRequiredWidget extends ConsumerStatefulWidget {
  const _BootstrapInstallRequiredWidget();

  @override
  ConsumerState<_BootstrapInstallRequiredWidget> createState() =>
      _BootstrapInstallRequiredWidgetState();
}

class _BootstrapInstallRequiredWidgetState
    extends ConsumerState<_BootstrapInstallRequiredWidget> {
  bool _installing = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _installRuntime());
  }

  Future<void> _installRuntime() async {
    if (_installing) return;

    setState(() {
      _installing = true;
      _errorMessage = null;
    });

    try {
      final bridge = ref.read(runtimeBridgeProvider);
      final result = await bridge.install();
      if (!mounted) return;

      if (result.error != null) {
        setState(() => _errorMessage = result.error!.message);
      } else if (!result.accepted) {
        setState(() => _errorMessage = '安装命令未被接受');
      } else {
        final config = ref.read(mobileDeploymentConfigProvider);
        unawaited(ref.read(mobileBackendLifecycleProvider).reconcile(config));
      }
    } catch (e) {
      if (mounted) {
        setState(() => _errorMessage = '安装失败: $e');
      }
    } finally {
      if (mounted) {
        setState(() => _installing = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      home: Stack(
        children: [
          Scaffold(
            body: Center(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  const Text('正在准备运行环境'),
                  const SizedBox(height: 16),
                  if (_installing)
                    const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  if (_errorMessage != null) ...[
                    const SizedBox(height: 16),
                    Text(
                      _errorMessage!,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 16),
                    ElevatedButton(
                      onPressed: _installRuntime,
                      child: const Text('重试'),
                    ),
                  ],
                ],
              ),
            ),
          ),
          const DebugLogOverlay(),
        ],
      ),
    );
  }
}

class _BootstrapFailedWidget extends StatelessWidget {
  final String? message;

  const _BootstrapFailedWidget({this.message});

  @override
  Widget build(BuildContext context) {
    final detail = message?.trim();
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      home: Stack(
        children: [
          Scaffold(
            body: Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Text('Runtime startup failed'),
                    if (detail != null && detail.isNotEmpty) ...[
                      const SizedBox(height: 12),
                      Text(detail, textAlign: TextAlign.center),
                    ],
                  ],
                ),
              ),
            ),
          ),
          const DebugLogOverlay(),
        ],
      ),
    );
  }
}

class _BootstrapUnavailableWidget extends StatelessWidget {
  const _BootstrapUnavailableWidget();

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      home: Stack(
        children: [
          const Scaffold(body: Center(child: Text('Runtime unavailable'))),
          const DebugLogOverlay(),
        ],
      ),
    );
  }
}

class _BootstrapErrorWidget extends StatelessWidget {
  const _BootstrapErrorWidget();

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      home: Stack(
        children: [
          const Scaffold(
            body: Center(child: Text('Failed to initialize runtime')),
          ),
          const DebugLogOverlay(),
        ],
      ),
    );
  }
}

class AmitiaApp extends ConsumerWidget {
  const AmitiaApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(nativeBridgeRelayBootstrapProvider);
    ref.watch(desktopPetMobileRuntimeBootstrapProvider);
    final appearance = ref.watch(appearancePreferencesProvider);
    AppMotion.setAnimationsEnabled(appearance.dynamicEffect && !appearance.reduceAnimation);
    final runtimeSnapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    if (runtimeSnapshot == null) {
      Future.microtask(
        () => ref.read(uiRuntimeProvider.notifier).ensureLoaded(),
      );
    }
    final visualProviders = <UIProviderDefinition?>[
      runtimeSnapshot?.resolve(UICapability.theme),
      runtimeSnapshot?.resolve(UICapability.tokens),
      runtimeSnapshot?.resolve(UICapability.icons),
      runtimeSnapshot?.resolve(UICapability.components),
    ];
    final router = ref.watch(goRouterProvider);
    ThemeData applyVisualProviders(ThemeData theme) {
      var result = theme;
      for (final provider in visualProviders) {
        result = applyUIThemeProvider(result, provider);
      }
      return result;
    }

    ThemeData applyAppearance(ThemeData theme) {
      final colors = theme.extension<AmitiaColorTokens>();
      final layout = theme.extension<AmitiaLayoutTokens>() ?? const AmitiaLayoutTokens();
      if (colors == null) return theme;

      const lightAccents = [
        Color(0xFF8A5728),
        Color(0xFF6C8FEA),
        Color(0xFF52B788),
        Color(0xFFE9A23B),
      ];
      const darkAccents = [
        Color(0xFF9C8068),
        Color(0xFF8CA8F0),
        Color(0xFF78C99A),
        Color(0xFFF0B65D),
      ];
      final accent = (theme.brightness == Brightness.dark ? darkAccents : lightAccents)[appearance.accentColorIndex];
      final accentPressed = Color.lerp(
        accent,
        theme.brightness == Brightness.dark ? Colors.white : Colors.black,
        0.16,
      )!;
      final accentSoft = Color.alphaBlend(accent.withValues(alpha: 0.14), colors.surfacePrimary);
      final userColors = colors.copyWith(
        accentPrimary: accent,
        accentSecondary: accent,
        accentPressed: accentPressed,
        accentSoft: accentSoft,
      );

      final userLayout = switch (appearance.cornerStyleIndex) {
        0 => layout.copyWith(radiusExtraSmall: 6, radiusSmall: 8, radiusMedium: 12, radiusLarge: 16, radiusTag: 8),
        2 => layout.copyWith(radiusExtraSmall: 10, radiusSmall: 16, radiusMedium: 20, radiusLarge: 28, radiusTag: 14),
        _ => layout.copyWith(radiusExtraSmall: 8, radiusSmall: 12, radiusMedium: 16, radiusLarge: 22, radiusTag: 10),
      };
      DesignTokenRuntime.activateLayout(userLayout);
      final extensions = theme.extensions.values
          .where((item) => item is! AmitiaColorTokens && item is! AmitiaLayoutTokens)
          .toList(growable: true)
        ..add(userColors)
        ..add(userLayout);
      return theme.copyWith(
        colorScheme: theme.colorScheme.copyWith(
          primary: accent,
          primaryContainer: accentSoft,
          secondary: accent,
          secondaryContainer: accentSoft,
        ),
        extensions: extensions,
      );
    }

    final lightTheme = applyAppearance(applyVisualProviders(AppTheme.lightTheme()));
    final darkTheme = applyAppearance(applyVisualProviders(AppTheme.darkTheme()));

    return MaterialApp.router(
      title: 'Amitia',
      debugShowCheckedModeBanner: false,
      theme: lightTheme,
      darkTheme: darkTheme,
      themeMode: appearance.themeMode,
      themeAnimationDuration: (!appearance.dynamicEffect || appearance.reduceAnimation) ? Duration.zero : const Duration(milliseconds: 200),
      routerConfig: router,
      builder: (context, child) {
        final media = MediaQuery.of(context);
        final systemScale = media.textScaler.scale(1.0);
        final effectiveScale = (systemScale * appearance.fontScale).clamp(0.8, 2.0).toDouble();
        final adjustedMedia = media.copyWith(
          textScaler: TextScaler.linear(effectiveScale),
          disableAnimations: media.disableAnimations || !appearance.dynamicEffect || appearance.reduceAnimation,
        );
        return MediaQuery(
          data: adjustedMedia,
          child: Stack(
            children: [
              if (child case final Widget currentChild) currentChild,
              const McpInteractionGuard(),
              const DebugLogOverlay(),
            ],
          ),
        );
      },
    );
  }
}
