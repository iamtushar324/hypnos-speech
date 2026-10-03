#if HYPNOS_SPEECH
import AppKit
import AVFoundation
import Foundation

/// Explicit local diagnostic: fresh microphone capture + real event tap + paste into a disposable document.
/// No network request, key access, existing recording, selected-text, or screen access.
@MainActor
final class HypnosNativeValidation {
    private var window: NSWindow?
    private let monitor = ShortcutMonitor()

    func run(recorder: HypnosRecorder) async throws -> String {
        guard AVCaptureDevice.authorizationStatus(for: .audio) == .authorized, AXIsProcessTrusted() else {
            return "Local validation needs Microphone and Accessibility permissions first."
        }
        let fm = FileManager.default
        let directory = fm.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("space.hypnos.speech.mac/Validation", isDirectory: true)
        try fm.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        let audio = directory.appendingPathComponent("fresh-\(UUID()).wav")
        guard fm.createFile(atPath: audio.path, contents: Data(), attributes: [.posixPermissions: 0o600]) else {
            throw HypnosError.storage
        }
        try await recorder.startRecording(toOutputFile: audio)
        try await Task.sleep(nanoseconds: 2_000_000_000)
        await recorder.stopRecording()
        try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: audio.path)
        let recorded = try AVAudioFile(forReading: audio)
        let microphonePassed = recorded.length > 0 && recorded.fileFormat.sampleRate == 16000

        var downCount = 0, upCount = 0
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
        let hotkeyPassed = installed && downCount == 1 && upCount == 1

        let textView = NSTextView(frame: NSRect(x: 0, y: 0, width: 600, height: 350))
        textView.isRichText = false
        textView.isAutomaticQuoteSubstitutionEnabled = false
        textView.isAutomaticDashSubstitutionEnabled = false
        textView.font = .monospacedSystemFont(ofSize: 14, weight: .regular)
        let document = NSWindow(contentRect: textView.frame, styleMask: [.titled, .closable, .resizable], backing: .buffered, defer: false)
        document.title = "Hypnos Speech — disposable validation document"
        document.contentView = textView
        document.isReleasedWhenClosed = false
        document.center()
        window = document
        NSApp.activate(ignoringOtherApps: true)
        document.makeKeyAndOrderFront(nil)
        document.makeFirstResponder(textView)
        try await Task.sleep(nanoseconds: 250_000_000)
        let fixture = " \n# Hypnos [keep brackets]\n\nनमस्ते — café 👋\n\n```swift\nlet name = \"Tushar\"\n  print(name)\n```\n\nTrailing spaces  \n"
        let result = await CursorPaster.startPasteAtCursor(fixture, allowAutoLearn: false).value
        try await Task.sleep(nanoseconds: 250_000_000)
        let pastePassed = result.result.didPostPasteCommand && Array(textView.string.utf8) == Array(fixture.utf8)
        var canceled = false
        let canceledPaste = CursorPaster.startPasteAtCursor("SHOULD NEVER PASTE", allowAutoLearn: false, shouldCancel: { canceled })
        // Let the paste prepare its clipboard and enter the native pre-paste delay.
        try await Task.sleep(nanoseconds: 30_000_000)
        canceled = true
        let cancellation = await canceledPaste.value
        try await Task.sleep(nanoseconds: 200_000_000)
        let cancelPassed = !cancellation.result.didPostPasteCommand && Array(textView.string.utf8) == Array(fixture.utf8)
        let report: [String: Any] = [
            "timestamp": ISO8601DateFormatter().string(from: Date()), "microphone_wav": microphonePassed,
            "global_hotkey_synthetic_events": hotkeyPassed, "exact_native_paste": pastePassed,
            "native_paste_cancellation": cancelPassed, "live_hypnos_transcription": false,
            "audio_file": audio.lastPathComponent, "frames": recorded.length
        ]
        let output = directory.appendingPathComponent("latest.json")
        try JSONSerialization.data(withJSONObject: report, options: [.prettyPrinted, .sortedKeys]).write(to: output, options: .atomic)
        try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: output.path)
        return "Local validation: microphone \(microphonePassed ? "PASS" : "FAIL"), hotkey \(hotkeyPassed ? "PASS" : "FAIL"), exact paste \(pastePassed ? "PASS" : "FAIL"), cancellation \(cancelPassed ? "PASS" : "FAIL"). No server upload."
    }
}
#endif
