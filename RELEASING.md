# Releasing NoiseFree

Publishing a GitHub release with a tag like `v1.0.0` runs
`.github/workflows/release.yml`. The tag sets the version and the run number
the build number. The workflow tests, builds every package and attaches them to
the release. The workflow is written by the Giode CLI from `giode.toml`: after
changing `[ci]` or a store section, run `giode ci -f` instead of editing it,
then redo the edits listed in the `[ci]` comment of `giode.toml` by hand:

- the two pins: the CLI at `giode-cli/cmd/giode@v0.1.35-alpha.1` instead of
  `@latest`, and `go-version-file: go.mod` instead of `go-version: stable`;
- the Windows job's **MSIX microphone capability** step
  (`go run ./internal/cmd/msixmic dist/windows/*.msix`);
- the macOS job's **Build** step running `.github/scripts/macos-dmg.sh` instead
  of `giode build $release dmg`.

The last two work around what CLI v0.1.35-alpha.1 can't express yet: the MSIX
manifest declares only `runFullTrust`, and Developer ID builds are signed with
Go's entitlements alone, so the hardened runtime would block the microphone.

| Where | What the release gets | Needs |
| --- | --- | --- |
| GitHub, Linux | `.deb` and `.rpm` (x64) | nothing |
| GitHub, Windows | `setup.exe` installer (x64) | nothing; a code signing certificate stops SmartScreen warnings |
| GitHub, macOS | a universal `.dmg`, signed with the microphone entitlement and notarized | a Developer ID certificate and notarization secrets, or macOS refuses to open it |
| Mac App Store | `NoiseFree-<version>.pkg` to upload with Transporter | an Apple Developer account, the store certificates and profile |
| Microsoft Store | `NoiseFree-<version>-amd64.msix` to upload in Partner Center | a Partner Center account and the reserved name |

NoiseFree is a desktop app (Windows, macOS, Linux): there is no Google Play or
iOS App Store package. Its job is a virtual microphone for other desktop apps,
built on desktop audio drivers and LADSPA plugins, which phones don't have.

## Repository secrets

Set these in GitHub under Settings → Secrets and variables → Actions. Each
`*_BASE64` secret is the file encoded with `base64 -i file | pbcopy` on macOS,
or `base64 -w0 file` on Linux. Without a secret, that step is skipped and the
rest of the release still builds.

| Secret | What it is |
| --- | --- |
| `MACOS_CERT_BASE64`, `MACOS_CERT_PASS` | The "Developer ID Application" certificate as a `.p12`, and its password: signs the `.dmg` |
| `GIODE_NOTARY_ID`, `GIODE_NOTARY_TEAM_ID`, `GIODE_NOTARY_PASS` | Your Apple ID, team ID and an app-specific password: notarizes the `.dmg` |
| `MACOS_STORE_CERT_BASE64`, `MACOS_STORE_CERT_PASS` | The "Apple Distribution" certificate as a `.p12` |
| `MACOS_INSTALLER_CERT_BASE64`, `MACOS_INSTALLER_CERT_PASS` | The "Mac Installer Distribution" certificate as a `.p12` |
| `MACOS_PROFILE_BASE64` | The Mac App Store provisioning profile (`.provisionprofile`) |
| `WINDOWS_CERT_BASE64`, `GIODE_WINDOWS_CERT_PASS` | An Authenticode `.pfx` and its password (optional; also set `[windows.signing] certificate = "codesign.pfx"` in `giode.toml` for giode to use it) |

## Mac App Store

1. Join the Apple Developer Program.
2. In Certificates, Identifiers & Profiles:
   - Register the App ID `io.github.xmhrnx.noisefree` (the `app.id` in
     `giode.toml`). It needs no extra capabilities.
   - Create an "Apple Distribution" and a "Mac Installer Distribution"
     certificate, and export each as a `.p12`.
   - Create a "Mac App Store" provisioning profile for the App ID.
   - For the GitHub `.dmg`, also create a "Developer ID Application"
     certificate, and an app-specific password for your Apple ID.
3. In App Store Connect, create a macOS app with the same bundle ID and the
   name "NoiseFree" (the SKU can be `noisefree`).
4. Add the Mac App Store secrets above, then publish a release. The release
   gets `NoiseFree-<version>.pkg`; upload it with the Transporter app.
5. In App Store Connect, fill in the listing (copy below):
   - **App Privacy:** "Data Not Collected".
   - **Privacy policy URL:** https://xmhrnx.github.io/noisefree/#privacy
   - **Category:** Utilities (already set in the build).
   - **Export compliance:** answered in the build
     (`ITSAppUsesNonExemptEncryption = false`).
   - **Age rating:** none of the questionnaire's content; 4+.
   - **Review notes:** "NoiseFree removes background noise from the
     microphone. To try it without extra software, turn on Monitor on the
     Microphone page, or use Preview to record 10 seconds and compare. To
     hand the clean voice to other apps it plays into a virtual audio device
     such as the free BlackHole driver, which the user installs separately
     from its website; NoiseFree only links to it."

The build is sandboxed with `device.audio-input` (the microphone),
`network.client` and `network.server` (giode's single-instance lock talks over
a loopback socket), and declares the microphone usage text.

