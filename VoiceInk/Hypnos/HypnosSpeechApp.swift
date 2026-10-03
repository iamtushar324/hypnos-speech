#if HYPNOS_SPEECH
import AppKit
import AVFoundation
import SwiftUI

@main
struct HypnosSpeechApp: App {
    @StateObject private var controller = HypnosController()

    var body: some Scene {
        Window("Hypnos Speech", id: "hypnos-settings") {
            HypnosMainView(controller: controller)
                .frame(minWidth: AppWindowLayout.width, minHeight: AppWindowLayout.minimumHeight)
        }
        .defaultSize(width: AppWindowLayout.width, height: AppWindowLayout.minimumHeight)
        MenuBarExtra("Hypnos Speech", systemImage: controller.menuBarSymbol) {
            HypnosMenu(controller: controller)
        }
    }
}

@MainActor
final class HypnosController: ObservableObject {
    static let keyAccount = "speech-device-key"
    let recorder = HypnosRecorder()
    @Published var configuration: HypnosConfiguration
    @Published var status = "Ready — Hypnos final-output mode"
    @Published var recordingState: RecordingState = .idle
    @Published var history: [HypnosRecording] = []
    @Published var hotkeyAvailable = false
    @Published var hasDeviceKey = false
    @Published var microphoneGranted = false
    @Published var accessibilityGranted = false
    @Published var indicatorMessage: String?
    @Published var shortcutPressCount = 0
    @Published var pushToTalk: Bool {
        didSet {
            UserDefaults.standard.set(pushToTalk, forKey: "HypnosPushToTalk")
            shortcutMonitor.updateStandaloneModifierActions(pushToTalk ? [] : [.primaryRecording])
        }
    }
    private var workflow: HypnosWorkflow?
    private var currentRecording: HypnosRecording?
    private var recordingGeneration = UUID()
    private var stopWhenStarted = false
    @Published private var captureTransition = false
    private let shortcutMonitor = ShortcutMonitor()
    private var shortcutObserver: NSObjectProtocol?
    private var appObserver: NSObjectProtocol?
    private var indicator: MiniRecorderPanel?
    private var lastTarget: NSRunningApplication?
    private var pasteTarget: NSRunningApplication?
    private var permissionTimer: Timer?
    private let nativeValidation = HypnosNativeValidation()
    private var validationTask: Task<String, Error>?
    private var feedbackTask: Task<Void, Never>?
    private var readinessSnapshot: Data?
    var busy: Bool { captureTransition || recordingState != .idle || workflow?.isBusy == true }
    var readiness: HypnosReadiness {
        HypnosReadiness(deviceKeyConfigured: hasDeviceKey, microphoneGranted: microphoneGranted,
                        accessibilityGranted: accessibilityGranted, hotkeyInstalled: hotkeyAvailable)
    }
    var menuBarSymbol: String {
        if recordingState == .recording { return "mic.fill" }
        if busy { return "ellipsis.circle" }
        return readiness.recordingBlocker == nil && hotkeyAvailable ? "waveform" : "exclamationmark.mic"
    }
    var recordButtonTitle: String {
        if recordingState == .recording { return validationTask != nil ? "Stop Test" : "Stop & Transcribe" }
        return "Start Recording"
    }

    init() {
        let defaults = UserDefaults.standard
        configuration = HypnosConfiguration(
            endpoint: defaults.string(forKey: "HypnosEndpoint") ?? HypnosConfiguration.defaultEndpoint,
            model: defaults.string(forKey: "HypnosModel") ?? "whisper-large-v3-turbo",
            profileID: defaults.string(forKey: "HypnosProfileID") ?? ""
        )
        pushToTalk = defaults.bool(forKey: "HypnosPushToTalk")
        // Preferences belong to the app's own bundle domain. No upstream migrations or model setup.
        defaults.register(defaults: ["restoreClipboardAfterPaste": true, "clipboardRestoreDelay": 0.5,
                                     "AutoLearnEnabled": false])
        hasDeviceKey = KeychainService.shared.exists(forKey: Self.keyAccount, syncable: false)
        do {
            let store = try HypnosStore()
            try store.recoverInterrupted()
            let workflow = HypnosWorkflow(store: store)
            self.workflow = workflow
            workflow.onChange = { [weak self] in self?.refreshWorkflow() }
            refreshHistory()
        } catch { status = HypnosError.storage.localizedDescription }
        ShortcutStore.seedShortcut(.key(keyCode: 49, modifierFlags: [.control, .option]), for: .primaryRecording)
        shortcutObserver = NotificationCenter.default.addObserver(forName: ShortcutStore.shortcutDidChange, object: nil, queue: .main) { [weak self] _ in
            Task { @MainActor in self?.refreshHotkey() }
        }
        appObserver = NSWorkspace.shared.notificationCenter.addObserver(forName: NSWorkspace.didActivateApplicationNotification, object: nil, queue: .main) { [weak self] note in
            guard let app = note.userInfo?[NSWorkspace.applicationUserInfoKey] as? NSRunningApplication,
                  app.bundleIdentifier != Bundle.main.bundleIdentifier else { return }
            Task { @MainActor in self?.lastTarget = app }
        }
        if let app = NSWorkspace.shared.frontmostApplication, app.bundleIdentifier != Bundle.main.bundleIdentifier { lastTarget = app }
        refreshHotkey()
        if let blocker = readiness.recordingBlocker { status = blocker }
        permissionTimer = Timer.scheduledTimer(withTimeInterval: 2, repeats: true) { [weak self] _ in
            Task { @MainActor in
                guard let self else { return }
                self.refreshPermissions()
                if self.accessibilityGranted && !self.hotkeyAvailable { self.refreshHotkey() }
                self.writeReadiness()
            }
        }
    }

