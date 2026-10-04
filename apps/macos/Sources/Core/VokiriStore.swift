import Foundation

struct VokiriRecording: Codable, Identifiable, Equatable {
    enum Status: String, Codable { case recording, processing, completed, failed, canceled }
    let id: UUID
    let createdAt: Date
    var status: Status
    var text: String?
    var error: String?
}

/// Separate, private app storage. Never scans or imports recordings from other applications.
final class VokiriStore {
    let root: URL
    private let fm = FileManager.default

    init(root: URL = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        .appendingPathComponent("space.hypnos.speech.mac/Recordings", isDirectory: true)) throws {
        self.root = root
        try fm.createDirectory(at: root, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
        try fm.setAttributes([.posixPermissions: 0o700], ofItemAtPath: root.deletingLastPathComponent().path)
        try fm.setAttributes([.posixPermissions: 0o700], ofItemAtPath: root.path)
    }

    func audioURL(_ record: VokiriRecording) -> URL { root.appendingPathComponent("\(record.id).wav") }
    private func metadataURL(_ record: VokiriRecording) -> URL { root.appendingPathComponent("\(record.id).json") }

    func create() throws -> VokiriRecording {
        let record = VokiriRecording(id: UUID(), createdAt: Date(), status: .recording)
        guard fm.createFile(atPath: audioURL(record).path, contents: Data(), attributes: [.posixPermissions: 0o600]) else {
            throw VokiriError.storage
        }
        try save(record)
        return record
    }

    func save(_ record: VokiriRecording) throws {
        try JSONEncoder().encode(record).write(to: metadataURL(record), options: .atomic)
        try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: metadataURL(record).path)
        if fm.fileExists(atPath: audioURL(record).path) {
            try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: audioURL(record).path)
        }
    }

    func list() throws -> [VokiriRecording] {
        try fm.contentsOfDirectory(at: root, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "json" }
            .map { try JSONDecoder().decode(VokiriRecording.self, from: Data(contentsOf: $0)) }
            .sorted { $0.createdAt > $1.createdAt }
    }

    /// Interrupted sessions are retained and never resubmitted at launch.
    func recoverInterrupted() throws {
        for var record in try list() where record.status == .recording || record.status == .processing {
            record.status = .failed
            record.error = "Interrupted session. Audio retained; retry only when you choose."
            try save(record)
        }
    }
}
