import Foundation

/// The Vokiri final-output mode deliberately never enters VoiceInk's formatting/enhancement pipeline.
@MainActor
final class VokiriWorkflow {
    enum State: Equatable { case idle, processing, success, failed(String), canceled }
    private(set) var state: State = .idle { didSet { onChange?() } }
    var onChange: (() -> Void)?
    let store: VokiriStore
    private let client: VokiriTranscribing
    private var task: Task<Void, Never>?
    private var generation = UUID()
    var isBusy: Bool { task != nil }

    init(store: VokiriStore, client: VokiriTranscribing = VokiriClient()) {
        self.store = store
        self.client = client
    }

    func submit(_ record: VokiriRecording, configuration: VokiriConfiguration, key: String,
                paste: @escaping @MainActor (String, @escaping @MainActor () -> Bool) async -> Bool) {
        guard task == nil else { return }
        let current = UUID()
        generation = current
        state = .processing
        task = Task { [self] in
            var updated = record
            do {
                updated.status = .processing
                updated.error = nil
                try store.save(updated)
                let text = try await client.transcribe(audioURL: store.audioURL(record), configuration: configuration, key: key)
                guard generation == current, !Task.isCancelled else { throw CancellationError() }
                // Persist the exact result before delivery, so paste failures never require another upload.
                updated.text = text
                updated.status = .completed
                try store.save(updated)
                let didPaste = await paste(text, { [weak self] in self?.generation != current })
                guard generation == current, !Task.isCancelled else { throw CancellationError() }
                state = didPaste ? .success : .failed(VokiriError.paste.localizedDescription)
            } catch {
                if generation != current || Task.isCancelled || error is CancellationError {
                    // A completed result stays available when cancellation happened during paste delay.
                    if updated.status != .completed { updated.status = .canceled }
                    updated.error = "Canceled. Processing may have occurred; no automatic retry."
                } else {
                    updated.status = .failed
                    updated.error = (error as? VokiriError)?.localizedDescription ?? "Transcription failed. The recording is retained."
                    state = .failed(updated.error!)
                }
                do { try store.save(updated) }
                catch { if generation == current { state = .failed(VokiriError.storage.localizedDescription) } }
            }
            if generation == current { task = nil }
            onChange?()
        }
    }

    /// Invalidate delivery synchronously, even if a server or mock ignores cooperative cancellation.
    func cancel() {
        generation = UUID()
        task?.cancel()
        // Keep the task until it settles to prevent overlapping processing of the same audio.
        state = .canceled
        let canceledTask = task
        Task { [weak self] in
            await canceledTask?.value
            self?.task = nil
            self?.onChange?()
        }
    }

    func waitUntilSettled() async { await task?.value }
}
