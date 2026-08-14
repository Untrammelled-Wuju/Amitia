import Foundation
import AVFoundation

public class AudioRecorderAdapter: NSObject {
    public static let shared = AudioRecorderAdapter()

    private var audioRecorder: AVAudioRecorder?

    private override init() {
        super.init()
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
