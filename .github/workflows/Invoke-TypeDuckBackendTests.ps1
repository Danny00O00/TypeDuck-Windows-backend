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

    THE COVERAGE GUARD (bidirectional, plus a self-check on the domain itself)
    -------------------------------------------------------------------------
    A `-run` pattern that matches nothing still exits 0, so a typo, a renamed test, or a brand-new
    test nobody listed would quietly turn this into a no-op that tests nothing. The guard closes
    every direction:

      (a) STALENESS    -- every name in $FeatureTests and $KnownUpstreamRed must actually exist in
                          the package. A deleted/renamed test fails the build.
      (b) IN-DOMAIN    -- every name in $FeatureTests and $KnownUpstreamRed must be DISCOVERABLE by
                          $DomainPattern. See "WHY (b) EXISTS" below. This is what makes the domain
                          pattern honest instead of decorative.
      (c) COMPLETENESS -- every test in the package that $DomainPattern discovers must be in EXACTLY
                          ONE of the two lists. A new feature test that nobody listed fails the
                          build with an actionable message instead of never running.

    (a) + (b) + (c) together pin the invariant:

        { tests discovered by $DomainPattern } == $FeatureTests + $KnownUpstreamRed   (as SETS)

    WHY (b) EXISTS
    --------------
    Before (b), the two lists could name a test that $DomainPattern could not see, and nothing
    complained -- the test ran, so it looked fine. It was not fine. It meant the ALLOWLIST was
    carrying the test while DISCOVERY was blind to it, so a NEW test in that same vocabulary that
    nobody hand-listed would never be discovered, never be run, and CI would still pass. That is the
    exact hole the guard exists to close, reopened one rename at a time.
    Two real tests were in that position -- TestCapsLockStillForwardedToRime and
    TestOnCommandModeIconTogglePersistsWithoutExistingSession -- listed, running, invisible to a
    domain of (shift|ascii|sharedinputstate). They were in the allowlist by luck.
    (b) turns that from luck into an enforced rule: if you add a test to either list and its name is
    outside the domain vocabulary, the build fails and tells you to either RENAME THE TEST into the
    vocabulary or WIDEN $DomainPattern. Either way discovery and the lists can never drift apart
    again.

    CASE SENSITIVITY (this bit is a trap)
    -------------------------------------
    Go test names are case-SENSITIVE; PowerShell's -contains / -notcontains / -eq / -match and
    Where-Object string comparisons are all case-INsensitive by DEFAULT. Mixing the two means
    `TestFooBar` and `TestFoobar` collapse into one entry, and a test could be "classified" by an
    allowlist entry that is not actually its name -- evading the guard while looking listed.
    So: every comparison that CLASSIFIES a test name uses the case-sensitive operator
    (-ccontains / -cnotcontains / -cmatch / -cnotmatch, and Sort-Object -Unique -CaseSensitive).

    $DomainPattern itself stays case-INsensitive on purpose. It is a Go regexp handed to
    `go test -list`, and DISCOVERY must over-approximate: `(?i)` also catches `TestCapslock...` and
    `TestCAPSLOCK...`, which then have to be classified case-sensitively. Widening discovery is the
    safe direction; widening classification is not.

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

    # Surfaced by widening the domain from `sharedinputstate` to `inputstate`: its name says
    # InputStateSharedToggle, not SharedInputState, so the old pattern could not see it. It drives
    # ascii_mode through inputStateShared + captureSharedInputStateFromBackend -- the exact sync
    # machinery the Shift toggle mutates -- and asserts always-synced switcher options (emoji) still
    # cross instances while ascii_mode does NOT when input-state sharing is off. Pure-Go testBackend
    # fake, green on a clean checkout.
    'TestAlwaysSyncedSwitcherOptionsIgnoreInputStateSharedToggle'
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

