import Foundation
import AVFoundation
import Flutter

public class AudioRecorderAdapter: NSObject, FlutterStreamHandler {
    public static let shared = AudioRecorderAdapter()

    private var audioRecorder: AVAudioRecorder?

    private var realtimeEngine: AVAudioEngine?
    private var realtimePlayer: AVAudioPlayerNode?
    private var captureConverter: AVAudioConverter?
    private var captureSink: FlutterEventSink?
    private var captureTapInstalled = false
    private var realtimeControlChannel: FlutterMethodChannel?
    private var realtimeEventChannel: FlutterEventChannel?

    private let captureSampleRate: Double = 16_000
    private let playbackSampleRate: Double = 24_000

    private override init() {
        super.init()
    }

    public func registerRealtimeChannels(messenger: FlutterBinaryMessenger) {
        if realtimeControlChannel != nil { return }

        let control = FlutterMethodChannel(
            name: "com.amitia.realtime_audio/control",
            binaryMessenger: messenger
        )
        control.setMethodCallHandler { [weak self] call, result in
            guard let self = self else {
                result(FlutterError(code: "AUDIO_UNAVAILABLE", message: "Realtime audio adapter unavailable", details: nil))
                return
            }
            self.handleRealtimeCall(call, result: result)
        }
        realtimeControlChannel = control

        let events = FlutterEventChannel(
            name: "com.amitia.realtime_audio/input",
            binaryMessenger: messenger
        )
        events.setStreamHandler(self)
        realtimeEventChannel = events
    }