    func refreshHotkey() {
        refreshPermissions()
        defer { writeReadiness() }
        guard accessibilityGranted else { shortcutMonitor.stop(); hotkeyAvailable = false; return }
        guard let shortcut = ShortcutStore.shortcut(for: .primaryRecording) else {
            shortcutMonitor.stop()
            hotkeyAvailable = false
            return
        }
        hotkeyAvailable = shortcutMonitor.start(
            shortcuts: [.primaryRecording: shortcut],
            standaloneModifierActions: pushToTalk ? [] : [.primaryRecording],
            onShortcutDown: { [weak self] _, _ in
                Task { @MainActor in
                    guard let self else { return }
                    self.shortcutPressCount += 1
                    self.writeReadiness()
                    await self.toggle()
                }
            },
            onShortcutUp: { [weak self] _, _ in
                Task { @MainActor in
                    guard let self, self.pushToTalk else { return }
                    if self.recordingState == .starting { self.stopWhenStarted = true }
                    else if self.recordingState == .recording { await self.stopAndSubmit() }
                }
            }
        )
    }

    func refreshPermissions() {
        microphoneGranted = AVCaptureDevice.authorizationStatus(for: .audio) == .authorized
        accessibilityGranted = AXIsProcessTrusted()
        if !accessibilityGranted && hotkeyAvailable { shortcutMonitor.stop(); hotkeyAvailable = false }
    }

