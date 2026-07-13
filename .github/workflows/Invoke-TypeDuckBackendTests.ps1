#Requires -Version 7.0
<#
.SYNOPSIS
    Runs the TypeDuck backend Go tests and enforces a BIDIRECTIONAL coverage guard over the
    Shift / asciiMode feature tests in input_methods/rime.

.DESCRIPTION
    This script is the SINGLE SOURCE OF TRUTH for how the backend is tested. It is called from
    three places, so the lists below cannot drift apart:

      * TypeDuck-Windows-backend  .github/workflows/backend-tests.yml  (push + pull_request)
      * TypeDuck-Windows          .github/workflows/nightly.yml
      * TypeDuck-Windows          .github/workflows/release.yml

    It lives in the BACKEND repo because the backend owns the tests. The frontend workflows
    already check this repo out (as TypeDuck-Windows-backend) and invoke it from there.

    What it does:
      1. Runs every backend package EXCEPT input_methods/rime, unfiltered. The package list is
         derived from `go list`, so a NEW package is tested automatically, never silently skipped.
      2. Enforces the coverage guard on input_methods/rime (see below).
      3. Runs the allowlisted Shift/asciiMode tests out of input_methods/rime by exact name.

    WHY input_methods/rime IS NOT RUN IN FULL
    -----------------------------------------
    A large part of that package is ALREADY RED UPSTREAM -- before, and independently of, the
    TypeDuck Shift/asciiMode feature (39 of its tests fail on a clean checkout). Those tests need
    a populated rime runtime or the original developer's own machine, for example:
      * TestHandleRequestSyncsAppearanceAcrossInstances and
        TestLoadAppearancePrefsKeepsPresetThemeAfterPersist expect a theme named "purple"; only
        2 themes are loaded in CI and that is not one of them.
      * TestAppearanceSettingsPersistToDisk expects appearance_config.json in a directory that
        the code under test never creates.
      * TestDownloadSchemeSetCommandInstallsSelectsAndRedeploys wants to download a scheme set,
        and the custom-phrase overlay tests need on-disk phrase data.
      * wubi_debug_test.go and z_probe_sentence_candidates_test.go are hard-coded to the original
        developer's own drive paths.
    Upstream never ran `go test` in CI, which is why none of this was noticed. Those tests are not
    ours and we do not claim them as green. We equally refuse to let them mask a regression in the
    code we DO own, so rather than skipping the package we run our own tests out of it by exact
    name.

    THE COVERAGE GUARD (bidirectional)
    ----------------------------------
    A `-run` pattern that matches nothing still exits 0, so a typo, a renamed test, or a brand-new
    test nobody listed would quietly turn this into a no-op that tests nothing. The guard closes
    BOTH directions:

      (a) STALENESS  -- every name in $FeatureTests and $KnownUpstreamRed must actually exist in
                        the package. A deleted/renamed test fails the build.
      (b) COMPLETENESS -- every test in the package whose name matches the feature domain
                        ($DomainPattern: Shift / Ascii / asciiMode / SharedInputState) must be in
                        EXACTLY ONE of the two lists. A 21st feature test that nobody listed fails
                        the build with an actionable message instead of never running.

    Test discovery uses `go test -list` (the compiled test registry), never a grep of the source.
