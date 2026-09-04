import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';

void main() {
  group('BackendConnectionCredential', () {
    group('tryCreate', () {
      test('accepts valid 32-char token', () {
        final token = 'a' * 32;
        final credential = BackendConnectionCredential.tryCreate(token);
        expect(credential, isNotNull);
      });

      test('accepts token longer than 32 chars', () {
        final token = 'a' * 64;
        final credential = BackendConnectionCredential.tryCreate(token);
        expect(credential, isNotNull);
      });

      test('trims whitespace before validation', () {
        final token = '  ${'a' * 32}  ';
        final credential = BackendConnectionCredential.tryCreate(token);
        expect(credential, isNotNull);
      });

      test('rejects token shorter than 32 chars', () {
        final token = 'a' * 31;
        final credential = BackendConnectionCredential.tryCreate(token);
        expect(credential, isNull);
      });

      test('rejects empty token', () {
        final credential = BackendConnectionCredential.tryCreate('');
        expect(credential, isNull);
      });

      test('rejects whitespace-only token', () {
        final credential = BackendConnectionCredential.tryCreate('   ');
        expect(credential, isNull);
      });

      test('rejects token containing NUL', () {
        final token = '${'a' * 16}\u0000${'a' * 16}';
        final credential = BackendConnectionCredential.tryCreate(token);
        expect(credential, isNull);
      });

      test('rejects token containing CR', () {
        final token = '${'a' * 16}\r${'a' * 16}';
        final credential = BackendConnectionCredential.tryCreate(token);
        expect(credential, isNull);
      });

      test('rejects token containing LF', () {
        final token = '${'a' * 16}\n${'a' * 16}';
        final credential = BackendConnectionCredential.tryCreate(token);
        expect(credential, isNull);
      });
    });

    group('revealForTransport', () {
      test('returns the original token', () {
        final token = 'b' * 40;
        final credential = BackendConnectionCredential.tryCreate(token)!;
        expect(credential.revealForTransport(), token);
      });

      test('returns trimmed token', () {
        final token = '  ${'c' * 32}  ';
        final credential = BackendConnectionCredential.tryCreate(token)!;
        expect(credential.revealForTransport(), 'c' * 32);
      });
    });

    group('toString', () {
      test('does not expose token', () {
        final token = 'secret-token-value-that-is-long-enough-1234567890';
        final credential = BackendConnectionCredential.tryCreate(token)!;
        final str = credential.toString();
        expect(str, isNot(contains(token)));
        expect(str, contains('REDACTED'));
      });
    });
  });
}
