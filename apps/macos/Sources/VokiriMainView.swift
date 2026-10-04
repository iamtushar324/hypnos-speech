#if VOKIRI
import AppKit
import SwiftUI

/// VoiceInk's actual sidebar, window geometry, hero, theme and controls, with Vokiri-owned data.
struct VokiriMainView: View {
    @ObservedObject var controller: VokiriController
    @State private var selectedView: ViewType = .dashboard

    var body: some View {
        HStack(spacing: 0) {
            AppSidebar(selectedView: $selectedView)
            detail
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background {
                    ZStack {
                        VisualEffectView(material: .sidebar, blendingMode: .behindWindow)
                        AppTheme.Surface.window.opacity(0.50)
                    }.ignoresSafeArea(.container, edges: .top)
                }
        }
        .frame(width: AppWindowLayout.width)
        .frame(minHeight: AppWindowLayout.minimumHeight)
    }

    @ViewBuilder private var detail: some View {
        switch selectedView {
        case .dashboard:
            dashboard
        case .transcribeAudio:
            VokiriPage(title: "Transcribe") {
                CompactHeroSection(icon: "waveform.path", title: "Record, then transcribe",
                    description: "Capture fresh audio on this Mac. Vokiri returns the final text after you stop.")
                VokiriCaptureCard(controller: controller)
                setupCard
            }
        case .history:
            VokiriHistoryView(controller: controller)
        case .models:
            VokiriConnectionView(controller: controller)
        case .audio, .settings:
            VokiriPreferencesView(controller: controller, audioOnly: selectedView == .audio)
        case .modes:
            serverPage(title: "Modes", icon: "sparkles.square.fill.on.square",
                description: "Vokiri handles cleanup, translation, and profiles. This Mac uses the server’s final output.")
        case .dictionary:
            serverPage(title: "Dictionary", icon: "text.book.closed.fill",
                description: "Manage names, dictionary entries, and snippets on Vokiri. They apply on the server before final text reaches this Mac.")
        case .license:
            VokiriPage(title: "Vokiri") {
                CompactHeroSection(icon: "waveform", title: "Vokiri",
                    description: "A native VoiceInk fork connected to your Vokiri speech gateway.")
                VStack(alignment: .leading, spacing: 12) {
                    Text("Based on VoiceInk v2.21 by Pax. Licensed under GNU GPLv3.")
                    Text("Native recording, hotkeys, Keychain, recording indicator, and paste come from VoiceInk. Vokiri owns transcription and cleanup.")
                    Link("Source and upstream history", destination: URL(string: "https://github.com/iamtushar324/vokiri")!)
                    Text("Updates are installed by building this fork. Stock VoiceInk binary updates are disabled.")
                        .font(.caption).foregroundStyle(.secondary)
                }.vokiriCard()
            }
        }
    }

    private var dashboard: some View {
        VokiriPage(title: "Dashboard") {
            DashboardHeroCard(isLocked: false, headline: .vokiri,
                subtext: "Use your shortcut to record. Stop to send to Vokiri and paste the final text.",
                actionTitle: "\(controller.recordButtonTitle)",
                actionIcon: controller.recordingState == .recording ? "stop.fill" : "mic.fill",
                canViewInsights: !controller.busy || controller.recordingState == .recording,
                actionHelp: "Record with the configured microphone and Vokiri gateway",
                actionAccessibilityLabel: "Start or stop recording", reviewCorrectionCount: nil,
                onViewInsights: { Task { await controller.toggle() } }, onReviewCorrections: {})
            VokiriCaptureCard(controller: controller)
            setupCard
            HStack(spacing: DashboardLayout.columnSpacing) {
                metric(title: "Transcriptions", value: "\(controller.history.filter { $0.status == .completed }.count)", icon: "doc.text.fill")
                metric(title: "Kept for retry", value: "\(controller.history.filter { $0.status == .failed || $0.status == .canceled }.count)", icon: "arrow.clockwise")
            }
            HStack {
                Text("Vokiri final-output mode").font(.caption).foregroundStyle(.secondary)
                Spacer()
                AppActionButton("View History") { selectedView = .history }
                AppActionButton("Settings") { selectedView = .settings }
            }
        }
    }

