import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _runtimeSource() => File(
      'lib/features/desktop_pet/runtime/desktop_pet_mobile_runtime.dart',
    ).readAsStringSync();

String _between(String source, String start, String end) {
  final startIndex = source.indexOf(start);
  expect(startIndex, greaterThanOrEqualTo(0), reason: 'missing start marker: $start');
  final endIndex = source.indexOf(end, startIndex + start.length);
  expect(endIndex, greaterThanOrEqualTo(0), reason: 'missing end marker: $end');
  return source.substring(startIndex, endIndex);
}

void main() {
  group('DesktopPetMobileRuntime Runtime V2 regression guards', () {
    test('durable replay is bound to canonical command identity', () {
      final source = _runtimeSource();
      expect(source, contains('commandPayloadHash: commandPayloadHash'));
      expect(source, contains("'commandType': commandType"));
      expect(source, contains("'desiredRevision': desiredRevision"));
      expect(source, contains("'desiredHash': desiredHash"));
      expect(source, contains('runtime command replay payload mismatch'));
      expect(source, contains('runtime command replay type mismatch'));
      expect(source, contains('runtime command replay desired-state mismatch'));
    });

    test('durable physical result is cached before transport acceptance', () {
      final source = _runtimeSource();
      final syncCase = _between(
        source,
        "case 'runtime.command.sync_desired_state':",
        "case 'runtime.command.ensure_absent':",
      );
      final cacheIndex = syncCase.indexOf('_cacheDurableReplay(');
      final ackIndex = syncCase.indexOf(
        "_sendCommandAck(commandId, commandSequence, 'runtime_accepted')",
      );
      expect(cacheIndex, greaterThanOrEqualTo(0));
      expect(ackIndex, greaterThanOrEqualTo(0));
      expect(cacheIndex, lessThan(ackIndex));
      expect(syncCase, contains('durableExecutionSettled = true'));
    });

    test('socket callbacks are fenced to their concrete WebSocket instance', () {
      final source = _runtimeSource();
      expect(source, contains('_handleSocketMessage(data, epoch, socket)'));
      expect(source, contains('_onSocketClosed(epoch, socket, error.toString())'));
      expect(source, contains("_onSocketClosed(epoch, socket, 'socket_closed')"));
      expect(source, contains('epoch != _attachEpoch || _socket != socket'));
    });

    test('connection generation survives reconnect and rejects regression', () {
      final source = _runtimeSource();
      final closeBlock = _between(
        source,
        'Future<void> _onSocketClosed(',
        'void _scheduleReconnect(',
      );
      final disconnectBlock = _between(
        source,
        'void _disconnect(',
        'Future<void> _stopPlaybackLocally(',
      );
      expect(closeBlock, isNot(contains('_connectionGeneration = 0')));
      expect(disconnectBlock, isNot(contains('_connectionGeneration = 0')));
      expect(source, contains('runtime hello_ack connection generation regressed'));
    });

    test('playback status polling is single-flight', () {
      final source = _runtimeSource();
      final pollBlock = _between(
        source,
        'void _startPlaybackPoll(',
        'Future<void> _pollPlayback(',
      );
      expect(pollBlock, contains('tracked.pollInFlight'));
      expect(pollBlock, contains('_pollPlaybackGuarded(tracked)'));
    });

    test('playback transport failure cannot be relabeled as renderer failure', () {
      final source = _runtimeSource();
      final pollBlock = _between(
        source,
        'Future<void> _pollPlaybackGuarded(_TrackedPlayback tracked)',
        'Future<void> _interruptPlayback(',
      );
      expect(pollBlock, contains("_native('desktop.pet.renderer.status')"));
      expect(pollBlock, contains("'errorCode': 'RENDERER_STATUS_FAILED'"));
      expect(pollBlock, contains('playback lifecycle delivery failed'));

      final completionIndex = pollBlock.indexOf(
        "_sendPlaybackEvent(\n      'runtime.playback.action_completed'",
      );
      final localTerminalIndex = pollBlock.lastIndexOf(
        '_playback = null;',
        completionIndex,
      );
      final cursorIndex = pollBlock.lastIndexOf(
        '_cursor.lastProcessedCommandSequence',
        completionIndex,
      );
      expect(completionIndex, greaterThanOrEqualTo(0));
      expect(localTerminalIndex, greaterThanOrEqualTo(0));
      expect(cursorIndex, greaterThanOrEqualTo(0));
      expect(localTerminalIndex, lessThan(completionIndex));
      expect(cursorIndex, lessThan(completionIndex));
    });

    test('started physical playback keeps a poller on acknowledgement loss', () {
      final source = _runtimeSource();
      final playCase = _between(
        source,
        "case 'runtime.command.play_action':",
        "case 'runtime.command.stop_action':",
      );
      expect(playCase, contains('finally {'));
      expect(playCase, contains('_playback == tracked && tracked.pollTimer == null'));
      expect(playCase, contains('_startPlaybackPoll(tracked)'));
    });

    test('drag completion persists position before network event send', () {
      final source = _runtimeSource();
      final flushBlock = _between(
        source,
        'Future<bool> _flushPendingRendererInteractions()',
        'Future<void> _persistDraggedPosition(',
      );
      final persistIndex = flushBlock.indexOf('_persistDraggedPosition(');
      final sendIndex = flushBlock.indexOf(
        "_sendEnvelope('runtime_event', pending.type, pending.payload)",
      );
      expect(persistIndex, greaterThanOrEqualTo(0));
      expect(sendIndex, greaterThanOrEqualTo(0));
      expect(persistIndex, lessThan(sendIndex));
    });

    test('position persistence waiters serialize instead of returning early', () {
      final source = _runtimeSource();
      final positionFlush = _between(
        source,
        'Future<void> _flushPendingPosition()',
        'Future<void> _persistRuntimePosition(',
      );
      expect(positionFlush, contains('_positionFlushSerial.then<void>'));
      expect(positionFlush, contains('onError:'));
      expect(positionFlush, contains('_runPositionFlush()'));
      expect(positionFlush, contains('_pendingPosition = pending'));
      expect(positionFlush, isNot(contains('_persistingPosition')));
    });

    test('position PATCH does not claim physical settings revision', () {
      final source = _runtimeSource();
      final positionBlock = _between(
        source,
        'Future<void> _persistRuntimePosition(',
        'Future<void> _sendPeriodicSnapshotSafely(',
      );
      expect(positionBlock, isNot(contains('_cursor.appliedSettingsRevision')));
      expect(positionBlock, contains('must not advance appliedSettingsRevision here'));
    });

    test('periodic snapshot catches asynchronous transport failures', () {
      final source = _runtimeSource();
      expect(source, contains('unawaited(_sendPeriodicSnapshotSafely())'));
      final helper = _between(
        source,
        'Future<void> _sendPeriodicSnapshotSafely()',
        'Future<void> _sendHeartbeat()',
      );
      expect(helper, contains('try {'));
      expect(helper, contains('catch (error)'));
      expect(helper, contains('await _flushPendingPosition()'));
      expect(helper, contains('await _sendStateSnapshot()'));
    });
  });
}
