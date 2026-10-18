# GitHub release updates

1. Keep the completed 0.3.4 integration-settings delivery as a separate commit. Align future Desktop versions with CLI release tags, beginning at 0.4.51.
2. Add a tested main-process update controller and GitHub release discovery. Only stable releases with Desktop installer metadata qualify. Pin downloads to that release; let electron-updater verify checksums. Deduplicate checks/downloads and install only a downloaded update after safe runtime shutdown.
3. Check once after the workspace and initial startup screen are ready. Animate availability and real download progress, show a restart request when downloaded, expose retry on failure, and provide no Later button. Keep unsaved-file and active-task safeguards.
4. Add Desktop sources under CLI/desktop and a Windows release job after the existing CLI release job. Build the bundled runtime from the same Git commit, set both versions from the release tag, and upload installer, blockmap and latest.yml to that release. Provide a repeatable source-sync script.
5. Verify controller transitions/errors/concurrency and restart guards, real Electron update visuals/readiness, the existing desktop suite, packaged assets/runtime hash, and Windows NSIS packaging. Do not publish a tag or release merely to test the updater.

6. Deliver a separate frameless Electron installer with fully custom React controls. Bundle the verified silent NSIS payload offline; verify installed files before success. Keep internal runtime terminology out of the visible interface. The release workflow publishes the custom installer and separate updater payload together.