# ---------------------------------------------------------------------------------------------
# THE FEATURE DOMAIN -- the vocabulary of the Shift/asciiMode feature, as a Go regexp.
#
# Anything in input_methods/rime whose test name matches this is in scope and must be classified
# into exactly one of the two lists above (check (c)); conversely nothing in those two lists may
# fall OUTSIDE it (check (b)). Case-insensitive ON PURPOSE -- see "CASE SENSITIVITY" in the header:
# discovery over-approximates, classification does not.
#
# Every token below is load-bearing, and each is as NARROW as it can be while still covering the
# feature's real vocabulary:
#
#   shift        the toggle itself.
#   ascii        asciiMode / ascii_mode, the state it toggles.
#   capslock     CapsLock is the key we must keep FORWARDING to rime while Shift is intercepted;
#                a CapsLock regression is a regression of this feature. NOT the bare token `caps` --
#                that also drags in TestBuildMenuCapsPerRowHighlightByCandidateCount ("CapsPerRow"),
#                a candidate-layout test we do not own.
#   modeicon     the tray/mode icon reflects and toggles the same ascii_mode state.
#   inputstate   the shared input state the toggle mutates and propagates across instances.
#                Deliberately WIDER than the old `sharedinputstate`: that spelling missed
#                TestAlwaysSyncedSwitcherOptionsIgnoreInputStateSharedToggle, which is squarely ours.
#
# Adding a token? Re-run and confirm it does not drag in tests we do not own -- a false positive
# here forces an unrelated author to classify a test they never touched.
$DomainPattern = '(?i)(shift|ascii|capslock|modeicon|inputstate)'

