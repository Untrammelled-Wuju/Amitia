import 'dart:io';

import 'package:crypto/crypto.dart';

String certificatePublicKeyInfoSha256(List<int> certificateDer) {
  final certificate = _readDerValue(certificateDer, 0);
  if (certificate.tag != 0x30 || certificate.end != certificateDer.length) {
    throw const FormatException('中心证书 DER 结构无效');
  }
  final tbs = _readDerValue(certificateDer, certificate.contentStart);
  if (tbs.tag != 0x30 || tbs.end > certificate.contentEnd) {
    throw const FormatException('中心证书 TBSCertificate 结构无效');
  }
  var cursor = tbs.contentStart;
  var field = _readDerValue(certificateDer, cursor);
  if (field.tag == 0xa0) {
    cursor = field.end;
  }
  for (var index = 0; index < 5; index++) {
    field = _readDerValue(certificateDer, cursor);
    cursor = field.end;
  }
  final publicKeyInfo = _readDerValue(certificateDer, cursor);
  if (publicKeyInfo.tag != 0x30 || publicKeyInfo.end > tbs.contentEnd) {
    throw const FormatException('中心证书 SubjectPublicKeyInfo 结构无效');
  }
  return sha256
      .convert(certificateDer.sublist(publicKeyInfo.start, publicKeyInfo.end))
      .toString();
}

HttpClient createCertificatePinnedHttpClient(String fingerprint) {
  final pin = _decodeFingerprint(fingerprint);
  final client = HttpClient();
  if (pin != null) {
    client.badCertificateCallback = (certificate, host, port) {
      try {
        final actual = _decodeFingerprint(
          certificatePublicKeyInfoSha256(certificate.der),
        );
        if (actual == null || actual.length != pin.length) return false;
        var difference = 0;
        for (var index = 0; index < pin.length; index++) {
          difference |= pin[index] ^ actual[index];
        }
        return difference == 0;
      } catch (_) {
        return false;
      }
    };
  }
  return client;
}

List<int>? _decodeFingerprint(String value) {
  final normalized = value.trim().toLowerCase();
  if (normalized.length != 64) return null;
  final result = <int>[];
  for (var index = 0; index < normalized.length; index += 2) {
    final byte = int.tryParse(
      normalized.substring(index, index + 2),
      radix: 16,
    );
    if (byte == null) return null;
    result.add(byte);
  }
  return result;
}

({int tag, int start, int contentStart, int contentEnd, int end}) _readDerValue(
  List<int> bytes,
  int offset,
) {
  if (offset < 0 || offset >= bytes.length) {
    throw const FormatException('中心证书 DER 长度越界');
  }
  final start = offset;
  final tag = bytes[offset++];
  if (offset >= bytes.length) {
    throw const FormatException('中心证书 DER 缺少长度');
  }
  var length = bytes[offset++];
  if (length & 0x80 != 0) {
    final count = length & 0x7f;
    if (count == 0 || count > 4 || offset + count > bytes.length) {
      throw const FormatException('中心证书 DER 长度编码无效');
    }
    length = 0;
    for (var index = 0; index < count; index++) {
      length = (length << 8) | bytes[offset++];
    }
  }
  final end = offset + length;
  if (end < offset || end > bytes.length) {
    throw const FormatException('中心证书 DER 内容越界');
  }
  return (
    tag: tag,
    start: start,
    contentStart: offset,
    contentEnd: end,
    end: end,
  );
}
