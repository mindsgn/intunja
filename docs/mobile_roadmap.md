Mobile roadmap: Running Intunja on iOS & Android (Expo / React Native / gomobile)

Goal
- Enable torrent downloading on iOS and Android using the existing Go engine where practical.
- Provide clear tradeoffs and implementation paths for Expo-managed React Native apps.

Executive summary
- Android: feasible to embed the Go engine via `gomobile` (AAR) and run it inside a foreground service. Use a native bridge (React Native native module) or a local HTTP/gRPC endpoint the JS layer talks to. Requires a custom build (not pure Expo managed) or EAS custom dev client.
- iOS: platform background restrictions make long-lived torrent engines impractical. Two realistic options:
  1) Use a remote daemon/server (recommended) — run engine on a remote host and control it from the app via REST/gRPC.
  2) Build a short-lived local engine using `gomobile` for foreground-only download while app is active (limited usefulness). Apple Store rules and background runtime limits discourage on-device torrenting.
- Expo: Managed workflow cannot include native Go bindings; use EAS Build + prebuild or a custom dev client to include native modules. Alternatively, use a server-hosted engine and keep the Expo app pure JS (best for managed workflow).

Roadmap and tasks
1) Discovery & constraints (1-2 days)
   - Confirm App Store policy risk for on-device torrenting. (Legal/IP policy + distribution risk.)
   - Verify the engine depends on native syscalls or libraries that `gomobile` cannot bind (cgo, low-level sockets). Identify build issues.
   - Decide port and RPC method (HTTP JSON or gRPC) between JS and engine.

2) Architecture decision (1 day)
   - Pick one of: (A) Remote daemon + mobile control (fastest, safest), (B) Android-first native engine + remote for iOS backup, (C) Full native on-device for both platforms (highest effort, high App Store risk).

3) Prototype gomobile binding (3-5 days)
   - Create a minimal Go package exposing a small API: StartEngine(config JSON), StopEngine(), AddMagnet(uri), ListTorrents() -> JSON.
   - Use `gomobile bind -target=android` to produce an AAR. Build a minimal Android app that loads the AAR and calls the API.
   - Resolve any issues with `anacrolix/torrent` building with `gomobile` (cgo, threads, syscalls). If issues arise, consider building the engine as a separate native service (via NDK) or use a thin REST server approach.

4) Android integration (5-10 days)
   - Create an Android foreground `Service` that runs the engine in a separate thread/process if needed.
   - Expose a local HTTP/gRPC endpoint or bind the `gomobile` APIs to a React Native native module.
   - Add startup/stop lifecycle: foreground notification for long-running downloads; request Doze-exempt or battery-optimized handling.
   - Implement storage permission handling and scoped storage support for API levels 29+.

5) iOS approach (3-7 days depending on decision)
   Option A — Remote daemon (recommended):
     - Build secure REST/gRPC API on server (use the existing server package). Add authentication (token). Use HTTPS.
     - React Native / Expo communicates with remote daemon; downloads occur server-side and app streams files or retrieves completed assets.
   Option B — Local `gomobile` framework (experimental):
     - `gomobile bind -target=ios` to produce a framework.
     - Integrate into Xcode project created by `react-native init` or Expo prebuild.
     - Note: iOS background execution limits will prevent persistent downloads. Use only for short foreground downloads.

6) React Native / Expo integration (2-4 days)
   - For native modules: create React Native native module wrappers for Android AAR and iOS framework.
   - For Expo-managed app: document EAS Build + prebuild steps to include native modules. Provide an `expo-dev-client` configuration for local development.
   - Provide a JS API that mirrors the CLI experience: connect(), addMagnet(), start(), stop(), list(), subscribeProgress(callback).
   - Consider using a small local websocket or EventEmitter to push torrent progress to JS.

7) Security, privacy, and store compliance (2-3 days)
   - Document privacy implications and include user-facing disclaimers.
   - Implement opt-in telemetry and clear permission prompts for storage and network.
   - For App Store: prepare arguments for review (if using remote server to avoid on-device P2P, easier acceptance). If shipping on-device torrenting, be prepared for rejection risk.

8) Testing and CI (2-4 days)
   - Add unit tests for the mobile binding wrapper (mock engine where possible).
   - Add end-to-end tests for Android foreground service using Firebase Test Lab or local emulators.
   - For Expo: add E2E tests using Detox or Appium.

9) Documentation and sample app (2-3 days)
   - Provide a sample React Native app (or Expo app + EAS config) demonstrating: start daemon (or connect to remote), add magnet, show progress, and save to device storage.

Risks & Mitigations
- App Store rejection: mitigate by preferring remote daemon architecture or restricting functionality on iOS; provide logs and justifications.
- Network / battery: torrenting is resource-heavy; require foreground service (Android) and warn users.
- Go native bindings complexity: `anacrolix/torrent` may use syscalls incompatible with `gomobile` — fallback to remote engine if binding fails.

Implementation details & recommendations
- RPC: Use REST JSON for simplicity (already present in `server/api.go`). For higher perf, use gRPC or a local socket.
- Fetching metadata: prefer magnet links; ensure DHT works on mobile networks.
- Storage: on Android, use scoped storage APIs; request `MANAGE_EXTERNAL_STORAGE` only if necessary.
- Background: Android: use a foreground `Service` with persistent notification. iOS: avoid attempting long background downloads.
- Expo: use `expo prebuild` + `EAS build` to include native modules or run a separate native app for complex scenarios.

Milestones & timeline (rough)
- Week 0: Discovery & architecture decision
- Week 1: Prototype gomobile binding; pick remote fallback if issues arise
- Week 2: Android integration & sample app
- Week 3: iOS remote integration or experimental local build
- Week 4: RN bridge, Expo EAS integration, tests, docs

Appendix: Useful links & tools
- gomobile: https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile
- Expo EAS: https://docs.expo.dev/eas/
- React Native native modules: https://reactnative.dev/docs/native-modules-intro
- Android foreground service docs: https://developer.android.com/guide/components/services
- Apple background execution: https://developer.apple.com/documentation/uikit/app_and_environment/managing_your_app_s_life_cycle


