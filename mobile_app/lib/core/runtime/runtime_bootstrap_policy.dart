class RuntimeBootstrapPolicy {
  /// Legacy compatibility switch. Runtime startup is now exclusively owned by
  /// MobileBackendLifecycle, so this value is intentionally not acted on by
  /// DefaultRuntimeBootstrap.
  @Deprecated('Runtime startup is owned by MobileBackendLifecycle')
  final bool autoStartInstalledRuntime;

  final bool autoInstallRuntime;
  final Duration installTimeout;

  const RuntimeBootstrapPolicy({
    this.autoStartInstalledRuntime = false,
    this.autoInstallRuntime = true,
    this.installTimeout = const Duration(seconds: 120),
  });
}