    private func handleRealtimeCall(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "startCapture":
            Task { @MainActor in
                let allowed = await self.requestPermission()
                guard allowed else {
                    result(FlutterError(code: "MICROPHONE_PERMISSION_DENIED", message: "Microphone permission denied", details: nil))
                    return
                }
                do {
                    try self.startRealtimeCapture()
                    result(nil)
                } catch {
                    result(FlutterError(code: "AUDIO_START_FAILED", message: error.localizedDescription, details: nil))
                }
            }
        case "stopCapture":
            Task { @MainActor in
                self.stopRealtimeCapture()
                result(nil)
            }
        case "playPcm":
            guard let typed = call.arguments as? FlutterStandardTypedData else {
                result(FlutterError(code: "INVALID_AUDIO", message: "playPcm requires Uint8List PCM data", details: nil))
                return
            }
            Task { @MainActor in
                do {
                    try self.playRealtimePCM(typed.data)
                    result(nil)
                } catch {
                    result(FlutterError(code: "AUDIO_PLAYBACK_FAILED", message: error.localizedDescription, details: nil))
                }
            }
        case "reset":
            Task { @MainActor in
                self.resetRealtimeAudio()
                result(nil)
            }
        case "status":
            result([
                "capturing": captureTapInstalled,
                "playing": realtimePlayer?.isPlaying ?? false,
            ])
        default:
            result(FlutterMethodNotImplemented)
        }
    }

    public func onListen(withArguments arguments: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? {
        captureSink = events
        return nil
    }

    public func onCancel(withArguments arguments: Any?) -> FlutterError? {
        captureSink = nil
        return nil
    }

    @MainActor
    private func configureRealtimeSession() throws {
        let session = AVAudioSession.sharedInstance()
        try session.setCategory(
            .playAndRecord,
            mode: .voiceChat,
            options: [.defaultToSpeaker, .allowBluetooth]
        )
        try session.setPreferredIOBufferDuration(0.02)
        try session.setActive(true, options: [])
    }

    @MainActor
    private func ensureRealtimeEngine() throws -> AVAudioEngine {
        if let engine = realtimeEngine {
            if !engine.isRunning {
                try engine.start()
            }
            return engine
        }

        try configureRealtimeSession()
        let engine = AVAudioEngine()
        let player = AVAudioPlayerNode()
        engine.attach(player)
        let playbackFormat = AVAudioFormat(
            standardFormatWithSampleRate: playbackSampleRate,
            channels: 1
        )!
        engine.connect(player, to: engine.mainMixerNode, format: playbackFormat)
        engine.prepare()
        try engine.start()
        realtimeEngine = engine
        realtimePlayer = player
        return engine
    }

    @MainActor
    private func startRealtimeCapture() throws {
        if captureTapInstalled { return }
        let engine = try ensureRealtimeEngine()
        let input = engine.inputNode
        let sourceFormat = input.outputFormat(forBus: 0)
        guard sourceFormat.sampleRate > 0, sourceFormat.channelCount > 0 else {
            throw NSError(domain: "AmitiaRealtimeAudio", code: 1, userInfo: [NSLocalizedDescriptionKey: "Microphone input format unavailable"])
        }
        guard let targetFormat = AVAudioFormat(
            commonFormat: .pcmFormatInt16,
            sampleRate: captureSampleRate,
            channels: 1,
            interleaved: true
        ), let converter = AVAudioConverter(from: sourceFormat, to: targetFormat) else {
            throw NSError(domain: "AmitiaRealtimeAudio", code: 2, userInfo: [NSLocalizedDescriptionKey: "Unable to create 16 kHz PCM converter"])
        }
        captureConverter = converter

        input.installTap(onBus: 0, bufferSize: 2048, format: sourceFormat) { [weak self] buffer, _ in
            self?.emitConvertedCapture(buffer: buffer, targetFormat: targetFormat)
        }
        captureTapInstalled = true
    }

    private func emitConvertedCapture(buffer: AVAudioPCMBuffer, targetFormat: AVAudioFormat) {
        guard let converter = captureConverter else { return }
        let ratio = targetFormat.sampleRate / buffer.format.sampleRate
        let capacity = AVAudioFrameCount(max(64.0, Double(buffer.frameLength) * ratio + 64.0))
        guard let output = AVAudioPCMBuffer(pcmFormat: targetFormat, frameCapacity: capacity) else { return }

        var supplied = false
        var conversionError: NSError?
        let status = converter.convert(to: output, error: &conversionError) { _, outStatus in
            if supplied {
                outStatus.pointee = .noDataNow
                return nil
            }
            supplied = true
            outStatus.pointee = .haveData
            return buffer
        }
        guard conversionError == nil,
              status != .error,
              output.frameLength > 0,
              let samples = output.int16ChannelData?[0] else {
            return
        }
        let byteCount = Int(output.frameLength) * MemoryLayout<Int16>.size
        let data = Data(bytes: samples, count: byteCount)
        DispatchQueue.main.async { [weak self] in
            self?.captureSink?(FlutterStandardTypedData(bytes: data))
        }
    }

    @MainActor
    private func stopRealtimeCapture() {
        guard captureTapInstalled else { return }
        realtimeEngine?.inputNode.removeTap(onBus: 0)
        captureTapInstalled = false
        captureConverter = nil
    }

    @MainActor
    private func playRealtimePCM(_ data: Data) throws {
        guard !data.isEmpty else { return }
        _ = try ensureRealtimeEngine()
        guard let player = realtimePlayer,
              let format = AVAudioFormat(standardFormatWithSampleRate: playbackSampleRate, channels: 1) else {
            throw NSError(domain: "AmitiaRealtimeAudio", code: 3, userInfo: [NSLocalizedDescriptionKey: "Playback engine unavailable"])
        }
        let frameCount = data.count / MemoryLayout<Int16>.size
        guard frameCount > 0,
              let buffer = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: AVAudioFrameCount(frameCount)),
              let output = buffer.floatChannelData?[0] else {
            return
        }
        buffer.frameLength = AVAudioFrameCount(frameCount)
        data.withUnsafeBytes { rawBuffer in
            let input = rawBuffer.bindMemory(to: Int16.self)
            for index in 0..<frameCount {
                output[index] = Float(input[index]) / 32768.0
            }
        }
        player.scheduleBuffer(buffer, completionHandler: nil)
        if !player.isPlaying {
            player.play()
        }
    }

    @MainActor
    private func resetRealtimeAudio() {
        stopRealtimeCapture()
        realtimePlayer?.stop()
        realtimeEngine?.stop()
        if let player = realtimePlayer, let engine = realtimeEngine {
            engine.detach(player)
        }
        realtimePlayer = nil
        realtimeEngine = nil
        captureConverter = nil
        do {
            try AVAudioSession.sharedInstance().setActive(false, options: [.notifyOthersOnDeactivation])
        } catch {
            // The OS can refuse deactivation while another media operation owns the session.
        }
    }

    public func startRecording(to url: URL, settings: [String: Any]) -> Bool {
        let audioSession = AVAudioSession.sharedInstance()
        do {
            try audioSession.setCategory(.record, mode: .default)
            try audioSession.setActive(true)
            audioRecorder = try AVAudioRecorder(url: url, settings: settings)
            return audioRecorder?.record() ?? false
        } catch {
            return false
        }
    }

    public func stopRecording() {
        audioRecorder?.stop()
        audioRecorder = nil
    }

    public func isRecording() -> Bool {
        return audioRecorder?.isRecording ?? false
    }

    public func requestPermission() async -> Bool {
        let status = AVCaptureDevice.authorizationStatus(for: .audio)
        if status == .authorized {
            return true
        }
        if status == .notDetermined {
            return await AVCaptureDevice.requestAccess(for: .audio)
        }
        return false
    }
}
