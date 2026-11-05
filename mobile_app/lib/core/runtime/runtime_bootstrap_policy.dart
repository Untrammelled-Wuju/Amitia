class RuntimeBootstrapPolicy {
  final bool autoInstallRuntime;
  final Duration installTimeout;

  const RuntimeBootstrapPolicy({
    this.autoInstallRuntime = true,
    this.installTimeout = const Duration(minutes: 10),
  });
}