    private var setupCard: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Text("Ready to dictate").font(.headline)
                Spacer()
                Text(controller.readiness.recordingBlocker == nil ? "Ready" : "Setup needed")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(controller.readiness.recordingBlocker == nil ? Color.green : Color.orange)
            }
            readinessRow("Microphone", ready: controller.microphoneGranted,
                detail: controller.microphoneGranted ? "Permission granted" : "Allow voice capture") {
                Task { await controller.requestMicrophone() }
            }
            readinessRow("Accessibility", ready: controller.accessibilityGranted,
                detail: controller.accessibilityGranted ? "Permission granted" : "Allow global hotkeys and paste") {
                controller.requestAccessibility()
            }
            readinessRow("Mac device key", ready: controller.hasDeviceKey,
                detail: controller.hasDeviceKey ? "Saved in Keychain" : "Enter your speech-only key securely") {
                selectedView = .models
            }
            readinessRow("Global hotkey", ready: controller.hotkeyAvailable,
                detail: controller.readiness.hotkeyStatus) { controller.refreshHotkey() }
        }.vokiriCard()
    }

    private func readinessRow(_ title: String, ready: Bool, detail: String, action: @escaping () -> Void) -> some View {
        HStack(spacing: 10) {
            Image(systemName: ready ? "checkmark.circle.fill" : "exclamationmark.circle.fill")
                .foregroundStyle(ready ? Color.green : Color.orange)
            Text(title).font(.system(size: 13, weight: .medium)).frame(width: 110, alignment: .leading)
            Text(detail).font(.caption).foregroundStyle(.secondary)
            Spacer()
            if !ready { AppActionButton("Set Up", action: action) }
        }
    }

    private func metric(title: String, value: String, icon: String) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 8) {
                Text(title).font(.system(size: 13, weight: .medium)).foregroundStyle(.secondary)
                Text(value).font(.system(size: 30, weight: .bold, design: .rounded))
            }
            Spacer()
            Image(systemName: icon).font(.system(size: 24)).foregroundStyle(.secondary)
        }.vokiriCard().frame(maxWidth: .infinity)
    }

    private func serverPage(title: LocalizedStringKey, icon: String, description: LocalizedStringKey) -> some View {
        VokiriPage(title: title) {
            CompactHeroSection(icon: icon, title: "Managed on Vokiri", description: description)
            HStack {
                Spacer()
                Link("Open Vokiri settings", destination: URL(string: "https://speech.tusharbhardwaj.space/")!)
                Spacer()
            }.vokiriCard()
        }
    }
}

