#if HYPNOS_SPEECH
import AppKit
import AVFoundation
import SwiftUI

@main
struct HypnosSpeechApp: App {
    @StateObject private var controller = HypnosController()

    var body: some Scene {
        Window("Hypnos Speech", id: "hypnos-settings") {
            HypnosSettingsView(controller: controller)
                .frame(minWidth: 640, minHeight: 560)
        }
        .defaultSize(width: 700, height: 650)
        MenuBarExtra("Hypnos Speech", systemImage: controller.recordingState == .recording ? "mic.fill" : "waveform") {
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
    @Published var pushToTalk: Bool {
        didSet { UserDefaults.standard.set(pushToTalk, forKey: "HypnosPushToTalk") }
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
    var busy: Bool { captureTransition || recordingState != .idle || workflow?.isBusy == true }

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
        permissionTimer = Timer.scheduledTimer(withTimeInterval: 2, repeats: true) { [weak self] _ in
            Task { @MainActor in
                guard let self else { return }
                if AXIsProcessTrusted() && !self.hotkeyAvailable { self.refreshHotkey() }
            }
        }
    }

    func refreshHotkey() {
        guard AXIsProcessTrusted() else { hotkeyAvailable = false; return }
        guard let shortcut = ShortcutStore.shortcut(for: .primaryRecording) else {
            shortcutMonitor.stop()
            hotkeyAvailable = false
            return
        }
        hotkeyAvailable = shortcutMonitor.start(
            shortcuts: [.primaryRecording: shortcut],
            standaloneModifierActions: [.primaryRecording],
            onShortcutDown: { [weak self] _, _ in
                Task { @MainActor in await self?.toggle() }
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
        return true
    }

    func removeKey() {
        if KeychainService.shared.delete(forKey: Self.keyAccount, syncable: false) {
            hasDeviceKey = false
            status = "Device key removed from this Mac. Revoke it on the server separately if needed."
        }
    }

    func requestMicrophone() async {
        let allowed = await AVCaptureDevice.requestAccess(for: .audio)
        status = allowed ? "Microphone granted. Grant Accessibility for hotkeys and paste." : "Enable Hypnos Speech in Privacy & Security → Microphone."
    }

    func requestAccessibility() {
        let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
        _ = AXIsProcessTrustedWithOptions(options)
        refreshHotkey()
    }

    func toggle() async {
        if recordingState == .recording { await stopAndSubmit(); return }
        guard !busy, let workflow else { return }
        guard hasDeviceKey else { status = HypnosError.missingKey.localizedDescription; return }
        guard AVCaptureDevice.authorizationStatus(for: .audio) == .authorized else {
            status = "Grant Microphone permission in Settings before recording."
            return
        }
        guard AXIsProcessTrusted() else {
            status = "Grant Accessibility permission for global hotkeys and paste."
            return
        }
        let generation = UUID()
        recordingGeneration = generation
        stopWhenStarted = false
        captureTransition = true
        defer { captureTransition = false }
        recordingState = .starting
        status = "Starting microphone…"
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
            indicator?.orderOut(nil)
            refreshHistory()
        }
    }

    func stopAndSubmit() async {
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

    func runNativeValidation() async {
        guard !busy else { return }
        captureTransition = true
        recordingState = .recording
        status = "Local validation: capturing two seconds of fresh microphone audio. No upload."
        showIndicator()
        do { status = try await nativeValidation.run(recorder: recorder) }
        catch { await recorder.stopRecording(); status = "Local validation failed; check permissions and audio device." }
        recordingState = .idle
        captureTransition = false
        indicator?.orderOut(nil)
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
        if !workflow.isBusy && workflow.state != .processing { indicator?.orderOut(nil) }
        refreshHistory()
    }

    private func refreshHistory() {
        do { history = try workflow?.store.list() ?? [] }
        catch { status = HypnosError.storage.localizedDescription }
    }

    private func showIndicator() {
        if indicator == nil {
            let panel = MiniRecorderPanel(contentRect: .zero)
            panel.contentView = NSHostingView(rootView: HypnosIndicator(controller: self))
            indicator = panel
        }
        _ = indicator?.show()
    }
}

private struct HypnosIndicator: View {
    @ObservedObject var controller: HypnosController
    var body: some View {
        VStack {
            Spacer()
            HStack(spacing: 14) {
                RecorderStatusDisplay(currentState: controller.recordingState,
                                      audioMeterProvider: controller.recorder.audioMeterSnapshot)
                Text(controller.recordingState == .recording ? "Recording" : "Processing")
                    .font(.system(size: 13, weight: .medium))
                if controller.recordingState == .recording {
                    Button("Stop") { Task { await controller.stopAndSubmit() } }
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
        Button(controller.recordingState == .recording ? "Stop and transcribe" : "Start recording") {
            Task { await controller.toggle() }
        }.disabled(controller.busy && controller.recordingState != .recording)
        Button("Cancel") { Task { await controller.cancel() } }.disabled(!controller.busy)
        Divider()
        Button("Settings and recordings…") {
            openWindow(id: "hypnos-settings")
            NSApp.activate(ignoringOtherApps: true)
        }
        Divider()
        Button("Quit Hypnos Speech") { NSApp.terminate(nil) }
    }
}

private struct HypnosSettingsView: View {
    @ObservedObject var controller: HypnosController
    @State private var key = ""
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                Text("Hypnos Speech").font(.largeTitle.bold())
                Text("Hypnos final-output mode").font(.headline)
                Text("Record here. Hypnos handles transcription, dictionary, snippets, and cleanup. Its final text is saved and pasted exactly. Results arrive after recording stops.")
                Text(controller.status).foregroundStyle(.secondary).textSelection(.enabled)
                GroupBox("Connection — keep Tailscale connected") {
                    VStack(alignment: .leading, spacing: 10) {
                        TextField("HTTPS transcription endpoint", text: $controller.configuration.endpoint)
                        TextField("Model", text: $controller.configuration.model)
                        TextField("Profile ID (optional)", text: $controller.configuration.profileID)
                        SecureField(controller.hasDeviceKey ? "Replace device key (leave blank to keep)" : "Dedicated speech-only device key", text: $key)
                            .privacySensitive()
                            .accessibilityIdentifier("hypnos-device-key")
                        HStack {
                            Button("Save settings") { if controller.saveSettings(key: key) { key = "" } }
                            Link("Create a Mac device key", destination: URL(string: "https://speech.tusharbhardwaj.space/#keys")!)
                            if controller.hasDeviceKey { Button("Remove saved key") { controller.removeKey() } }
                        }
                        Text("The key stays in this app’s macOS Keychain namespace. Only audio and the fields above are sent. Server/provider retention follows your server settings.").font(.caption).foregroundStyle(.secondary)
                    }.padding(8)
                }.disabled(controller.busy)
                GroupBox("Recording and permissions") {
                    VStack(alignment: .leading, spacing: 10) {
                        HStack {
                            Text("Global hotkey")
                            ShortcutRecorder(action: .primaryRecording, onShortcutChanged: controller.refreshHotkey)
                            Toggle("Push to talk", isOn: $controller.pushToTalk)
                        }
                        Text(controller.hotkeyAvailable ? "Hotkey active. Toggle: press to start/stop. Push to talk: hold to record." : "Hotkey needs Accessibility permission. Default: Control–Option–Space.")
                            .font(.caption)
                        HStack {
                            Button("Grant Microphone") { Task { await controller.requestMicrophone() } }
                            Button("Grant Accessibility") { controller.requestAccessibility() }
                            Button("Refresh hotkey") { controller.refreshHotkey() }
                            Button("Run local validation") { Task { await controller.runNativeValidation() } }.disabled(controller.busy)
                        }
                        Text("Microphone captures your voice. Accessibility enables global hotkeys and Command-V paste. Allow Hypnos Speech in System Settings → Privacy & Security. Screen Recording is not required.").font(.caption).foregroundStyle(.secondary)
                        HStack {
                            Button(controller.recordingState == .recording ? "Stop and transcribe" : "Start recording") { Task { await controller.toggle() } }
                                .disabled(controller.busy && controller.recordingState != .recording)
                            Button("Cancel") { Task { await controller.cancel() } }.disabled(!controller.busy)
                        }
                    }.padding(8)
                }
                GroupBox("Retained recordings") {
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Stored privately on this Mac. Failed or interrupted recordings stay here until you remove them in Finder. Retry makes a new server request and may incur processing again. Saved text can be pasted without uploading again.").font(.caption).foregroundStyle(.secondary)
                        if controller.history.isEmpty { Text("No recordings yet.").foregroundStyle(.secondary) }
                        ForEach(controller.history) { record in
                            VStack(alignment: .leading, spacing: 6) {
                                HStack {
                                    Text(record.createdAt, style: .date)
                                    Text(record.createdAt, style: .time)
                                    Text(record.status.rawValue).foregroundStyle(.secondary)
                                    Spacer()
                                    if record.status == .failed || record.status == .canceled {
                                        Button("Retry") { controller.retry(record) }.disabled(controller.busy)
                                    }
                                    if record.text != nil {
                                        Button("Paste saved text") { controller.pasteSaved(record) }.disabled(controller.busy)
                                    }
                                }
                                if let error = record.error { Text(error).font(.caption).foregroundStyle(.secondary) }
                                if let text = record.text { Text(verbatim: text).textSelection(.enabled).font(.system(.body, design: .monospaced)) }
                                Divider()
                            }
                        }
                        Button("Show private recordings in Finder") {
                            let root = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("space.hypnos.speech.mac/Recordings")
                            NSWorkspace.shared.open(root)
                        }
                    }.padding(8)
                }
                Text("Based on VoiceInk v2.21 by Pax. GPLv3. Updates: fetch upstream source and rebuild; binary updates are disabled.").font(.caption).foregroundStyle(.secondary)
            }.padding(24)
        }
    }
}
#endif
