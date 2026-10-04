#if VOKIRI
import AppKit
import Combine
import AVFoundation
import CoreAudio
import Foundation

struct AudioMeter: Equatable {
    var averagePower: Double
    var peakPower: Double
}

/// Thin capture-only wrapper around VoiceInk's AUHAL recorder. No media, context, or inference services.
@MainActor
final class VokiriRecorder: ObservableObject {
    private let core = CoreAudioRecorder()
    private let queue = DispatchQueue(label: "space.hypnos.speech.mac.capture", qos: .userInitiated)

    func startRecording(toOutputFile url: URL) async throws {
        var address = AudioObjectPropertyAddress(mSelector: kAudioHardwarePropertyDefaultInputDevice,
                                                 mScope: kAudioObjectPropertyScopeGlobal,
                                                 mElement: kAudioObjectPropertyElementMain)
        var device = AudioDeviceID(0)
        var size = UInt32(MemoryLayout<AudioDeviceID>.size)
        guard AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &address, 0, nil, &size, &device) == noErr,
              device != 0 else { throw VokiriError.emptyAudio }
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            queue.async { [core] in
                do { try core.startRecording(toOutputFile: url, deviceID: device); continuation.resume() }
                catch { continuation.resume(throwing: error) }
            }
        }
    }

    func stopRecording() async {
        await withCheckedContinuation { continuation in
            queue.async { [core] in core.stopRecording(); continuation.resume() }
        }
    }

    func audioMeterSnapshot() -> AudioMeter {
        AudioMeter(averagePower: Double(max(0, min(1, (core.averagePower + 60) / 60))),
                   peakPower: Double(max(0, min(1, (core.peakPower + 60) / 60))))
    }
}

/// Shortcut legacy value decoding remains available; this app does not run upstream migrations.
struct LegacyKeyboardShortcut: Codable {
    let carbonKeyCode: Int
    let carbonModifiers: Int
}

@MainActor
enum VokiriAlert {
    enum Kind { case error, warning, info }
    static func showNotification(title: String, type: Kind, duration: TimeInterval = 2) {
        let alert = NSAlert()
        alert.messageText = title
        alert.alertStyle = .warning
        alert.addButton(withTitle: "OK")
        alert.runModal()
    }
}
#endif
