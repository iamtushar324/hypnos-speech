import XCTest
@testable import VokiriCore

private final class MockURLProtocol: URLProtocol {
    static var handler: ((URLRequest) throws -> (Int, Data))?
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        do {
            let (status, data) = try Self.handler!(request)
            client?.urlProtocol(self, didReceive: HTTPURLResponse(url: request.url!, statusCode: status, httpVersion: nil, headerFields: nil)!, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch { client?.urlProtocol(self, didFailWithError: error) }
    }
    override func stopLoading() {}
}

private final class ControlledClient: VokiriTranscribing {
    var calls = 0
    var result: Result<String, Error> = .success("text")
    var continuation: CheckedContinuation<String, Error>?
    var suspend = false
    func transcribe(audioURL: URL, configuration: VokiriConfiguration, key: String) async throws -> String {
        calls += 1
        if suspend { return try await withCheckedThrowingContinuation { continuation = $0 } }
        return try result.get()
    }
}

final class VokiriCoreTests: XCTestCase {
    private let exact = " \n# Vokiri [keep this]\n\n- Tushar — नमस्ते 👋\n\n```swift\nlet café = \"[yes]\"\n  print(café)\n```\n\nTrailing spaces  \n"

    func testSetupBlockersAndHotkeyReadiness() throws {
        var ready = VokiriReadiness(deviceKeyConfigured: true, microphoneGranted: true,
                                    accessibilityGranted: true, hotkeyInstalled: true)
        XCTAssertNil(ready.recordingBlocker)
        ready.microphoneGranted = false
        XCTAssertTrue(try XCTUnwrap(ready.recordingBlocker).contains("Microphone"))
        ready.microphoneGranted = true; ready.accessibilityGranted = false
        XCTAssertTrue(try XCTUnwrap(ready.recordingBlocker).contains("Accessibility"))
        ready.accessibilityGranted = true; ready.deviceKeyConfigured = false
        XCTAssertTrue(try XCTUnwrap(ready.recordingBlocker).contains("Device key"))
        ready.deviceKeyConfigured = true; ready.hotkeyInstalled = false
        XCTAssertNil(ready.recordingBlocker) // The on-screen Record button still works.
        XCTAssertTrue(ready.hotkeyStatus.contains("Not active"))
    }

    func testMultipartOnlyApprovedFieldsAndBearerAuthentication() throws {
        let audio = Data(repeating: 65, count: 100)
        let (request, body) = try VokiriClient.request(audio: audio, configuration: VokiriConfiguration(), key: "unit-test-placeholder", boundary: "TestBoundary")
        XCTAssertEqual(request.httpMethod, "POST")
        XCTAssertEqual(request.url?.absoluteString, VokiriConfiguration.defaultEndpoint)
        XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer unit-test-placeholder")
        XCTAssertEqual(request.value(forHTTPHeaderField: "Content-Type"), "multipart/form-data; boundary=TestBoundary")
        let multipart = String(decoding: body, as: UTF8.self)
        XCTAssertTrue(multipart.contains("name=\"file\"; filename=\"recording.wav\"\r\nContent-Type: audio/wav\r\n\r\n"))
        XCTAssertTrue(multipart.contains(String(decoding: audio, as: UTF8.self)))
        XCTAssertTrue(multipart.contains("name=\"model\"\r\n\r\nwhisper-large-v3-turbo\r\n"))
        XCTAssertTrue(multipart.contains("name=\"response_format\"\r\n\r\njson\r\n"))
        XCTAssertTrue(multipart.hasSuffix("--TestBoundary--\r\n"))
        for name in ["language", "profile_id", "prompt", "context", "clipboard", "selected_text", "temperature", "dictionary"] {
            XCTAssertFalse(multipart.contains("name=\"\(name)\""), name)
        }
        var config = VokiriConfiguration(); config.profileID = "mac-profile"
        let (_, configured) = try VokiriClient.request(audio: audio, configuration: config, key: "test")
        XCTAssertTrue(String(decoding: configured, as: UTF8.self).contains("name=\"profile_id\"\r\n\r\nmac-profile\r\n"))
    }

