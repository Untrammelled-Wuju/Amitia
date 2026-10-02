import 'dart:async';
import 'package:amitia_app/core/settings/background_preferences.dart';
import 'package:amitia_app/core/widgets/background_media.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:video_player_platform_interface/video_player_platform_interface.dart';

class _VideoPlatform extends VideoPlayerPlatform {
  final streams = <int, StreamController<VideoEvent>>{};
  final playing = <int, bool>{};
  final volumes = <int, double>{};
  final looping = <int, bool>{};
  final disposed = <int>[];
  @override
  Future<void> init() async {}
  @override
  Future<int?> createWithOptions(VideoCreationOptions options) async {
    final id = streams.length + 1;
    streams[id] = StreamController<VideoEvent>.broadcast();
    return id;
  }

  @override
  Stream<VideoEvent> videoEventsFor(int playerId) {
    scheduleMicrotask(
      () => streams[playerId]!.add(
        VideoEvent(
          eventType: VideoEventType.initialized,
          duration: const Duration(seconds: 10),
          size: const Size(640, 480),
        ),
      ),
    );
    return streams[playerId]!.stream;
  }

  @override
  Future<void> setMixWithOthers(bool value) async {}
  @override
  Future<void> setLooping(int playerId, bool value) async {
    looping[playerId] = value;
  }

  @override
  Future<void> setVolume(int playerId, double value) async {
    volumes[playerId] = value;
  }

  @override
  Future<void> setPlaybackSpeed(int playerId, double value) async {}
  @override
  Future<void> play(int playerId) async {
    playing[playerId] = true;
  }

  @override
  Future<void> pause(int playerId) async {
    playing[playerId] = false;
  }

  @override
  Future<Duration> getPosition(int playerId) async => Duration.zero;
  @override
  Future<void> seekTo(int playerId, Duration position) async {}
  @override
  Widget buildViewWithOptions(VideoViewOptions options) =>
      const SizedBox.expand();
  @override
  Future<void> dispose(int playerId) async {
    disposed.add(playerId);
    await streams[playerId]?.close();
  }
}

void main() {
  testWidgets(
    'video stays muted, pauses in background and reduced motion, and releases replaced players',
    (tester) async {
      final previousPlatform = VideoPlayerPlatform.instance;
      final platform = _VideoPlatform();
      VideoPlayerPlatform.instance = platform;
      addTearDown(() => VideoPlayerPlatform.instance = previousPlatform);
      Future<void> render({
        String path = '/clip.mp4',
        bool animate = true,
        bool disabled = false,
      }) async {
        await tester.pumpWidget(
          MaterialApp(
            home: MediaQuery(
              data: MediaQueryData(disableAnimations: disabled),
              child: SizedBox.expand(
                child: BackgroundMedia(
                  preferences: BackgroundPreferences(
                    enabled: true,
                    kind: BackgroundMediaKind.video,
                    path: path,
                  ),
                  baseColor: Colors.white,
                  animate: animate,
                ),
              ),
            ),
          ),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 10));
        await tester.idle();
        await tester.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 1)),
        );
      }

      await render();
      expect(platform.volumes[1], 0);
      expect(platform.looping[1], true);
      expect(platform.playing[1], true);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      await tester.pump();
      expect(platform.playing[1], false);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pump();
      expect(platform.playing[1], true);
      await render(disabled: true);
      expect(platform.playing[1], false);
      await render(path: '/second.mp4', animate: false);
      expect(
        tester
            .widget<BackgroundMedia>(find.byType(BackgroundMedia))
            .preferences
            .path,
        '/second.mp4',
      );
      expect(platform.streams.keys, [1, 2]);
      expect(platform.disposed, contains(1));
      expect(platform.playing[2], false);
      await tester.pumpWidget(const SizedBox());
      await tester.pump();
      await tester.idle();
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 1)),
      );
      expect(platform.disposed, contains(2));
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'blur and opacity apply to the media layer while controls remain clickable',
    (tester) async {
      var tapped = false;
      await tester.pumpWidget(
        MaterialApp(
          home: SizedBox.expand(
            child: BackgroundMedia(
              preferences: const BackgroundPreferences(
                path: '/photo.png',
                opacity: 0.4,
                blurEnabled: true,
                blurRadius: 15,
              ),
              baseColor: Colors.white,
              child: Center(
                child: TextButton(
                  onPressed: () => tapped = true,
                  child: const Text('操作'),
                ),
              ),
            ),
          ),
        ),
      );
      expect(tester.widget<Opacity>(find.byType(Opacity)).opacity, 0.4);
      expect(
        tester.widget<ImageFiltered>(find.byType(ImageFiltered)).enabled,
        true,
      );
      await tester.tap(find.text('操作'));
      expect(tapped, true);
      expect(tester.takeException(), isNull);
    },
  );
}
