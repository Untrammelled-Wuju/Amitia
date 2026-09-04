import AVFoundation
import CoreImage
import Flutter
import ReplayKit
import UIKit

final class RealtimeVisualAdapter: NSObject, FlutterStreamHandler, AVCaptureVideoDataOutputSampleBufferDelegate {
  static let shared = RealtimeVisualAdapter()

  private let controlChannelName = "com.amitia.realtime_visual/control"
  private let frameChannelName = "com.amitia.realtime_visual/frames"
  private let captureQueue = DispatchQueue(label: "com.amitia.realtime.visual.capture", qos: .userInitiated)
  private let encodeQueue = DispatchQueue(label: "com.amitia.realtime.visual.encode", qos: .utility)
  private let ciContext = CIContext(options: nil)

  private var eventSink: FlutterEventSink?
  private var cameraSession: AVCaptureSession?
  private var cameraOutput: AVCaptureVideoDataOutput?
  private var cameraPosition: AVCaptureDevice.Position = .front
  private var cameraSequence: Int64 = 0
  private var screenSequence: Int64 = 0
  private var lastCameraFrameAt: TimeInterval = 0
  private var lastScreenFrameAt: TimeInterval = 0
  private var forceCameraFrame = false
  private var forceScreenFrame = false
  private var screenActive = false

  private override init() {
    super.init()
  }

  func registerRealtimeChannels(messenger: FlutterBinaryMessenger) {
    let control = FlutterMethodChannel(name: controlChannelName, binaryMessenger: messenger)
    control.setMethodCallHandler { [weak self] call, result in
      self?.handle(call: call, result: result)
    }
    let frames = FlutterEventChannel(name: frameChannelName, binaryMessenger: messenger)
    frames.setStreamHandler(self)
  }

