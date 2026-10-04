import Foundation

/// Setup state is independent of recording state: a blocked shortcut must never look like capture.
struct VokiriReadiness: Equatable {
    var deviceKeyConfigured: Bool
    var microphoneGranted: Bool
    var accessibilityGranted: Bool
    var hotkeyInstalled: Bool

    var recordingBlocker: String? {
        if !microphoneGranted { return "Microphone is off. Grant Microphone in Vokiri → Settings." }
        if !accessibilityGranted { return "macOS has not granted this build Accessibility. Enable Vokiri, or refresh its entry if already enabled." }
        if !deviceKeyConfigured { return "Device key is missing. Enter your dedicated Mac key in Vokiri → AI Models and save." }
        return nil
    }

    var hotkeyStatus: String {
        if !accessibilityGranted { return "Needs Accessibility permission" }
        return hotkeyInstalled ? "Listening for your shortcut" : "Not active — refresh the shortcut or relaunch the app"
    }
}
