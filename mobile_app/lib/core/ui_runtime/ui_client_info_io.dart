import 'dart:ffi';

String currentArchitecture() {
  final abi = Abi.current().toString().toLowerCase();
  if (abi.contains('arm64')) return 'arm64';
  if (abi.contains('x64')) return 'x86_64';
  if (abi.contains('ia32')) return 'x86';
  if (abi.contains('arm')) return 'armv7';
  return '';
}