    // Private diagnostic contains booleans and state only, never a key, transcript, or audio.
    private func writeReadiness() {
        let report: [String: Any] = ["microphone_granted": microphoneGranted,
            "accessibility_granted": accessibilityGranted, "device_key_configured": hasDeviceKey,
            "hotkey_installed": hotkeyAvailable, "shortcut_presses": shortcutPressCount,
            "recording_state": String(describing: recordingState)]
        guard let data = try? JSONSerialization.data(withJSONObject: report, options: [.sortedKeys]),
              data != readinessSnapshot else { return }
        do {
            let fm = FileManager.default
            let directory = fm.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
                .appendingPathComponent("space.hypnos.speech.mac/Validation", isDirectory: true)
            try fm.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
            let output = directory.appendingPathComponent("readiness.json")
            try data.write(to: output, options: .atomic)
            try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: output.path)
            readinessSnapshot = data
        } catch { /* Diagnostic failure must not affect capture. */ }
    }

    func saveSettings(key: String) -> Bool {
        do { _ = try configuration.validatedURL() }
        catch { status = HypnosError.configuration.localizedDescription; return false }
        if !key.isEmpty {
            guard key.rangeOfCharacter(from: .whitespacesAndNewlines) == nil,
                  KeychainService.shared.save(key, forKey: Self.keyAccount, syncable: false,
                                              accessibility: .afterFirstUnlockThisDeviceOnly) else {
                status = "Could not save the device key in Keychain."
                return false
            }
        }
        let defaults = UserDefaults.standard
        defaults.set(configuration.endpoint, forKey: "HypnosEndpoint")
        defaults.set(configuration.model, forKey: "HypnosModel")
        defaults.set(configuration.profileID, forKey: "HypnosProfileID")
        hasDeviceKey = KeychainService.shared.exists(forKey: Self.keyAccount, syncable: false)
        status = hasDeviceKey ? "Settings saved. Ready for a fresh recording." : HypnosError.missingKey.localizedDescription
        writeReadiness()
        return true
    }

    func removeKey() {
        if KeychainService.shared.delete(forKey: Self.keyAccount, syncable: false) {
            hasDeviceKey = false
            status = "Device key removed from this Mac. Revoke it on the server separately if needed."
            writeReadiness()
        }
    }

    func requestMicrophone() async {
        let allowed = await AVCaptureDevice.requestAccess(for: .audio)
        refreshPermissions()
        status = allowed ? (readiness.recordingBlocker ?? "Microphone granted. Ready to record.") : "Enable Hypnos Speech in Privacy & Security → Microphone."
        writeReadiness()
        if !allowed { openPrivacySettings("Microphone") }
    }

    func requestAccessibility() {
        let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
        _ = AXIsProcessTrustedWithOptions(options)
        refreshHotkey()
        if !accessibilityGranted {
            status = "Enable Hypnos Speech in Privacy & Security → Accessibility."
            openPrivacySettings("Accessibility")
        }
    }

    func openPrivacySettings(_ permission: String) {
        guard ["Microphone", "Accessibility"].contains(permission),
              let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_\(permission)") else { return }
        NSWorkspace.shared.open(url)
    }

    func toggle() async {
        if validationTask != nil { await cancel(); return }
        if recordingState == .recording { await stopAndSubmit(); return }
        guard !busy, let workflow else { return }
        refreshPermissions()
        if let blocker = readiness.recordingBlocker { showFeedback(blocker); return }
        let generation = UUID()
        recordingGeneration = generation
        stopWhenStarted = false
        captureTransition = true
        defer { captureTransition = false }
        recordingState = .starting
        status = "Starting microphone…"
        indicatorMessage = nil
        pasteTarget = lastTarget
        showIndicator()
        do {
            let record = try workflow.store.create()
            currentRecording = record
            try await recorder.startRecording(toOutputFile: workflow.store.audioURL(record))
            guard recordingGeneration == generation else {
                await recorder.stopRecording()
                return
            }
            recordingState = .recording
            status = "Recording — stop to send to Hypnos"
            writeReadiness()
            if stopWhenStarted { await stopAndSubmit() }
        } catch {
            if var record = currentRecording {
                record.status = .failed
                record.error = "Microphone could not start. Check the selected audio device."
                try? workflow.store.save(record)
            }
            currentRecording = nil
            recordingState = .idle
            status = "Microphone could not start. Check permission and the selected audio device."
            showFeedback(status)
            refreshHistory()
        }
    }

    func stopAndSubmit() async {
        if validationTask != nil { await cancel(); return }
        guard recordingState == .recording, let record = currentRecording else { return }
        let generation = recordingGeneration
        recordingState = .transcribing
        status = "Stopping recording…"
        await recorder.stopRecording()
        guard recordingGeneration == generation else { return }
        currentRecording = nil
        submit(record)
    }

    func retry(_ record: HypnosRecording) {
        guard !busy, record.status != .completed else { return }
        pasteTarget = lastTarget
        submit(record) // Only this explicit user action resubmits a retained recording.
    }

    private func submit(_ record: HypnosRecording) {
        guard let workflow else { return }
        let key = KeychainService.shared.getString(forKey: Self.keyAccount, syncable: false) ?? ""
        indicatorMessage = nil
        showIndicator()
        workflow.submit(record, configuration: configuration, key: key) { [weak self] text, canceled in
            guard let self, !canceled() else { return false }
            if let target = self.pasteTarget, !target.isTerminated { target.activate(options: []) }
            // No restriction prefix, trailing space, auto-learn, dictionaries, snippets, or cleanup.
            let outcome = await CursorPaster.startPasteAtCursor(text, allowAutoLearn: false, shouldCancel: canceled).value
            return outcome.result.didPostPasteCommand
        }
        refreshWorkflow()
    }

    func cancel() async {
        recordingGeneration = UUID()
        stopWhenStarted = false
        workflow?.cancel() // Invalidate late delivery before awaiting hardware shutdown.
        validationTask?.cancel()
        if let record = currentRecording, let workflow {
            currentRecording = nil
            await recorder.stopRecording()
            var canceled = record
            canceled.status = .canceled
            canceled.error = "Canceled; recording retained."
            do { try workflow.store.save(canceled) }
            catch { status = HypnosError.storage.localizedDescription }
        }
        recordingState = .idle
        status = "Canceled — no late response will be pasted"
        indicator?.orderOut(nil)
        writeReadiness()
        refreshHistory()
    }

    func pasteSaved(_ record: HypnosRecording) {
        guard !busy, let text = record.text else { return }
        Task { @MainActor in
            if let target = lastTarget, !target.isTerminated { target.activate(options: []) }
            let result = await CursorPaster.startPasteAtCursor(text, allowAutoLearn: false).value
            status = result.result.didPostPasteCommand ? "Paste command posted — text preserved exactly" : HypnosError.paste.localizedDescription
        }
    }

    func runNativeValidation(microphoneOnly: Bool = false) async {
        guard !busy else { return }
        refreshPermissions()
        guard microphoneGranted && (microphoneOnly || accessibilityGranted) else {
            showFeedback(microphoneOnly ? "Grant Microphone permission before testing." : "Local validation needs Microphone and Accessibility permissions first.")
            return
        }
        captureTransition = true
        recordingState = .starting
        indicatorMessage = nil
        status = "Starting local microphone test… Nothing will be uploaded."
        showIndicator()
        let onStarted = { [weak self] in
            self?.recordingState = .recording
            self?.status = microphoneOnly ? "Microphone test: recording for five seconds. Nothing will be uploaded." : "Local validation: capturing two seconds of fresh microphone audio. No upload."
        }
        let onStopped = { [weak self] in
            self?.recordingState = .busy
            self?.status = microphoneOnly ? "Checking the captured WAV…" : "Checking native hotkeys and paste… No server upload."
        }
        let task = Task { @MainActor in
            if microphoneOnly {
                return try await nativeValidation.runMicrophone(recorder: recorder, onStarted: onStarted, onStopped: onStopped)
            }
            return try await nativeValidation.run(recorder: recorder, shouldCancel: { [weak self] in
                self?.validationTask?.isCancelled ?? true
            }, onStarted: onStarted, onStopped: onStopped)
        }
        validationTask = task
        do { status = try await task.value }
        catch {
            await recorder.stopRecording()
            status = task.isCancelled ? "Local validation canceled." : "Local validation failed: \(error.localizedDescription)"
        }
        validationTask = nil
        recordingState = .idle
        captureTransition = false
        showFeedback(status)
    }

    private func refreshWorkflow() {
        guard let workflow else { return }
        switch workflow.state {
        case .idle: break
        case .processing: recordingState = .transcribing; status = "Processing on Hypnos…"
        case .success: recordingState = .idle; status = "Text saved; paste command posted"
        case .failed(let message): recordingState = .idle; status = message
        case .canceled: recordingState = .idle; status = "Canceled — recording retained"
        }
        if workflow.state != .idle && workflow.state != .processing { showFeedback(status) }
        refreshHistory()
    }

    private func refreshHistory() {
        do { history = try workflow?.store.list() ?? [] }
        catch { status = HypnosError.storage.localizedDescription }
    }

    private func showIndicator() {
        feedbackTask?.cancel()
        if indicator == nil {
            let panel = MiniRecorderPanel(contentRect: .zero)
            panel.contentView = NSHostingView(rootView: HypnosIndicator(controller: self))
            indicator = panel
        }
        _ = indicator?.show()
    }

    private func showFeedback(_ message: String) {
        status = message
        indicatorMessage = message
        showIndicator()
        writeReadiness()
        feedbackTask = Task { @MainActor [weak self] in
            do { try await Task.sleep(nanoseconds: 6_000_000_000) } catch { return }
            guard let self, !self.busy else { return }
            self.indicator?.orderOut(nil)
        }
    }
}