    func testExactDecodeWithUnknownMetadata() throws {
        let data = try JSONSerialization.data(withJSONObject: ["text": exact, "metadata": ["anything": true], "duration": "unknown"])
        XCTAssertEqual(Array(try VokiriClient.decode(data, status: 200).utf8), Array(exact.utf8))
    }

    func testInvalidEmptyAndAuthenticationResponses() throws {
        for input in ["{}", "{\"text\":null}", "{\"text\":12}", "[]", "invalid"] {
            XCTAssertThrowsError(try VokiriClient.decode(Data(input.utf8), status: 200)) { XCTAssertEqual($0 as? VokiriError, .malformedResponse) }
        }
        for text in ["", " \n\t"] {
            let data = try JSONSerialization.data(withJSONObject: ["text": text])
            XCTAssertThrowsError(try VokiriClient.decode(data, status: 200)) { XCTAssertEqual($0 as? VokiriError, .emptyResponse) }
        }
        XCTAssertThrowsError(try VokiriClient.decode(Data("sensitive error body".utf8), status: 401)) { XCTAssertEqual($0 as? VokiriError, .authentication) }
        XCTAssertThrowsError(try VokiriClient.decode(Data(), status: 503)) { XCTAssertEqual($0 as? VokiriError, .http(503)) }
    }

    func testRejectUnsafeEndpointKeyAndAudio() throws {
        for endpoint in ["http://example.com", "https://user:password@example.com", "https://example.com?key=test", "https://example.com/#fragment", "file:///tmp/audio"] {
            var config = VokiriConfiguration(); config.endpoint = endpoint
            XCTAssertThrowsError(try config.validatedURL())
        }
        for key in ["", "bad\r\nInjected: true"] {
            XCTAssertThrowsError(try VokiriClient.request(audio: Data(repeating: 0, count: 100), configuration: VokiriConfiguration(), key: key))
        }
        XCTAssertThrowsError(try VokiriClient.request(audio: Data(), configuration: VokiriConfiguration(), key: "test"))
    }

    func testActualURLSessionUploadAndTimeout() async throws {
        let file = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".wav")
        try Data(repeating: 1, count: 100).write(to: file)
        defer { try? FileManager.default.removeItem(at: file); MockURLProtocol.handler = nil }
        let config = URLSessionConfiguration.ephemeral; config.protocolClasses = [MockURLProtocol.self]
        let client = VokiriClient(session: URLSession(configuration: config))
        let expected = exact
        var calls = 0
        MockURLProtocol.handler = { request in
            calls += 1
            XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer test-device")
            return (200, try JSONSerialization.data(withJSONObject: ["text": expected, "extra": true]))
        }
        let text = try await client.transcribe(audioURL: file, configuration: VokiriConfiguration(), key: "test-device")
        XCTAssertEqual(text, expected); XCTAssertEqual(calls, 1)
        MockURLProtocol.handler = { _ in throw URLError(.timedOut) }
        do { _ = try await client.transcribe(audioURL: file, configuration: VokiriConfiguration(), key: "test-device"); XCTFail() }
        catch { XCTAssertEqual(error as? VokiriError, .timeout) }
    }

    @MainActor func testPreservationThroughHistoryAndPaste() async throws {
        let store = try temporaryStore(); defer { cleanup(store) }
        let client = ControlledClient(); client.result = .success(exact)
        let workflow = VokiriWorkflow(store: store, client: client)
        var pasted: String?
        workflow.submit(try store.create(), configuration: VokiriConfiguration(), key: "test") { text, canceled in
            XCTAssertFalse(canceled()); pasted = text; return true
        }
        await workflow.waitUntilSettled()
        XCTAssertEqual(pasted, exact)
        let stored = try XCTUnwrap(store.list().first)
        XCTAssertEqual(Array(stored.text!.utf8), Array(exact.utf8))
        XCTAssertEqual(stored.status, .completed); XCTAssertEqual(workflow.state, .success)
    }