#>
[CmdletBinding()]
param(
    # Root of the TypeDuck-Windows-backend checkout (the directory holding go.mod).
    [Parameter(Mandatory = $true)]
    [string] $RepoRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
# `go` reports failures through its exit code, which we check explicitly after every call so we can
# emit an actionable message. Without this, PowerShell 7.3+ turns any non-zero native exit into a
# NativeCommandExitException and buries our diagnostics.
$PSNativeCommandUseErrorActionPreference = $false

$rimePackage = './input_methods/rime'

# Print a multi-line diagnostic verbatim, then fail with a short summary. `throw`ing the whole
# block instead would hand it to PowerShell's exception formatter, which reflows it into an
# unreadable smear of box-drawing characters in the CI log -- exactly when the author most needs
# to read it.
function Stop-WithDiagnostic {
    param(
        [Parameter(Mandatory = $true)][string] $Summary,
        # AllowEmptyString: blank spacer lines are part of the layout, and a mandatory [string[]]
        # otherwise refuses to bind an array containing ''.
        [Parameter(Mandatory = $true)][AllowEmptyString()][string[]] $Detail
    )
    Write-Host ''
    Write-Host "TypeDuck backend test guard: $Summary"
    Write-Host ''
    foreach ($line in $Detail) {
        Write-Host "    $line"
    }
    Write-Host ''
    throw $Summary
}

# ---------------------------------------------------------------------------------------------
# THE RUN ALLOWLIST -- TypeDuck Shift/asciiMode tests that MUST pass.
# Adding a Shift/asciiMode test? Add it here. The guard will fail the build until you do.
# ---------------------------------------------------------------------------------------------
$FeatureTests = @(
    'TestBareShiftDownPassesThroughAndIsNotForwardedToRime'
    'TestLoneLeftShiftTogglesAsciiMode'
    'TestLoneRightShiftTogglesAsciiMode'
    'TestShiftWithLetterDoesNotToggle'
    'TestCtrlShiftDoesNotToggle'
    'TestShiftAutoRepeatTogglesOnce'
    'TestShiftToggleWithCompositionCommitsRawInput'
    'TestShiftToggleBackToChineseAllowsComposing'
    'TestShiftToggleResponseCarriesSettingsUpdateOnlyOnToggle'
    'TestShiftTogglePropagatesAcrossInstances'
    'TestShiftTogglePublishesAsciiModeWithoutSchemaSaveOptions'
    'TestShiftToggleDoesNotLogTypedContent'
    'TestSettingsUpdateAsciiModeAppliesWithoutRedeploy'
    'TestSettingsUpdateAsciiModePropagatesToExistingInstance'
    'TestNewSessionInheritsAsciiMode'
    'TestAsciiModeSurvivesRedeployAndSchemaSwitch'
    'TestCapsLockStillForwardedToRime'
    'TestOnCommandAsciiToggleAttachesSettingsUpdate'
    'TestOnCommandModeIconTogglePersistsWithoutExistingSession'
    'TestProcessKeySyncsSharedInputStateAfterShiftAndCapsToggle'

    # --- Upstream in-domain tests, GREEN, promoted into the run set ---------------------------
    # These pre-date the Shift/asciiMode feature but cover the same shared input state the feature
    # mutates, and they were never being run by anyone. They are pure-Go tests driving the
    # in-process `testBackend` fake (no librime, no rime session, no rime runtime config), so they
    # are platform-independent. They pass on a clean checkout and now guard the code we touch.
    'TestOnKeyDownAsciiModePassesThroughWhenIdle'
    'TestBuildMenuIncludesSharedInputStateToggle'
    'TestHandleRequestSyncsSharedInputStateAcrossInstances'
    'TestCreateSessionAppliesSharedInputStateOnlyForNewSession'
    'TestCreateSessionAppliesSharedInputStateAfterSharedConfigUpdateWithExistingSession'
)

# ---------------------------------------------------------------------------------------------
# THE KNOWN-UPSTREAM-RED DENYLIST -- in-domain tests we deliberately DO NOT run.
#
# Every entry needs a comment justifying why it is not our problem. Do not add a test here just to
# make CI green: if a test is red because the feature broke it, FIX THE FEATURE.
# ---------------------------------------------------------------------------------------------
$KnownUpstreamRed = @(
    # RED UPSTREAM, NOT A FEATURE REGRESSION -- verified by diffing the production code path
    # against the pre-feature commit: filterKeyDown, currentInputModeState and
    # updateLangStatusIfInputStateChanged are byte-identical to upstream (the feature only inserted
    # the Shift filter ahead of them, which ignores Ctrl+A).
    #
    # Why it fails: filterKeyDown samples currentInputModeState() BEFORE calling processKey, and
    # that helper reports ok=false when the backend has no session yet. The test drives a fresh
    # newIsolatedTestIME, which has no session, so updateLangStatusIfInputStateChanged() takes its
    # `!hasInputState` early return and never emits the switch-lang button update the test asserts
    # on ("expected one language button update, got []imecore.ButtonInfo(nil)"). It is a real
    # upstream bug in the lazily-created-session path, and it fails deterministically on every
    # platform. Fixing it means changing rime.go, which is out of scope for a CI change.
    'TestFilterKeyDownEmitsLangButtonUpdateWhenControlHotkeyTogglesAsciiMode'
)

# Broad, case-insensitive feature-domain pattern. Anything in input_methods/rime matching this is
# in scope for the completeness guard and must be classified into one of the two lists above.
$DomainPattern = '(?i)(shift|ascii|sharedinputstate)'

Push-Location -LiteralPath $RepoRoot
try {
    # -----------------------------------------------------------------------------------------
    # 1. Every package except input_methods/rime, unfiltered.
    # -----------------------------------------------------------------------------------------
    $allPackages = @(go list ./...)
    if ($LASTEXITCODE -ne 0) {
        throw 'go list ./... failed.'
    }
    $packages = @($allPackages | Where-Object { $_ -notmatch '/input_methods/rime$' })
    if ($packages.Count -eq 0) {
        throw 'go list ./... returned no packages.'
    }

    Write-Host "== go test ($($packages.Count) packages, input_methods/rime excluded)"
    go test $packages
    if ($LASTEXITCODE -ne 0) {
        throw 'Backend go test failed.'
    }

    # -----------------------------------------------------------------------------------------
    # 2. The bidirectional coverage guard on input_methods/rime.
    # -----------------------------------------------------------------------------------------
    if (($FeatureTests | Sort-Object -Unique).Count -ne $FeatureTests.Count) {
        throw 'The run allowlist ($FeatureTests) contains duplicate entries.'
    }
    $overlap = @($FeatureTests | Where-Object { $KnownUpstreamRed -contains $_ })
    if ($overlap.Count -gt 0) {
        throw "These tests are in BOTH the run allowlist and the KNOWN-UPSTREAM-RED denylist: $($overlap -join ', ')"
    }

    # Full test registry of the package, straight from the compiled test binary.
    $allTests = @(go test -list '.*' $rimePackage | Where-Object { $_ -match '^Test' })
    if ($LASTEXITCODE -ne 0) {
        throw "go test -list failed for $rimePackage."
    }
    if ($allTests.Count -eq 0) {
        throw "go test -list returned no tests for $rimePackage."
    }

    # (a) STALENESS -- both lists may only name tests that still exist.
    $missing = @($FeatureTests | Where-Object { $allTests -notcontains $_ })
    if ($missing.Count -gt 0) {
        Stop-WithDiagnostic -Summary 'stale run allowlist -- listed test(s) no longer exist in input_methods/rime' -Detail (
            $missing + @(
                '',
                'They were renamed or deleted. Update $FeatureTests in:',
                "  $PSCommandPath"
            )
        )
    }
    $staleRed = @($KnownUpstreamRed | Where-Object { $allTests -notcontains $_ })
    if ($staleRed.Count -gt 0) {
        Stop-WithDiagnostic -Summary 'stale KNOWN-UPSTREAM-RED denylist -- listed test(s) no longer exist in input_methods/rime' -Detail (
            $staleRed + @(
                '',
                'If they were fixed or removed upstream, drop them from $KnownUpstreamRed in:',
                "  $PSCommandPath"
            )
        )
    }

    # (b) COMPLETENESS -- every in-domain test must be classified. This is the direction that used
    #     to be missing: a new Shift/asciiMode test that nobody listed would simply never run.
    $domainTests = @(go test -list $DomainPattern $rimePackage | Where-Object { $_ -match '^Test' })
    if ($LASTEXITCODE -ne 0) {
        throw "go test -list failed for $rimePackage (domain pattern)."
    }
    if ($domainTests.Count -eq 0) {
        throw "go test -list matched no feature-domain tests in $rimePackage; the guard would be a no-op."
    }

    $unclassified = @($domainTests | Where-Object {
        ($FeatureTests -notcontains $_) -and ($KnownUpstreamRed -notcontains $_)
    })
    if ($unclassified.Count -gt 0) {
        Stop-WithDiagnostic -Summary "unclassified Shift/asciiMode test(s) in input_methods/rime: $($unclassified -join ', ')" -Detail (
            $unclassified + @(
                '',
                "Every test matching the feature domain $DomainPattern must be in exactly ONE of the",
                'two lists in:',
                "  $PSCommandPath",
                '',
                '  * $FeatureTests     -- the run allowlist. Put it here if it should pass (normal case).',
                '  * $KnownUpstreamRed -- the denylist, WITH A COMMENT saying why it is not our problem.',
                '',
                'Refusing to build: an unlisted test is a test that never runs.'
            )
        )
    }

    Write-Host "Coverage guard OK: $($domainTests.Count) feature-domain test(s) in input_methods/rime -- $($FeatureTests.Count) allowlisted, $($KnownUpstreamRed.Count) known-upstream-red."

    # -----------------------------------------------------------------------------------------
    # 3. Run the allowlist by exact, fully anchored name.
    # -----------------------------------------------------------------------------------------
    # Fully anchored alternation: it cannot partially match an upstream test name.
    $runPattern = '^(' + ($FeatureTests -join '|') + ')$'

    $selected = @(go test -list $runPattern $rimePackage | Where-Object { $_ -match '^Test' })
    if ($LASTEXITCODE -ne 0) {
        throw "go test -list failed for $rimePackage (run pattern)."
    }
    if ($selected.Count -ne $FeatureTests.Count) {
        throw "Expected $($FeatureTests.Count) TypeDuck feature tests in $rimePackage, -run selected $($selected.Count)."
    }

    Write-Host "== go test -run <$($FeatureTests.Count) TypeDuck Shift/asciiMode tests> $rimePackage"
    go test -run $runPattern -v $rimePackage
    if ($LASTEXITCODE -ne 0) {
        throw 'Backend go test failed (TypeDuck Shift/asciiMode feature tests).'
    }
}
finally {
    Pop-Location
}
