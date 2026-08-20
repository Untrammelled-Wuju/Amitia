class RuntimeBootstrapPolicy {
  final bool autoStartInstalledRuntime;
  final bool autoInstallRuntime;
  final Duration installTimeout;

  const RuntimeBootstrapPolicy({
    this.autoStartInstalledRuntime = true,
    this.autoInstallRuntime = true,
    this.installTimeout = const Duration(seconds: 120),
  });
}
