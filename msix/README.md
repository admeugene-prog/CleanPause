# CleanPause MSIX preview / Предварительная сборка MSIX

Build: `./build-msix.ps1 -Version 1.0.2`. The script builds/tests the application,
downloads pinned Microsoft Windows SDK BuildTools from NuGet, checks MakeAppx's
Microsoft signature, and creates `dist/CleanPause.msix` with schema validation.
The package targets Windows 10 2004+ x64. No private signing keys are used.

GitHub Actions includes the package and its SHA256 checksum in build artifacts
and future tag releases. The package is **unsigned and experimental**. Ordinary
double-click installation is unavailable until it is signed or distributed by
the Microsoft Store. This is not a Store-certified release.

## Partner Center identity

Register your application first. Copy the exact Package/Identity/Name,
Package/Identity/Publisher and publisher display name from Partner Center's
app identity page. The default identity now matches the CleanPause Partner Center listing: EvgeniiALEKSEEV.CleanPause, publisher CN=6E79EBE5-4570-4408-87D0-17F295D9EA33, display name Evgenii ALEKSEEV.

Optional overrides use these repository **Actions variables** (not secrets):

- `STORE_IDENTITY_NAME`
- `STORE_PUBLISHER`
- `STORE_PUBLISHER_DISPLAY_NAME`

Or pass `-IdentityName`, `-Publisher`, and `-PublisherDisplayName` to the script.
Versions use `MAJOR.MINOR.PATCH.0`; all components must fit in 16 bits.

## Capabilities and validation still required

The desktop application declares `runFullTrust` and `allowElevation` because
it explicitly relaunches with a UAC prompt to use BlockInput. Microsoft requires
additional review for restricted capabilities; acceptance is not guaranteed.
Explain the cleaning timer, voluntary input blocking, Ctrl+Alt+Del recovery,
worker watchdog, and why elevation is needed in the certification notes.

MSIX startup uses the declared `CleanPauseStartup` StartupTask. It is managed in
Windows Settings → Apps → Startup; the application's ordinary Run-registry
checkbox is disabled for packaged processes. The package never overwrites or
deletes an unpackaged installation's Run entry. Windows can disable startup.
Startup and UAC behavior must be tested together on the target Windows versions.

Before submission, test a signed package in a disposable Windows user/VM:
installation, Start menu launch, UAC accept/cancel, reminders, actual input
blocking, Ctrl+Alt+Del recovery, worker termination, login startup, language,
update, and uninstall. Do not run the EXE installation and MSIX copy together:
they share the application's single-instance mutex. MSIX can virtualize app
data/registry, so existing EXE settings are not assumed to migrate automatically.

Microsoft re-signs accepted MSIX packages; purchasing a trusted signing
certificate for the Store submission is not required.

References:

- [Store signing](https://learn.microsoft.com/en-us/windows/apps/publish/publish-your-app/package-version-numbering)
- [Restricted capabilities](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/app-capability-declarations)
- [StartupTask](https://learn.microsoft.com/en-us/uwp/api/windows.applicationmodel.startuptask)

## По-русски

Сборка: `./build-msix.ps1 -Version 1.0.2`. Пакет появляется в артефактах Actions;
будущие релизы по тегу тоже будут включать MSIX. Это предварительная сборка,
без подписи и без сертификации Store. Она не устанавливается обычным двойным
щелчком, пока не подписана или не распространяется через Store.

Для подачи в Store сначала создайте приложение в Partner Center и внесите
три значения Identity/Publisher в переменные Actions, перечисленные выше.
Автозапуск пакетной версии управляется через «Параметры Windows → Приложения →
Автозагрузка». Повышение прав требует отдельного рассмотрения Microsoft.
Установка и реальные сценарии блокировки/восстановления ввода в MSIX ещё
требуют проверки на отдельном профиле или виртуальной машине.
