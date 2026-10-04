#if VOKIRI
import AppKit
import AVFoundation
import Foundation

/// Explicit local diagnostic: fresh microphone capture + real event tap + paste into a disposable document.
/// No network request, key access, existing recording, selected-text, or screen access.
@MainActor
final class VokiriNativeValidation {
    private var window: NSWindow?
    private let monitor = ShortcutMonitor()

    private func capture(recorder: VokiriRecorder, seconds: Int, onStarted: () -> Void, onStopped: () -> Void) async throws -> (URL, AVAudioFile, Double) {
        let fm = FileManager.default
        let directory = fm.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("space.hypnos.speech.mac/Validation", isDirectory: true)
        try fm.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let audio = directory.appendingPathComponent("fresh-\(UUID()).wav")
        guard fm.createFile(atPath: audio.path, contents: Data(), attributes: [.posixPermissions: 0o600]) else {
            throw VokiriError.storage
        }
        try await recorder.startRecording(toOutputFile: audio)
        try Task.checkCancellation()
        onStarted()
        var peak = 0.0
        for _ in 0..<(seconds * 10) {
            try await Task.sleep(nanoseconds: 100_000_000)
            peak = max(peak, recorder.audioMeterSnapshot().peakPower)
        }
        await recorder.stopRecording()
        try Task.checkCancellation()
        onStopped()
        try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: audio.path)
        return (audio, try AVAudioFile(forReading: audio), peak)
    }

    func runMicrophone(recorder: VokiriRecorder, onStarted: () -> Void, onStopped: () -> Void) async throws -> String {
        let (audio, recorded, peak) = try await capture(recorder: recorder, seconds: 5, onStarted: onStarted, onStopped: onStopped)
        let passed = recorded.length > 0 && recorded.fileFormat.sampleRate == 16000
        let report: [String: Any] = ["timestamp": ISO8601DateFormatter().string(from: Date()),
            "microphone_wav": passed, "frames": recorded.length, "sample_rate": recorded.fileFormat.sampleRate,
            "peak_input_level": peak, "audio_file": audio.lastPathComponent, "server_upload": false]
        let output = audio.deletingLastPathComponent().appendingPathComponent("microphone.json")
        try JSONSerialization.data(withJSONObject: report, options: [.prettyPrinted, .sortedKeys]).write(to: output, options: .atomic)
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: output.path)
        return passed ? "Microphone test passed — fresh 16 kHz WAV captured locally. Nothing uploaded." : "Microphone test failed — no usable audio frames."
    }

    func run(recorder: VokiriRecorder, shouldCancel: @escaping @MainActor () -> Bool, onStarted: () -> Void, onStopped: () -> Void) async throws -> String {
        guard AVCaptureDevice.authorizationStatus(for: .audio) == .authorized, AXIsProcessTrusted() else {
            return "Local validation needs Microphone and Accessibility permissions first."
        }
        let fm = FileManager.default
        let directory = fm.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("space.hypnos.speech.mac/Validation", isDirectory: true)
        try fm.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let (audio, recorded, _) = try await capture(recorder: recorder, seconds: 2, onStarted: onStarted, onStopped: onStopped)
        let microphonePassed = recorded.length > 0 && recorded.fileFormat.sampleRate == 16000

        var downCount = 0, upCount = 0
        defer { monitor.stop() }
        let installed = monitor.start(
            shortcuts: [.secondaryRecording: .key(keyCode: 111, modifierFlags: [.control, .option])],
            onShortcutDown: { _, _ in downCount += 1 }, onShortcutUp: { _, _ in upCount += 1 })
        if installed {
            for down in [true, false] {
                let event = CGEvent(keyboardEventSource: CGEventSource(stateID: .privateState), virtualKey: 111, keyDown: down)
                event?.flags = [.maskControl, .maskAlternate]
                event?.post(tap: .cghidEventTap)
                try await Task.sleep(nanoseconds: 100_000_000)
            }
        }
        monitor.stop()
        try Task.checkCancellation()
        let hotkeyPassed = installed && downCount == 1 && upCount == 1

        let textView = NSTextView(frame: NSRect(x: 0, y: 0, width: 600, height: 350))
        textView.isRichText = false
        textView.isAutomaticQuoteSubstitutionEnabled = false
        textView.isAutomaticDashSubstitutionEnabled = false
        textView.font = .monospacedSystemFont(ofSize: 14, weight: .regular)
        let document = NSWindow(contentRect: textView.frame, styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
        document.title = "Vokiri — disposable validation document"
        document.contentView = textView
        document.isReleasedWhenClosed = false
        document.center()
        window = document
        NSApp.activate(ignoringOtherApps: true)
        document.makeKeyAndOrderFront(nil)
        document.makeFirstResponder(textView)
        try await Task.sleep(nanoseconds: 250_000_000)
        let fixture = " \n# Vokiri [keep brackets]\n\nनमस्ते — café 👋\n\n```swift\nlet name = \"Tushar\"\n  print(name)\n```\n\nTrailing spaces  \n"
        try Task.checkCancellation()
        let result = await CursorPaster.startPasteAtCursor(fixture, allowAutoLearn: false, shouldCancel: shouldCancel).value
        try await Task.sleep(nanoseconds: 250_000_000)
        let pastePassed = result.result.didPostPasteCommand && Array(textView.string.utf8) == Array(fixture.utf8)
        var canceled = false
        let canceledPaste = CursorPaster.startPasteAtCursor("SHOULD NEVER PASTE", allowAutoLearn: false, shouldCancel: { canceled || shouldCancel() })
        // Let the paste prepare its clipboard and enter the native pre-paste delay.
        try await Task.sleep(nanoseconds: 30_000_000)
        canceled = true
        let cancellation = await canceledPaste.value
        try await Task.sleep(nanoseconds: 200_000_000)
        let cancelPassed = !cancellation.result.didPostPasteCommand && Array(textView.string.utf8) == Array(fixture.utf8)
        try Task.checkCancellation()
        let report: [String: Any] = [
            "timestamp": ISO8601DateFormatter().string(from: Date()), "microphone_wav": microphonePassed,
            "global_hotkey_synthetic_events": hotkeyPassed, "exact_native_paste": pastePassed,
            "native_paste_cancellation": cancelPassed, "live_vokiri_transcription": false,
            "audio_file": audio.lastPathComponent, "frames": recorded.length
        ]
        let output = directory.appendingPathComponent("latest.json")
        try JSONSerialization.data(withJSONObject: report, options: [.prettyPrinted, .sortedKeys]).write(to: output, options: .atomic)
        try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: output.path)
        return "Local validation: microphone \(microphonePassed ? "PASS" : "FAIL"), hotkey \(hotkeyPassed ? "PASS" : "FAIL"), exact paste \(pastePassed ? "PASS" : "FAIL"), cancellation \(cancelPassed ? "PASS" : "FAIL"). No server upload."
    }
}
#endif