Push-Location -LiteralPath $RepoRoot
try {
    # -----------------------------------------------------------------------------------------
    # 1. Every package except input_methods/rime, unfiltered.
    # -----------------------------------------------------------------------------------------
    $allPackages = @(go list ./...)
    if ($LASTEXITCODE -ne 0) {
        throw 'go list ./... failed.'
    }
    # -cnotmatch: Go import paths are case-sensitive, so the exclusion must be too.
    $packages = @($allPackages | Where-Object { $_ -cnotmatch '/input_methods/rime$' })
    if ($packages.Count -eq 0) {
        throw 'go list ./... returned no packages.'
    }

    Write-Host "== go test ($($packages.Count) packages, input_methods/rime excluded)"
    go test $packages
    if ($LASTEXITCODE -ne 0) {
        throw 'Backend go test failed.'
    }

    # -----------------------------------------------------------------------------------------
    # 2. The coverage guard on input_methods/rime.
    #    (a) STALENESS  : listed  => exists          (c) COMPLETENESS: discovered => listed
    #    (b) IN-DOMAIN  : listed  => discoverable    => together: discovered == listed, as SETS.
    #    Every name comparison below is CASE-SENSITIVE; see the header. Go test names are.
    # -----------------------------------------------------------------------------------------
    # -CaseSensitive: without it Sort-Object -Unique folds 'TestFoo' and 'Testfoo' together and this
    # check would report a duplicate that is not one (and, worse, teach the reader that these lists
    # are case-insensitive -- they are not).
    # The @(...) around each pipeline is load-bearing: Sort-Object on a ONE-element list returns a
    # scalar, and under Set-StrictMode -Version Latest `.Count` on a scalar string throws
    # "The property 'Count' cannot be found on this object". $KnownUpstreamRed currently has exactly
    # one entry, so this is not hypothetical.
    if (@($FeatureTests | Sort-Object -Unique -CaseSensitive).Count -ne @($FeatureTests).Count) {
        throw 'The run allowlist ($FeatureTests) contains duplicate entries.'
    }
    if (@($KnownUpstreamRed | Sort-Object -Unique -CaseSensitive).Count -ne @($KnownUpstreamRed).Count) {
        throw 'The KNOWN-UPSTREAM-RED denylist ($KnownUpstreamRed) contains duplicate entries.'
    }
    # -ccontains: two tests differing only in case are two DIFFERENT Go tests.
    $overlap = @($FeatureTests | Where-Object { $KnownUpstreamRed -ccontains $_ })
    if ($overlap.Count -gt 0) {
        throw "These tests are in BOTH the run allowlist and the KNOWN-UPSTREAM-RED denylist: $($overlap -join ', ')"
    }

    $listedTests = @($FeatureTests) + @($KnownUpstreamRed)

    # Full test registry of the package, straight from the compiled test binary.
    # -cmatch '^Test': `go test -list` also emits Benchmark*/Fuzz*/Example* entries and a trailing
    # `ok <pkg> <time>` line; only real tests may pass, and only with a capital T.
    $allTests = @(go test -list '.*' $rimePackage | Where-Object { $_ -cmatch '^Test' })
    if ($LASTEXITCODE -ne 0) {
        throw "go test -list failed for $rimePackage."
    }
    if ($allTests.Count -eq 0) {
        throw "go test -list returned no tests for $rimePackage."
    }

    # (a) STALENESS -- both lists may only name tests that still exist.
    $missing = @($FeatureTests | Where-Object { $allTests -cnotcontains $_ })
    if ($missing.Count -gt 0) {
        Stop-WithDiagnostic -Summary 'stale run allowlist -- listed test(s) no longer exist in input_methods/rime' -Detail (
            $missing + @(
                '',
                'They were renamed or deleted. Update $FeatureTests in:',
                "  $PSCommandPath"
            )
        )
    }
    $staleRed = @($KnownUpstreamRed | Where-Object { $allTests -cnotcontains $_ })
    if ($staleRed.Count -gt 0) {
        Stop-WithDiagnostic -Summary 'stale KNOWN-UPSTREAM-RED denylist -- listed test(s) no longer exist in input_methods/rime' -Detail (
            $staleRed + @(
                '',
                'If they were fixed or removed upstream, drop them from $KnownUpstreamRed in:',
                "  $PSCommandPath"
            )
        )
    }

    # Everything $DomainPattern can SEE. Both remaining checks are about this set.
    $domainTests = @(go test -list $DomainPattern $rimePackage | Where-Object { $_ -cmatch '^Test' })
    if ($LASTEXITCODE -ne 0) {
        throw "go test -list failed for $rimePackage (domain pattern)."
    }
    if ($domainTests.Count -eq 0) {
        throw "go test -list matched no feature-domain tests in $rimePackage; the guard would be a no-op."
    }

    # (b) IN-DOMAIN -- nothing we own may sit OUTSIDE what discovery can see.
    #     Reached only after (a), so these tests are known to EXIST: being invisible here therefore
    #     means the NAME is outside the domain vocabulary, not that the test is gone.
    #     Without this check the lists silently become the only thing keeping such a test alive, and
    #     the next test named like it is never discovered and never runs. -cnotcontains: a listed
    #     name that differs from the discovered one only by case is NOT that test.
    $outOfDomain = @($listedTests | Where-Object { $domainTests -cnotcontains $_ })
    if ($outOfDomain.Count -gt 0) {
        Stop-WithDiagnostic -Summary "listed test(s) that `$DomainPattern cannot discover: $($outOfDomain -join ', ')" -Detail (
            $outOfDomain + @(
                '',
                'These tests are in a list, so they RUN -- but the domain pattern',
                "  $DomainPattern",
                'cannot see them. That makes the pattern a lie: a NEW test named in the same',
                'vocabulary would be discovered by nobody, listed by nobody, and would never run,',
                'while CI stayed green. That is the exact hole this guard exists to close.',
                '',
                'Fix it in one of these two ways, in:',
                "  $PSCommandPath",
                '',
                '  * RENAME the test into the domain vocabulary (preferred -- keeps the domain tight), or',
                '  * WIDEN $DomainPattern to cover it, then re-run and confirm the wider pattern does',
                '    not drag in tests this feature does not own.',
                '',
                'Refusing to build: a test the domain cannot discover is a hole in the domain.'
            )
        )
    }

    # (c) COMPLETENESS -- every in-domain test must be classified. A new Shift/asciiMode test that
    #     nobody listed would otherwise simply never run.
    #     -cnotcontains, twice: with the case-INsensitive -notcontains, a test named
    #     TestCapsLockStillForwardedToRIME would be "classified" by the allowlist entry
    #     TestCapsLockStillForwardedToRime -- a DIFFERENT Go test -- and would evade the guard
    #     entirely while the real one kept running in its place.
    $unclassified = @($domainTests | Where-Object {
        ($FeatureTests -cnotcontains $_) -and ($KnownUpstreamRed -cnotcontains $_)
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
                'Names are compared CASE-SENSITIVELY, because Go test names are. If this test looks',
                'like it is already listed, check the capitalisation: it is a different test.',
                '',
                'Refusing to build: an unlisted test is a test that never runs.'
            )
        )
    }

    # (a)+(b)+(c) hold, so the discovered domain and the two lists are now the same SET.
    # Assert it outright -- if this ever fires, one of the three checks above has a bug.
    if ($domainTests.Count -ne $listedTests.Count) {
        throw "Guard invariant broken: $($domainTests.Count) discovered in-domain test(s) but $($listedTests.Count) listed. This is a bug in the guard itself."
    }

    Write-Host "Coverage guard OK: $($domainTests.Count) feature-domain test(s) in input_methods/rime -- $($FeatureTests.Count) allowlisted, $($KnownUpstreamRed.Count) known-upstream-red."

    # -----------------------------------------------------------------------------------------
    # 3. Run the allowlist by exact, fully anchored name.
    # -----------------------------------------------------------------------------------------
    # Fully anchored alternation: it cannot partially match an upstream test name.
    $runPattern = '^(' + ($FeatureTests -join '|') + ')$'

    $selected = @(go test -list $runPattern $rimePackage | Where-Object { $_ -cmatch '^Test' })
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