private struct VokiriPage<Content: View>: View {
    let title: LocalizedStringKey
    @ViewBuilder let content: () -> Content
    var body: some View {
        VStack(spacing: 0) {
            AppScreenHeader(title: title)
            ScrollView {
                VStack(alignment: .leading, spacing: DashboardLayout.sectionSpacing, content: content)
                    .padding(.horizontal, DashboardLayout.pageHorizontalPadding)
                    .padding(.top, 12).padding(.bottom, 28)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }
}

private extension View {
    func vokiriCard() -> some View {
        padding(20).frame(maxWidth: .infinity, alignment: .leading)
            .background(AppCardBackground(cornerRadius: DashboardLayout.cardCornerRadius))
    }
}

private struct VokiriCaptureCard: View {
    @ObservedObject var controller: VokiriController
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(spacing: 14) {
                RecorderStatusDisplay(currentState: controller.recordingState,
                    audioMeterProvider: controller.recorder.audioMeterSnapshot)
                    .frame(width: 92, height: 38).padding(.horizontal, 8)
                    .background(.black.opacity(0.88), in: Capsule())
                if controller.recordingState == .recording {
                    Image(systemName: "record.circle.fill").foregroundStyle(.red)
                        .symbolEffect(.pulse, options: .repeating)
                }
                VStack(alignment: .leading, spacing: 4) {
                    Text(controller.status).font(.system(size: 14, weight: .semibold)).textSelection(.enabled)
                    Text(controller.recordingState == .recording ? "Microphone active · the bars respond to your voice" :
                        controller.recordingState == .transcribing ? "Waiting for the final server response" :
                        controller.recordingState == .busy ? "Local validation in progress · no server upload" :
                        "\(controller.pushToTalk ? "Hold to record" : "Press to start / stop") · \(controller.readiness.hotkeyStatus)")
                        .font(.caption).foregroundStyle(.secondary)
                }
            }
            HStack {
                Text("Shortcut").font(.caption).foregroundStyle(.secondary)
                ShortcutRecorder(action: .primaryRecording, onShortcutChanged: controller.refreshHotkey)
                    .disabled(controller.busy)
                Spacer()
                AppActionButton(LocalizedStringKey(controller.recordButtonTitle), kind: .primary) {
                    Task { await controller.toggle() }
                }.disabled(controller.busy && controller.recordingState != .recording)
                if controller.busy { AppActionButton("Cancel") { Task { await controller.cancel() } } }
            }
            if !controller.busy {
                HStack {
                    Button("Test Microphone · no upload") { Task { await controller.runNativeValidation(microphoneOnly: true) } }
                    Text("Five-second local check").font(.caption).foregroundStyle(.secondary)
                }
            }
        }.vokiriCard()
    }
}

private struct VokiriConnectionView: View {
    @ObservedObject var controller: VokiriController
    @State private var key = ""
    var body: some View {
        VStack(spacing: 0) {
            AppScreenHeader(title: "AI Models")
            Form {
                Section("Vokiri Gateway") {
                    LabeledContent("Provider") { Text("Vokiri · final output") }
                    TextField("Endpoint", text: $controller.configuration.endpoint)
                    TextField("Model", text: $controller.configuration.model)
                    TextField("Profile ID (optional)", text: $controller.configuration.profileID)
                }
                Section {
                    SecureField(controller.hasDeviceKey ? "Replace key (blank keeps saved key)" : "Dedicated Mac speech-only key", text: $key)
                        .privacySensitive().accessibilityIdentifier("vokiri-device-key")
                    LabeledContent("Device key") {
                        Label(controller.hasDeviceKey ? "Saved in Keychain" : "Not configured",
                            systemImage: controller.hasDeviceKey ? "checkmark.circle.fill" : "exclamationmark.circle")
                    }
                    HStack {
                        AppActionButton("Save Settings", kind: .primary) {
                            if controller.saveSettings(key: key) { key = "" }
                        }
                        Link("Create a Mac key", destination: URL(string: "https://speech.tusharbhardwaj.space/#keys")!)
                        Spacer()
                        if controller.hasDeviceKey {
                            AppActionButton("Remove Key", kind: .destructive) { controller.removeKey() }
                        }
                    }
                } header: { Text("Authentication") } footer: {
                    Text("Enter the dedicated key here, never in chat. It stays in this app’s macOS Keychain namespace.")
                }
                Section("Batch Transcription") {
                    Text("Keep Tailscale connected. Audio is sent after recording stops. Vokiri handles transcription, dictionary, snippets, and cleanup.")
                    Text("The final server text is saved and pasted exactly, including Markdown and paragraph breaks.")
                }
                Section { Text(controller.status).textSelection(.enabled) }
            }.formStyle(.grouped).disabled(controller.busy)
        }
    }
}