  func onListen(withArguments arguments: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? {
    eventSink = events
    return nil
  }

  func onCancel(withArguments arguments: Any?) -> FlutterError? {
    eventSink = nil
    return nil
  }

  private func handle(call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "startCamera":
      let arguments = call.arguments as? [String: Any]
      cameraPosition = (arguments?["facing"] as? String)?.lowercased() == "back" ? .back : .front
      startCamera(result: result)
    case "stopCamera":
      stopCamera()
      result(nil)
    case "switchCamera":
      cameraPosition = cameraPosition == .front ? .back : .front
      let wasRunning = cameraSession?.isRunning == true
      stopCamera()
      if wasRunning { startCamera(result: result) } else { result(nil) }
    case "startScreen":
      startScreen(result: result)
    case "stopScreen":
      stopScreen()
      result(nil)
    case "requestImmediateFrame":
      let source = (call.arguments as? [String: Any])?["source"] as? String
      if source == "camera" { forceCameraFrame = true }
      if source == "screen" { forceScreenFrame = true }
      result(nil)
    case "status":
      result([
        "cameraActive": cameraSession?.isRunning == true,
        "screenActive": screenActive,
        "cameraSupported": true,
        "screenSupported": RPScreenRecorder.shared().isAvailable,
        "crossAppScreenSupported": false,
      ])
    case "reset":
      stopCamera()
      stopScreen()
      result(nil)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func startCamera(result: @escaping FlutterResult) {
    switch AVCaptureDevice.authorizationStatus(for: .video) {
    case .authorized:
      configureCamera(result: result)
    case .notDetermined:
      AVCaptureDevice.requestAccess(for: .video) { [weak self] granted in
        DispatchQueue.main.async {
          if granted { self?.configureCamera(result: result) }
          else { result(FlutterError(code: "CAMERA_PERMISSION_DENIED", message: "Camera permission denied", details: nil)) }
        }
      }
    default:
      result(FlutterError(code: "CAMERA_PERMISSION_DENIED", message: "Camera permission denied", details: nil))
    }
  }

  private func configureCamera(result: @escaping FlutterResult) {
    captureQueue.async { [weak self] in
      guard let self else { return }
      do {
        let session = AVCaptureSession()
        session.beginConfiguration()
        if session.canSetSessionPreset(.hd1280x720) { session.sessionPreset = .hd1280x720 }
        guard let device = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: self.cameraPosition) ?? AVCaptureDevice.default(for: .video) else {
          throw NSError(domain: "AmitiaRealtimeVisual", code: 1, userInfo: [NSLocalizedDescriptionKey: "No camera device available"])
        }
        let input = try AVCaptureDeviceInput(device: device)
        guard session.canAddInput(input) else {
          throw NSError(domain: "AmitiaRealtimeVisual", code: 2, userInfo: [NSLocalizedDescriptionKey: "Unable to add camera input"])
        }
        session.addInput(input)
        let output = AVCaptureVideoDataOutput()
        output.alwaysDiscardsLateVideoFrames = true
        output.videoSettings = [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA]
        output.setSampleBufferDelegate(self, queue: self.captureQueue)
        guard session.canAddOutput(output) else {
          throw NSError(domain: "AmitiaRealtimeVisual", code: 3, userInfo: [NSLocalizedDescriptionKey: "Unable to add camera output"])
        }
        session.addOutput(output)
        session.commitConfiguration()
        self.cameraSession = session
        self.cameraOutput = output
        self.forceCameraFrame = true
        session.startRunning()
        DispatchQueue.main.async { result(nil) }
      } catch {
        DispatchQueue.main.async {
          result(FlutterError(code: "CAMERA_START_FAILED", message: error.localizedDescription, details: nil))
        }
      }
    }
  }

  private func stopCamera() {
    captureQueue.async { [weak self] in
      guard let self else { return }
      self.cameraOutput?.setSampleBufferDelegate(nil, queue: nil)
      self.cameraSession?.stopRunning()
      self.cameraOutput = nil
      self.cameraSession = nil
    }
  }

  func captureOutput(_ output: AVCaptureOutput, didOutput sampleBuffer: CMSampleBuffer, from connection: AVCaptureConnection) {
    let now = CACurrentMediaTime()
    if !forceCameraFrame && now - lastCameraFrameAt < 0.65 { return }
    forceCameraFrame = false
    lastCameraFrameAt = now
    cameraSequence += 1
    encodeAndEmit(sampleBuffer: sampleBuffer, source: "camera", sequence: cameraSequence, quality: 0.72, maxWidth: 1024, maxHeight: 768)
  }

  private func startScreen(result: @escaping FlutterResult) {
    let recorder = RPScreenRecorder.shared()
    guard recorder.isAvailable else {
      result(FlutterError(code: "SCREEN_UNAVAILABLE", message: "Screen capture is unavailable", details: nil))
      return
    }
    if screenActive {
      result(nil)
      return
    }
    forceScreenFrame = true
    recorder.startCapture(handler: { [weak self] sampleBuffer, sampleType, error in
      guard let self, error == nil, sampleType == .video else { return }
      let now = CACurrentMediaTime()
      if !self.forceScreenFrame && now - self.lastScreenFrameAt < 0.7 { return }
      self.forceScreenFrame = false
      self.lastScreenFrameAt = now
      self.screenSequence += 1
      self.encodeAndEmit(sampleBuffer: sampleBuffer, source: "screen", sequence: self.screenSequence, quality: 0.78, maxWidth: 1280, maxHeight: 800)
    }, completionHandler: { [weak self] error in
      DispatchQueue.main.async {
        if let error {
          result(FlutterError(code: "SCREEN_START_FAILED", message: error.localizedDescription, details: nil))
        } else {
          self?.screenActive = true
          result(nil)
        }
      }
    })
  }

  private func stopScreen() {
    guard screenActive || RPScreenRecorder.shared().isRecording else { return }
    screenActive = false
    RPScreenRecorder.shared().stopCapture { _ in }
  }

  private func encodeAndEmit(sampleBuffer: CMSampleBuffer, source: String, sequence: Int64, quality: CGFloat, maxWidth: CGFloat, maxHeight: CGFloat) {
    guard let pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }
    let image = CIImage(cvPixelBuffer: pixelBuffer)
    let extent = image.extent
    guard extent.width > 0, extent.height > 0 else { return }
    let scale = min(1, min(maxWidth / extent.width, maxHeight / extent.height))
    let transformed = scale < 0.999 ? image.transformed(by: CGAffineTransform(scaleX: scale, y: scale)) : image
    encodeQueue.async { [weak self] in
      guard let self,
            let cgImage = self.ciContext.createCGImage(transformed, from: transformed.extent),
            let data = UIImage(cgImage: cgImage).jpegData(compressionQuality: quality),
            data.count <= 2 * 1024 * 1024 else { return }
      let payload: [String: Any] = [
        "source": source,
        "sequence": sequence,
        "capturedAtMs": Int64(Date().timeIntervalSince1970 * 1000),
        "mime": "image/jpeg",
        "width": cgImage.width,
        "height": cgImage.height,
        "data": FlutterStandardTypedData(bytes: data),
      ]
      DispatchQueue.main.async { self.eventSink?(payload) }
    }
  }
}
