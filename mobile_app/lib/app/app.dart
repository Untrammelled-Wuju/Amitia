import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/runtime/runtime_bootstrap_provider.dart';
import '../core/runtime/runtime_bootstrap_phase.dart';
import '../core/widgets/amitia_drawer.dart';
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
    final bootstrap = ref.read(runtimeBootstrapProvider);
    await bootstrap.initialize();
    if (mounted) {
      setState(() => _bootstrapInitialized = true);
    }
  }

  @override
  Widget build(BuildContext context) {
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
    final bootstrapAsync = ref.watch(runtimeBootstrapSnapshotProvider);

    return bootstrapAsync.when(
      data: (snapshot) => _buildAppForPhase(context, ref, snapshot.phase),
      loading: () => const _BootstrapInitializingWidget(),
      error: (_, __) => const _BootstrapErrorWidget(),
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
    return const MaterialApp(
      home: Scaffold(
        body: Center(
          child: CircularProgressIndicator(),
        ),
      ),
    );
  }
}

class _BootstrapInstallRequiredWidget extends StatelessWidget {
  const _BootstrapInstallRequiredWidget();

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(
      home: Scaffold(
        body: Center(
          child: Text('Runtime installation required'),
        ),
      ),
    );
  }
}

class _BootstrapFailedWidget extends StatelessWidget {
  const _BootstrapFailedWidget();

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(
      home: Scaffold(
        body: Center(
          child: Text('Runtime startup failed'),
        ),
      ),
    );
  }
}

class _BootstrapUnavailableWidget extends StatelessWidget {
  const _BootstrapUnavailableWidget();

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(
      home: Scaffold(
        body: Center(
          child: Text('Runtime unavailable'),
        ),
      ),
    );
  }
}

class _BootstrapErrorWidget extends StatelessWidget {
  const _BootstrapErrorWidget();

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(
      home: Scaffold(
        body: Center(
          child: Text('Failed to initialize runtime'),
        ),
      ),
    );
  }
}

class AmitiaApp extends ConsumerWidget {
  const AmitiaApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final themeMode = ref.watch(themeModeProvider);
    final router = ref.watch(goRouterProvider);

    return MaterialApp.router(
      title: 'Amitia',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.lightTheme(),
      darkTheme: AppTheme.darkTheme(),
      themeMode: themeMode,
      routerConfig: router,
    );
  }
}
