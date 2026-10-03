import Foundation

/// Hypnos is a final-output provider. Only this configuration and recorded audio cross the wire.
struct HypnosConfiguration: Equatable, Sendable {
    static let defaultEndpoint = "https://speech.tusharbhardwaj.space/v1/audio/transcriptions"
    var endpoint = Self.defaultEndpoint
    var model = "whisper-large-v3-turbo"
    var profileID = ""

    func validatedURL() throws -> URL {
        guard let url = URL(string: endpoint), url.scheme == "https", url.host != nil,
              url.user == nil, url.password == nil, url.query == nil, url.fragment == nil,
              !model.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              model.rangeOfCharacter(from: .newlines) == nil,
              profileID.rangeOfCharacter(from: .newlines) == nil else { throw HypnosError.configuration }
        return url
    }
}

enum HypnosError: LocalizedError, Equatable {
    case configuration, missingKey, emptyAudio, audioTooLarge, authentication, timeout, unavailable
    case http(Int), malformedResponse, emptyResponse, storage, paste

    var errorDescription: String? {
        switch self {
        case .configuration: return "Set a valid HTTPS transcription endpoint and model in Settings."
        case .missingKey: return "Enter a dedicated speech-only device key in Settings."
        case .emptyAudio: return "The recording contains no audio. It has been retained."
        case .audioTooLarge: return "The recording exceeds the 25 MB upload limit. It has been retained."
        case .authentication: return "The device key was rejected. Replace it in Settings, then explicitly retry."
        case .timeout: return "Hypnos timed out. Processing may have occurred; retry only when you choose."
        case .unavailable: return "Hypnos is unavailable. Check your network and Tailscale connection, then explicitly retry."
        case .http(let code): return "Hypnos returned HTTP \(code). The recording is retained for explicit retry."
        case .malformedResponse: return "Hypnos returned an invalid response. The recording is retained."
        case .emptyResponse: return "Hypnos returned empty text. The recording is retained."
        case .storage: return "Private recording storage could not be saved. Check disk space and permissions."
        case .paste: return "Text was saved, but paste could not be posted. Grant Accessibility permission and use Paste saved text."
        }
    }
}

protocol HypnosTranscribing {
    func transcribe(audioURL: URL, configuration: HypnosConfiguration, key: String) async throws -> String
}

/// Refuse redirects rather than forward an audio upload or bearer key to another origin.
private final class HypnosSessionDelegate: NSObject, URLSessionTaskDelegate {
    func urlSession(_ session: URLSession, task: URLSessionTask,
                    willPerformHTTPRedirection response: HTTPURLResponse, newRequest request: URLRequest,
                    completionHandler: @escaping (URLRequest?) -> Void) {
        completionHandler(nil)
    }
}

final class HypnosClient: HypnosTranscribing {
    private let session: URLSession

    init(session: URLSession? = nil) {
        if let session {
            self.session = session
        } else {
            let config = URLSessionConfiguration.ephemeral
            config.urlCache = nil
            config.httpCookieStorage = nil
            config.requestCachePolicy = .reloadIgnoringLocalCacheData
            config.timeoutIntervalForRequest = 90
            config.timeoutIntervalForResource = 120
            config.waitsForConnectivity = false
            self.session = URLSession(configuration: config, delegate: HypnosSessionDelegate(), delegateQueue: nil)
        }
    }

    deinit { session.invalidateAndCancel() }

    static func request(audio: Data, configuration: HypnosConfiguration, key: String,
                        boundary: String = "Hypnos-\(UUID().uuidString)") throws -> (URLRequest, Data) {
        let url = try configuration.validatedURL()
        guard !key.isEmpty, key.rangeOfCharacter(from: .whitespacesAndNewlines) == nil else { throw HypnosError.missingKey }
        guard audio.count > 44 else { throw HypnosError.emptyAudio }
        guard audio.count <= 25 * 1024 * 1024 else { throw HypnosError.audioTooLarge }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.timeoutInterval = 90
        request.setValue("Bearer \(key)", forHTTPHeaderField: "Authorization")
        request.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        var body = Data()
        func append(_ value: String) { body.append(Data(value.utf8)) }
        func field(_ name: String, _ value: String) {
            append("--\(boundary)\r\nContent-Disposition: form-data; name=\"\(name)\"\r\n\r\n\(value)\r\n")
        }
        append("--\(boundary)\r\nContent-Disposition: form-data; name=\"file\"; filename=\"recording.wav\"\r\nContent-Type: audio/wav\r\n\r\n")
        body.append(audio)
        append("\r\n")
        field("model", configuration.model)
        field("response_format", "json")
        if !configuration.profileID.isEmpty { field("profile_id", configuration.profileID) }
        append("--\(boundary)--\r\n")
        return (request, body)
    }

    static func decode(_ data: Data, status: Int) throws -> String {
        if status == 401 || status == 403 { throw HypnosError.authentication }
        guard (200...299).contains(status) else { throw HypnosError.http(status) }
        struct Response: Decodable { let text: String }
        guard let response = try? JSONDecoder().decode(Response.self, from: data) else {
            throw HypnosError.malformedResponse
        }
        guard !response.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw HypnosError.emptyResponse
        }
        // Validation above does not mutate text: preserve whitespace, Markdown, code, and Unicode.
        return response.text
    }

    func transcribe(audioURL: URL, configuration: HypnosConfiguration, key: String) async throws -> String {
        try Task.checkCancellation()
        let audio = try Data(contentsOf: audioURL)
        let (request, body) = try Self.request(audio: audio, configuration: configuration, key: key)
        do {
            // Exactly one application-level upload. No automatic retry or batch fallback.
            let (data, response) = try await session.upload(for: request, from: body)
            try Task.checkCancellation()
            guard let response = response as? HTTPURLResponse else { throw HypnosError.malformedResponse }
            return try Self.decode(data, status: response.statusCode)
        } catch let error as URLError {
            if error.code == .cancelled || Task.isCancelled { throw CancellationError() }
            throw error.code == .timedOut ? HypnosError.timeout : HypnosError.unavailable
        }
    }
}