private struct HypnosIndicator: View {
    @ObservedObject var controller: HypnosController
    var body: some View {
        VStack {
            Spacer()
            HStack(spacing: 14) {
                if let message = controller.indicatorMessage {
                    Image(systemName: "info.circle.fill")
                    Text(message).font(.system(size: 13, weight: .medium))
                        .fixedSize(horizontal: false, vertical: true)
                } else {
                    RecorderStatusDisplay(currentState: controller.recordingState,
                                          audioMeterProvider: controller.recorder.audioMeterSnapshot)
                    if controller.recordingState == .recording {
                        Image(systemName: "record.circle.fill").foregroundStyle(.red)
                            .symbolEffect(.pulse, options: .repeating)
                    }
                    Text(controller.recordingState == .recording ? "Recording" : controller.recordingState == .starting ? "Starting" : "Processing")
                        .font(.system(size: 13, weight: .medium))
                    if controller.recordingState == .recording {
                        Button("Stop") { Task { await controller.stopAndSubmit() } }
                    }
                }
                RecorderCloseButton { Task { await controller.cancel() } }
            }
            .padding(16)
            .foregroundStyle(.white)
            .background(.black.opacity(0.88), in: Capsule())
            .padding(.bottom, 10)
        }
        .frame(width: 540, height: 430)
    }
}

private struct HypnosMenu: View {
    @ObservedObject var controller: HypnosController
    @Environment(\.openWindow) private var openWindow
    var body: some View {
        Text(controller.status)
        Button(controller.recordButtonTitle) {
            Task { await controller.toggle() }
        }.disabled(controller.busy && controller.recordingState != .recording)
        Button("Cancel") { Task { await controller.cancel() } }.disabled(!controller.busy)
        Divider()
        Button("Open Hypnos Speech…") {
            openWindow(id: "hypnos-settings")
            NSApp.activate(ignoringOtherApps: true)
        }
        Divider()
        Button("Quit Hypnos Speech") { NSApp.terminate(nil) }
    }
}

#endif