private struct VokiriPreferencesView: View {
    @ObservedObject var controller: VokiriController
    let audioOnly: Bool
    @AppStorage("restoreClipboardAfterPaste") private var restoreClipboard = true
    var body: some View {
        VStack(spacing: 0) {
            AppScreenHeader(title: audioOnly ? "Audio" : "Settings")
            Form {
                Section("Shortcuts") {
                    LabeledContent("Primary Shortcut") {
                        ShortcutRecorder(action: .primaryRecording, onShortcutChanged: controller.refreshHotkey)
                            .disabled(controller.busy)
                    }
                    Toggle("Push to Talk", isOn: $controller.pushToTalk).disabled(controller.busy)
                    LabeledContent("Hotkey Status") { Text(controller.readiness.hotkeyStatus) }
                    Button("Refresh Hotkey") { controller.refreshHotkey() }
                }
                Section {
                    LabeledContent("Microphone", value: controller.microphoneGranted ? "Granted" : "Permission needed")
                    Button("Grant Microphone") { Task { await controller.requestMicrophone() } }
                    LabeledContent("Accessibility", value: controller.accessibilityGranted ? "Granted" : "Permission needed")
                    Button("Grant Accessibility") { controller.requestAccessibility() }
                    Button("Show This App in Finder") { controller.revealApp() }
                } header: { Text("macOS Permissions") } footer: {
                    Text("Microphone captures your voice. Accessibility enables global hotkeys and Command-V paste. If Accessibility is already on but this app remains blocked, remove its old entry with −, add this installed app with +, enable it, and relaunch. A changed signing identity can leave an older grant visible.")
                }
                Section("Audio Input") {
                    Text("Uses the macOS default input device. Change it in System Settings → Sound → Input.")
                    Text("Recording format: mono WAV · 16 kHz PCM16").foregroundStyle(.secondary)
                }
                if !audioOnly {
                    Section("Pasting") {
                        Toggle("Keep Clipboard Content", isOn: $restoreClipboard)
                        Text("Paste uses VoiceInk’s Command-V machinery. Clipboard contents stay on this Mac.").font(.caption).foregroundStyle(.secondary)
                    }
                }
                Section {
                    Button("Test Microphone — No Upload") { Task { await controller.runNativeValidation(microphoneOnly: true) } }.disabled(controller.busy)
                    Button("Run Local Validation") { Task { await controller.runNativeValidation() } }.disabled(controller.busy)
                    Text("Records two seconds of fresh microphone audio, tests native hotkey events, and checks exact paste in a disposable document. No server upload.").font(.caption).foregroundStyle(.secondary)
                    Text(controller.status).textSelection(.enabled)
                } header: { Text("Diagnostics") }
            }.formStyle(.grouped)
        }
    }
}

private struct VokiriHistoryView: View {
    @ObservedObject var controller: VokiriController
    var body: some View {
        VokiriPage(title: "History") {
            Text("Failed and canceled audio stays on this Mac for explicit retry. Paste saved text without uploading again.")
                .font(.callout).foregroundStyle(.secondary)
            if controller.history.isEmpty {
                CompactHeroSection(icon: "doc.text", title: "No recordings yet",
                    description: "Use your shortcut or Start Recording to make your first transcription.")
            }
            ForEach(controller.history) { record in
                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        Image(systemName: "doc.text.fill").foregroundStyle(.secondary)
                        Text(record.createdAt, format: .dateTime.month().day().hour().minute())
                        Spacer()
                        Text(record.status.rawValue.capitalized).font(.caption).foregroundStyle(.secondary)
                    }
                    if let error = record.error { Text(error).font(.caption).foregroundStyle(.secondary) }
                    if let text = record.text {
                        Text(verbatim: text).textSelection(.enabled).font(.system(.body, design: .monospaced))
                    }
                    HStack {
                        if record.status == .failed || record.status == .canceled {
                            AppActionButton("Retry") { controller.retry(record) }.disabled(controller.busy)
                        }
                        if record.text != nil {
                            AppActionButton("Paste Saved Text") { controller.pasteSaved(record) }.disabled(controller.busy)
                        }
                    }
                }.vokiriCard()
            }
            AppActionButton("Show Private Recordings in Finder") {
                let root = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
                    .appendingPathComponent("space.hypnos.speech.mac/Recordings")
                NSWorkspace.shared.open(root)
            }
        }
    }
}
#endif
