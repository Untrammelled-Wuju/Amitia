import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/native_bridge/providers/native_bridge_relay_bootstrap_provider.dart';
import '../core/runtime/runtime_bootstrap_provider.dart';
import '../core/runtime/runtime_bootstrap_phase.dart';
import '../core/runtime/runtime_bridge_provider.dart';
import '../core/runtime/backend/mobile_backend_providers.dart';
import '../core/runtime/backend/mobile_deployment_mode.dart';
import '../core/widgets/amitia_drawer.dart';
import '../core/debug/debug_log_overlay.dart';
import '../core/debug/debug_runtime_bridge.dart';
import 'theme/app_theme.dart';
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
    final deploymentNotifier = ref.read(mobileDeploymentConfigProvider.notifier);
    await deploymentNotifier.init();

    final runtimeBootstrap = ref.read(runtimeBootstrapProvider);
    await runtimeBootstrap.initialize();

    final config = ref.read(mobileDeploymentConfigProvider);
    final lifecycle = ref.read(mobileBackendLifecycleProvider);
    await lifecycle.reconcile(config);

    if (mounted) {
      setState(() => _bootstrapInitialized = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.watch(debugRuntimeLogBridgeProvider);
    return _BootstrapGate(
      bootstrapInitialized: _bootstrapInitialized,
    );
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

      if (deployment.mode == MobileDeploymentMode.cloud ||
          deployment.mode == MobileDeploymentMode.hybrid) {
        content = const AmitiaApp();
      } else {
        final bootstrapAsync = ref.watch(runtimeBootstrapSnapshotProvider);
        content = bootstrapAsync.when(
          data: (snapshot) => _buildAppForPhase(context, ref, snapshot.phase),
          loading: () => const _BootstrapInitializingWidget(),
          error: (_, __) => const _BootstrapErrorWidget(),
        );
      }
    }

    return Directionality(
      textDirection: TextDirection.ltr,
      child: content,
    );
  }

  Widget _buildAppForPhase(
    BuildContext context,
    WidgetRef ref,
    RuntimeBootstrapPhase phase,
  ) {
    switch (phase) {
      case RuntimeBootstrapPhase.ready:
      case RuntimeBootstrapPhase.stopped:
      case RuntimeBootstrapPhase.starting:
      case RuntimeBootstrapPhase.stopping:
        return const AmitiaApp();
      case RuntimeBootstrapPhase.initializing:
        return const _BootstrapInitializingWidget();
      case RuntimeBootstrapPhase.installRequired:
        return const _BootstrapInstallRequiredWidget();
      case RuntimeBootstrapPhase.failed:
        return const _BootstrapFailedWidget();
      case RuntimeBootstrapPhase.unavailable:
        return const _BootstrapUnavailableWidget();
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
          const Scaffold(
            body: Center(
              child: CircularProgressIndicator(),
            ),
          ),
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
                  const Text('运行环境尚未安装'),
                  const SizedBox(height: 16),
                  ElevatedButton(
                    onPressed: _installing ? null : _installRuntime,
                    child: _installing
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('安装运行环境'),
                  ),
                  if (_errorMessage != null) ...[
                    const SizedBox(height: 16),
                    Text(
                      _errorMessage!,
                      style: TextStyle(color: Theme.of(context).colorScheme.error),
                      textAlign: TextAlign.center,
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
  const _BootstrapFailedWidget();

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      home: Stack(
        children: [
          const Scaffold(
            body: Center(
              child: Text('Runtime startup failed'),
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
          const Scaffold(
            body: Center(
              child: Text('Runtime unavailable'),
            ),
          ),
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
            body: Center(
              child: Text('Failed to initialize runtime'),
            ),
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
    final themeMode = ref.watch(themeModeProvider);
    final router = ref.watch(goRouterProvider);

    return MaterialApp.router(
      title: 'Amitia',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.lightTheme(),
      darkTheme: AppTheme.darkTheme(),
      themeMode: themeMode,
      routerConfig: router,
      builder: (context, child) {
        return Stack(
          children: [
            if (child != null) child,
            const DebugLogOverlay(),
          ],
        );
      },
    );
  }
}