    @MainActor func testCancellationIgnoresLateResponseAndRetainsAudio() async throws {
        let store = try temporaryStore(); defer { cleanup(store) }
        let record = try store.create(), audio = Data(repeating: 2, count: 100)
        try audio.write(to: store.audioURL(record))
        let client = ControlledClient(); client.suspend = true
        let workflow = VokiriWorkflow(store: store, client: client)
        var pasteCount = 0
        workflow.submit(record, configuration: VokiriConfiguration(), key: "test") { _, _ in pasteCount += 1; return true }
        while client.continuation == nil { await Task.yield() }
        workflow.cancel(); client.continuation?.resume(returning: "late server text")
        await workflow.waitUntilSettled()
        XCTAssertEqual(pasteCount, 0)
        XCTAssertEqual(try store.list().first?.status, .canceled)
        XCTAssertEqual(try Data(contentsOf: store.audioURL(record)), audio)
    }

    @MainActor func testCancellationDuringPasteDelay() async throws {
        let store = try temporaryStore(); defer { cleanup(store) }
        let workflow = VokiriWorkflow(store: store, client: ControlledClient())
        var continuePaste: CheckedContinuation<Void, Never>?, pasted = false
        workflow.submit(try store.create(), configuration: VokiriConfiguration(), key: "test") { _, canceled in
            await withCheckedContinuation { continuePaste = $0 }
            if !canceled() { pasted = true }
            return pasted
        }
        while continuePaste == nil { await Task.yield() }
        workflow.cancel(); continuePaste?.resume()
        await workflow.waitUntilSettled()
        XCTAssertFalse(pasted); XCTAssertEqual(try store.list().first?.text, "text")
    }

    @MainActor func testFailureRetentionAndExplicitRetryWithoutAutomaticRetry() async throws {
        let store = try temporaryStore(); defer { cleanup(store) }
        let record = try store.create(), audio = Data(repeating: 3, count: 100)
        try audio.write(to: store.audioURL(record))
        let client = ControlledClient(); client.result = .failure(VokiriError.authentication)
        let workflow = VokiriWorkflow(store: store, client: client)
        var pasted: String?
        workflow.submit(record, configuration: VokiriConfiguration(), key: "test") { text, _ in pasted = text; return true }
        await workflow.waitUntilSettled()
        XCTAssertNil(pasted); XCTAssertEqual(client.calls, 1)
        XCTAssertEqual(try store.list().first?.status, .failed)
        XCTAssertEqual(try Data(contentsOf: store.audioURL(record)), audio)
        client.result = .success(exact)
        workflow.submit(try XCTUnwrap(store.list().first), configuration: VokiriConfiguration(), key: "replacement") { text, _ in pasted = text; return true }
        await workflow.waitUntilSettled()
        XCTAssertEqual(client.calls, 2); XCTAssertEqual(pasted, exact); XCTAssertEqual(try store.list().count, 1)
    }

    @MainActor func testPasteFailureKeepsTextWithoutAnotherUpload() async throws {
        let store = try temporaryStore(); defer { cleanup(store) }
        let client = ControlledClient()
        let counted = VokiriWorkflow(store: store, client: client)
        counted.submit(try store.create(), configuration: VokiriConfiguration(), key: "test") { _, _ in false }
        await counted.waitUntilSettled()
        XCTAssertEqual(client.calls, 1); XCTAssertEqual(try store.list().first?.status, .completed)
        XCTAssertEqual(try store.list().first?.text, "text"); XCTAssertEqual(counted.state, .failed(VokiriError.paste.localizedDescription))
    }

    func testPrivateStorageAndInterruptedRecovery() throws {
        let store = try temporaryStore(); defer { cleanup(store) }
        let record = try store.create()
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: store.root.path)[.posixPermissions] as? Int, 0o700)
        XCTAssertEqual(try FileManager.default.attributesOfItem(atPath: store.audioURL(record).path)[.posixPermissions] as? Int, 0o600)
        try store.recoverInterrupted(); XCTAssertEqual(try store.list().first?.status, .failed)
    }

    private func temporaryStore() throws -> VokiriStore {
        try VokiriStore(root: FileManager.default.temporaryDirectory.appendingPathComponent("vokiri-tests-\(UUID())/Recordings"))
    }
    private func cleanup(_ store: VokiriStore) { try? FileManager.default.removeItem(at: store.root.deletingLastPathComponent()) }
}
