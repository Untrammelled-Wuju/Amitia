import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';
import 'package:flutter/services.dart';

class MobileDeviceMeshIdentity {
  static const MethodChannel _channel = MethodChannel(
    'com.amitia.device_mesh/identity',
  );

  const MobileDeviceMeshIdentity();

  Future<Map<String, dynamic>> identity() async {
    final raw = await _channel.invokeMapMethod<String, dynamic>('identity');
    if (raw == null) {
      throw StateError('iOS Device Mesh identity unavailable');
    }
    final value = Map<String, dynamic>.from(raw);
    for (final key in const ['deviceId', 'runtimeId', 'publicKey']) {
      if ((value[key] ?? '').toString().trim().isEmpty) {
        throw StateError('iOS Device Mesh identity is incomplete: $key');
      }
    }
    value['platform'] = 'ios';
    return value;
  }

  Future<Map<String, dynamic>> pairingProof({
    required Map<String, dynamic> claimBody,
    required String coreId,
  }) async {
    final identityValue = await identity();
    final publicKey = identityValue['publicKey'].toString().trim();
    final normalizedBody = <String, dynamic>{
      'deviceId': (claimBody['deviceId'] ?? '').toString().trim(),
      'runtimeId': (claimBody['runtimeId'] ?? '').toString().trim(),
      'platform': (claimBody['platform'] ?? '').toString().trim(),
      'label': (claimBody['label'] ?? '').toString().trim(),
      'offerToken': (claimBody['offerToken'] ?? '').toString().trim(),
      'setupCode': (claimBody['setupCode'] ?? '').toString().trim(),
    };
    final bodyHash = sha256
        .convert(utf8.encode(jsonEncode(normalizedBody)))
        .toString();
    final proof = <String, dynamic>{
      'publicKey': publicKey,
      'audience': coreId.trim(),
      'method': 'POST',
      'path': '/api/public/device-mesh/v1/pairing/claim',
      'bodyHash': bodyHash,
      'issuedAt': DateTime.now().toUtc().millisecondsSinceEpoch ~/ 1000,
      'nonce': _nonce(),
      'signature': '',
    };
    final bytes = utf8.encode(jsonEncode(proof));
    final signature = await _channel.invokeMethod<String>('sign', <String, dynamic>{
      'data': base64Encode(bytes),
    });
    if (signature == null || signature.trim().isEmpty) {
      throw StateError('iOS Device Mesh identity signature unavailable');
    }
    proof['signature'] = signature.trim();
    return proof;
  }

  String _nonce() {
    final random = Random.secure();
    final bytes = Uint8List.fromList(
      List<int>.generate(24, (_) => random.nextInt(256)),
    );
    return base64UrlEncode(bytes).replaceAll('=', '');
  }
}