**The App Sandbox limits NoiseFree in the store version:**

- **Engines:** only the two built into the app, NoiseFree Lite and RNNoise.
  The Engines page hides **Add engine** and lists no downloads: the store
  forbids downloading code (guideline 2.5.2) and the sandbox won't load
  other developers' LADSPA plugins or ONNX Runtime. So RNNoise Voice Gate,
  DTLN and Core ML GPU acceleration are GitHub-only on macOS.
- **Virtual microphone:** the user installs BlackHole (or Loopback) themselves,
  as with the GitHub build. Review may ask about this (guideline 4.2.3); the
  review note above explains it.
- **Start at login:** works on macOS 13 and later (SMAppService); earlier
  versions show an error.

The notarized `.dmg` on GitHub has none of these limits.

## Microsoft Store

1. In Partner Center (the same account as EasySend and AnyFile), reserve the
   name "NoiseFree".
2. `giode.toml` already has the account's values under `[windows.msix]`:
   `publisher = "CN=B40E8F57-A7E0-40BB-8E74-5D051D0B578B"` and
   `publisher_display_name = "Mehran Zahiroddini"`. `identity_name` is set to
   `MehranZahir.NoiseFree`: once the name is reserved, open the product's
   Product identity page and check that **Package/Identity/Name** matches it
   exactly; if not, change it and commit before the release. If the reserved
   name differs from "NoiseFree" (say it was taken), the MSIX's display name
   must match it too.
3. Publish a release. It gets `NoiseFree-<version>-amd64.msix`; upload it in a
   Partner Center submission. The Store signs it, so it needs no certificate
   here.
4. The package declares `runFullTrust` and the `microphone` device capability.
   When the submission asks why it uses `runFullTrust`, explain: "NoiseFree is
   a desktop audio app: it captures the microphone with low latency, plays the
   cleaned audio into a virtual audio cable driver, and restores the user's
   default audio devices when a cable driver takes them over."
5. **Privacy policy URL:** https://xmhrnx.github.io/noisefree/#privacy.
   **Age rating (IARC):** no violence, language, purchases, user interaction
   or data sharing; it should come out as 3+ / Everyone.
6. Engines from the Engines page are downloaded from their projects (GitHub,
   NuGet) when the user installs them; they are described in the listing as
   part of what the app does (Store policy 10.2.2 allows this when the
   functionality is described).

Startup at login uses the package's startup task (`startup_task = true`), so
Settings → Start at login works in the Store version.

## Store listing

**Name:** NoiseFree

**Subtitle / short description (≤ 80 characters):**
Real-time AI noise removal and a virtual microphone for any GPU and CPU.

**Keywords (Apple, ≤ 100 characters):**
noise,microphone,mic,noise cancelling,rnnoise,voice,zoom,discord,streaming,calls,podcast

**Description:**

> NoiseFree removes background noise from your microphone in real time:
> keyboards, fans, traffic, chatter and room hum. Your clean voice goes to
> Discord, Zoom, Teams, OBS and the browser through a virtual microphone.
>
> It runs on any computer, not just the one with the big GPU. RNNoise, a small
> neural network, uses about 1–2% of one CPU core; NoiseFree Lite, a pure
> spectral filter, even less. Both are built in and work offline.
>
> Hear the difference: record up to 10 seconds, play it back, and switch
> between the cleaned and the original take while it plays. Set the strength,
> add a rumble filter, a gate and gain, and watch live meters for your raw and
> cleaned voice.
>
> Your audio is processed on your computer and never leaves it. No account, no
> tracking.

Add for the Microsoft Store (and the GitHub builds), not for the Mac App Store:

> Install more engines from the Engines page, such as DeepFilterNet 3 and DTLN,
> which can run on any DirectX 12 GPU, or add your own LADSPA plugin. If a
> virtual cable driver makes itself your default speaker, NoiseFree switches
> your real devices back.

Add for the Mac App Store: "For the virtual microphone, install the free
BlackHole 2ch driver and pick BlackHole 2ch as the microphone in your apps."
For the Microsoft Store: "For the virtual microphone, install the free
VB-Audio Virtual Cable and pick CABLE Output as the microphone in your apps."

**Category:** Utilities (Mac App Store), Utilities & tools (Microsoft Store).

**Screenshots:** `docs/screenshots/` (home, preview, engines, home-light) are
1240 × 820, which neither store accepts. Capture the app again at the stores'
sizes: Mac 2880 × 1800 (or 1440 × 900), Microsoft Store 1920 × 1080. Show the
Microphone page with the meters moving, Preview, and Settings. Don't show
Add engine or downloadable engines in the Mac App Store screenshots: that
version doesn't have them.

**App Privacy / Data safety:** Data Not Collected. The privacy manifest
(`[privacy]` in `giode.toml`) declares no tracking and no collected data.

**Export compliance:** only standard encryption (HTTPS through Go and the OS)
for engine downloads; `ITSAppUsesNonExemptEncryption = false`.

## Linux

The `.deb` and `.rpm` are attached to the release. They list the system
libraries they need, so `apt` and `dnf` install those too.
