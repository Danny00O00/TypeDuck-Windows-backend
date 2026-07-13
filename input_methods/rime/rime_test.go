package rime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

func TestMain(m *testing.M) {
	tempAppData, err := os.MkdirTemp("", "moqi-ime-rime-test-*")
	if err == nil {
		_ = os.Setenv("APPDATA", tempAppData)
	}
	code := m.Run()
	if err == nil {
		_ = os.RemoveAll(tempAppData)
	}
	os.Exit(code)
}

func writeTestAIConfig(t *testing.T, appData string, body string) {
	t.Helper()
	configPath := filepath.Join(appData, APP, aiConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create AI config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write AI config: %v", err)
	}
}

func writeTestSchemeSetConfig(t *testing.T, appData, current string) {
	t.Helper()
	configPath := filepath.Join(appData, APP, schemeSetConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create scheme set config dir: %v", err)
	}
	body, err := json.MarshalIndent(schemeSetConfig{Current: current}, "", "  ")
	if err != nil {
		t.Fatalf("marshal scheme set config: %v", err)
	}
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		t.Fatalf("write scheme set config: %v", err)
	}
}

func writeTestCustomPhraseFile(t *testing.T, appData, body string) string {
	t.Helper()
	configPath := filepath.Join(appData, APP, customPhraseFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create custom phrase dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write custom phrase file: %v", err)
	}
	return configPath
}

func writeTestSuperAbbrevFile(t *testing.T, appData, body string) string {
	t.Helper()
	configPath := filepath.Join(appData, APP, superAbbrevFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create super abbrev dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write super abbrev file: %v", err)
	}
	return configPath
}

type testDictEntry struct {
	code  string
	words []candidateItem
}

type testBackend struct {
	session              bool
	destroySessionCalls  int
	composition          string
	rawInput             string
	pageNo               int
	pageSize             int
	isLastPage           bool
	candidateCursor      int
	candidates           []candidateItem
	selectKeys           string
	commitString         string
	asciiMode            bool
	fullShape            bool
	options              map[string]bool
	saveOptions          []string
	schemaSwitches       map[string][]RimeSwitch
	schemas              []RimeSchema
	currentSchemaID      string
	selectSchemaCalls    []string
	pageSizeCalls        []int
	pageSizeOK           bool
	redeployCalls        int
	redeploySharedDir    string
	redeployUserDir      string
	redeployOK           bool
	syncCalls            int
	syncOK               bool
	setOptionCalls       int
	getOptionCalls       int
	toggleASCIIOnCtrlA   bool
	disableArrowHandling bool
	translatedKeyCodes   []int
}

func newTestBackend() *testBackend {
	return &testBackend{
		redeployOK: true,
		syncOK:     true,
		selectKeys: "1234567890",
		schemas: []RimeSchema{
			{ID: "rime_frost", Name: "白霜拼音"},
			{ID: "rime_frost_double_pinyin", Name: "自然码双拼"},
			{ID: "rime_frost_double_pinyin_flypy", Name: "小鹤双拼"},
		},
		options: map[string]bool{
			"ascii_mode":         false,
			"full_shape":         false,
			"ascii_punct":        false,
			"traditionalization": false,
		},
		saveOptions: []string{"ascii_mode", "traditionalization", "ascii_punct", "full_shape"},
		schemaSwitches: map[string][]RimeSwitch{
			"rime_frost": {
				{Name: "ascii_mode", States: []string{"中文", "西文"}},
				{Name: "traditionalization", States: []string{"简体", "繁体"}},
				{Name: "ascii_punct", States: []string{"中文标点", "英文标点"}},
				{Name: "full_shape", States: []string{"半角", "全角"}},
			},
			"rime_frost_double_pinyin": {
				{Name: "ascii_mode", States: []string{"中文", "西文"}},
				{Name: "traditionalization", States: []string{"简体", "繁体"}},
				{Name: "ascii_punct", States: []string{"中文标点", "英文标点"}},
				{Name: "full_shape", States: []string{"半角", "全角"}},
			},
			"rime_frost_double_pinyin_flypy": {
				{Name: "ascii_mode", States: []string{"中文", "西文"}},
				{Name: "traditionalization", States: []string{"简体", "繁体"}},
				{Name: "ascii_punct", States: []string{"中文标点", "英文标点"}},
				{Name: "full_shape", States: []string{"半角", "全角"}},
			},
		},
		currentSchemaID: "rime_frost",
		pageSize:        6,
		pageSizeOK:      false,
	}
}

func (b *testBackend) Initialize(sharedDir, userDir string, firstRun bool) bool {
	return true
}

func (b *testBackend) Redeploy(sharedDir, userDir string) bool {
	b.redeployCalls++
	b.redeploySharedDir = sharedDir
	b.redeployUserDir = userDir
	b.DestroySession()
	return b.redeployOK
}

func (b *testBackend) SyncUserData() bool {
	b.syncCalls++
	return b.syncOK
}

func (b *testBackend) HasSession() bool {
	return b.session
}

func (b *testBackend) EnsureSession() bool {
	b.session = true
	return true
}

func (b *testBackend) DestroySession() {
	b.destroySessionCalls++
	b.session = false
	b.ClearComposition()
}

func (b *testBackend) ClearComposition() {
	b.composition = ""
	b.rawInput = ""
	b.candidates = nil
	b.candidateCursor = 0
	b.commitString = ""
}

func (b *testBackend) ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool {
	b.translatedKeyCodes = append(b.translatedKeyCodes, translatedKeyCode)
	b.commitString = ""
	keyCode := req.KeyCode
	charCode := req.CharCode
	if charCode == 0 && keyCode >= 'A' && keyCode <= 'Z' {
		charCode = keyCode + 32
	}
	if keyCode == vkCapital && modifiers&releaseMask == 0 {
		b.SetOption("ascii_mode", !b.GetOption("ascii_mode"))
		return true
	}
	if b.toggleASCIIOnCtrlA && keyCode == 'A' && charCode == 1 {
		b.SetOption("ascii_mode", !b.GetOption("ascii_mode"))
		return false
	}
	if b.asciiMode && b.composition == "" && charCode >= 0x20 {
		return false
	}
	if modifiers&releaseMask != 0 {
		return false
	}

	switch keyCode {
	case vkBack:
		if b.composition == "" {
			return false
		}
		b.composition = trimLastRuneForTest(b.composition)
		b.refreshCandidates()
		return true
	case vkEscape:
		if b.composition == "" {
			return false
		}
		b.ClearComposition()
		return true
	case vkReturn, vkSpace:
		if b.composition == "" {
			return false
		}
		b.commitString = b.currentCommit()
		b.composition = ""
		b.candidates = nil
		b.candidateCursor = 0
		return true
	case vkUp:
		if len(b.candidates) == 0 {
			return false
		}
		if b.disableArrowHandling {
			return false
		}
		if b.candidateCursor > 0 {
			b.candidateCursor--
		}
		return true
	case vkDown:
		if len(b.candidates) == 0 {
			return false
		}
		if b.disableArrowHandling {
			return false
		}
		if b.candidateCursor < len(b.candidates)-1 {
			b.candidateCursor++
		}
		return true
	}

	if b.composition != "" && keyCode >= '1' && keyCode <= '9' {
		index := keyCode - '1'
		if index >= 0 && index < len(b.candidates) {
			b.commitString = b.candidates[index].Text
			b.composition = ""
			b.candidates = nil
			b.candidateCursor = 0
			return true
		}
	}

	if charCode >= 'a' && charCode <= 'z' {
		b.composition += string(rune(charCode))
		b.refreshCandidates()
		return true
	}
	if charCode == '\'' && b.composition != "" && !strings.HasSuffix(b.composition, "'") {
		b.composition += "'"
		b.refreshCandidates()
		return true
	}
	if b.composition != "" && charCode >= 0x20 && charCode != '\'' {
		b.commitString = b.currentCommit() + string(rune(charCode))
		b.composition = ""
		b.candidates = nil
		return true
	}
	return false
}

func (b *testBackend) State() rimeState {
	state := rimeState{
		CommitString:    b.commitString,
		Composition:     b.composition,
		RawInput:        b.rawInput,
		PageNo:          b.pageNo,
		PageSize:        b.pageSize,
		IsLastPage:      b.isLastPage,
		CursorPos:       len(b.composition),
		Candidates:      append([]candidateItem(nil), b.candidates...),
		CandidateCursor: b.candidateCursor,
		SelectKeys:      b.selectKeys,
		AsciiMode:       b.asciiMode,
		FullShape:       b.fullShape,
	}
	if state.RawInput == "" {
		state.RawInput = b.composition
	}
	b.commitString = ""
	return state
}

func (b *testBackend) SetOption(name string, value bool) {
	b.setOptionCalls++
	if b.options == nil {
		b.options = map[string]bool{}
	}
	b.options[name] = value
	switch name {
	case "ascii_mode":
		b.asciiMode = value
	case "full_shape":
		b.fullShape = value
	}
}

func (b *testBackend) GetOption(name string) bool {
	b.getOptionCalls++
	if b.options != nil {
		if value, ok := b.options[name]; ok {
			return value
		}
	}
	switch name {
	case "ascii_mode":
		return b.asciiMode
	case "full_shape":
		return b.fullShape
	default:
		return false
	}
}

func (b *testBackend) SaveOptions() []string {
	return append([]string(nil), b.saveOptions...)
}

func (b *testBackend) SchemaSwitches() []RimeSwitch {
	return append([]RimeSwitch(nil), b.schemaSwitches[b.currentSchemaID]...)
}

func (b *testBackend) SchemaList() []RimeSchema {
	return append([]RimeSchema(nil), b.schemas...)
}

func (b *testBackend) CurrentSchemaID() string {
	return b.currentSchemaID
}

func (b *testBackend) SelectSchema(schemaID string) bool {
	for _, schema := range b.schemas {
		if schema.ID != schemaID {
			continue
		}
		b.currentSchemaID = schemaID
		b.selectSchemaCalls = append(b.selectSchemaCalls, schemaID)
		return true
	}
	return false
}

func (b *testBackend) SetCandidatePageSize(pageSize int) bool {
	b.pageSizeCalls = append(b.pageSizeCalls, pageSize)
	return b.pageSizeOK
}

func (b *testBackend) SelectCandidate(index int) bool {
	if index < 0 || index >= len(b.candidates) {
		return false
	}
	b.candidateCursor = index
	b.commitString = b.candidates[index].Text
	b.composition = ""
	b.candidates = nil
	b.candidateCursor = 0
	return true
}

func (b *testBackend) HighlightCandidate(index int) bool {
	if index < 0 || index >= len(b.candidates) {
		return false
	}
	b.candidateCursor = index
	return true
}

func (b *testBackend) DeleteCandidateOnCurrentPage(index int) bool {
	return false
}

func (b *testBackend) ChangePage(backward bool) bool {
	if backward {
		if b.pageNo == 0 {
			return false
		}
		b.pageNo--
	} else {
		b.pageNo++
	}
	b.candidateCursor = 0
	return true
}

func (b *testBackend) currentCommit() string {
	if len(b.candidates) > 0 {
		return b.candidates[0].Text
	}
	return strings.ReplaceAll(b.composition, "'", "")
}

func (b *testBackend) refreshCandidates() {
	code := strings.ReplaceAll(b.composition, "'", "")
	if code == "" {
		b.candidates = nil
		b.candidateCursor = 0
		return
	}
	results := make([]candidateItem, 0, 9)
	seen := make(map[string]struct{})
	appendWords := func(words []candidateItem) {
		for _, word := range words {
			if _, ok := seen[word.Text]; ok {
				continue
			}
			seen[word.Text] = struct{}{}
			results = append(results, word)
			if len(results) == 9 {
				return
			}
		}
	}
	for _, entry := range testDictionary() {
		if entry.code == code {
			appendWords(entry.words)
		}
	}
	for _, entry := range testDictionary() {
		if len(results) == 9 {
			break
		}
		if entry.code != code && strings.HasPrefix(entry.code, code) {
			appendWords(entry.words)
		}
	}
	if len(results) == 0 {
		results = []candidateItem{{Text: code}}
	}
	b.candidates = results
	if b.candidateCursor < 0 {
		b.candidateCursor = 0
	}
	if len(b.candidates) == 0 {
		b.candidateCursor = 0
	} else if b.candidateCursor >= len(b.candidates) {
		b.candidateCursor = len(b.candidates) - 1
	}
}

func testDictionary() []testDictEntry {
	return []testDictEntry{
		{code: "ni", words: []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}, {Text: "尼"}, {Text: "拟"}}},
		{code: "nihao", words: []candidateItem{{Text: "你好"}, {Text: "你号"}, {Text: "拟好"}}},
		{code: "nihaoma", words: []candidateItem{{Text: "你好吗"}, {Text: "尼玛"}}},
		{code: "nimen", words: []candidateItem{{Text: "你们"}}},
		{code: "zhong", words: []candidateItem{{Text: "中"}, {Text: "种"}, {Text: "重"}}},
		{code: "zhongwen", words: []candidateItem{{Text: "中文"}}},
	}
}

func trimLastRuneForTest(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	return string(runes[:len(runes)-1])
}

func newTestIME() *IME {
	return &IME{
		TextServiceBase:  imecore.NewTextServiceBase(&imecore.Client{ID: "test-client"}),
		style:            defaultStyle(),
		backend:          newTestBackend(),
		schemeSetVersion: currentSchemeSetVersion(),
	}
}

func newIsolatedTestIME(t *testing.T) *IME {
	t.Helper()
	resetSharedAppearanceConfigForTest()
	resetSchemeSetVersionForTest()
	return newTestIME()
}

func shiftKeyRequest(seq int, ctrl, alt bool) *imecore.Request {
	states := make(imecore.KeyStates, 256)
	if ctrl {
		states[vkControl] |= 0x80
	}
	if alt {
		states[vkMenu] |= 0x80
	}
	return &imecore.Request{SeqNum: seq, KeyCode: vkShift, KeyStates: states}
}

func typeTestLetters(t *testing.T, ime *IME, seq int, letters string) {
	t.Helper()
	for i, ch := range letters {
		resp := ime.filterKeyDown(&imecore.Request{
			SeqNum:    seq + i,
			KeyCode:   int(ch) - 32,
			CharCode:  int(ch),
			KeyStates: make(imecore.KeyStates, 256),
		}, imecore.NewResponse(seq+i, true))
		if resp.ReturnValue != 1 {
			t.Fatalf("expected letter %q handled, got %d", string(ch), resp.ReturnValue)
		}
	}
}

func TestNewInitialState(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	if !ime.style.DisplayTrayIcon {
		t.Fatal("expected tray icon style enabled by default")
	}
	if backend.composition != "" {
		t.Fatalf("expected empty composition, got %q", backend.composition)
	}
	if len(backend.candidates) != 0 {
		t.Fatalf("expected no candidates, got %v", backend.candidates)
	}
	if ime.style.CandidatePerRow != 1 {
		t.Fatalf("expected vertical layout by default, got CandidatePerRow=%d", ime.style.CandidatePerRow)
	}
	if ime.style.CandidateTheme != "default" || ime.style.FontPoint != 16 || ime.style.CandidateCommentFontPoint != 14 {
		t.Fatalf("expected default theme defaults, got theme=%q font=%d commentFont=%d",
			ime.style.CandidateTheme, ime.style.FontPoint, ime.style.CandidateCommentFontPoint)
	}
	if ime.style.CandidateCommentColor != ime.style.CandidateTextColor || ime.style.CandidateCommentHighlightColor != ime.style.CandidateHighlightTextColor {
		t.Fatalf("expected default comment colors to follow text colors, got comment=%q highlight=%q text=%q hltext=%q",
			ime.style.CandidateCommentColor, ime.style.CandidateCommentHighlightColor, ime.style.CandidateTextColor, ime.style.CandidateHighlightTextColor)
	}
	if ime.style.CandidateBackgroundColor != "#ffffff" || ime.style.CandidateHighlightColor != "#c6ddf9" {
		t.Fatalf("expected default theme colors, got bg=%q hl=%q",
			ime.style.CandidateBackgroundColor, ime.style.CandidateHighlightColor)
	}
	if ime.style.CandidateSpacing != 20 {
		t.Fatalf("expected default candidate spacing 20, got %d", ime.style.CandidateSpacing)
	}
	if ime.keyComposing {
		t.Fatal("expected keyComposing to be false initially")
	}
}

func TestFilterKeyDownProcessesKeyWithoutUpdatingUI(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   1,
		KeyCode:  0x4E,
		CharCode: 'n',
	}, imecore.NewResponse(1, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected n to be handled, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "" || len(resp.CandidateList) != 0 || resp.ShowCandidates {
		t.Fatalf("expected filterKeyDown not to emit UI state, got %#v", resp)
	}
}

func TestFilterKeyDownFallsBackToKeyCodeWhenCharCodeMissing(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  2,
		KeyCode: 0x4E,
	}, imecore.NewResponse(2, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected keyCode-only N to be handled, got %d", resp.ReturnValue)
	}
}

func TestFilterKeyDownEmitsLangButtonUpdateWhenControlHotkeyTogglesAsciiMode(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.toggleASCIIOnCtrlA = true

	iconDir := t.TempDir()
	for _, name := range []string{"chi.ico", "eng.ico"} {
		if err := os.WriteFile(filepath.Join(iconDir, name), []byte{}, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	ime.iconDir = iconDir

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   3,
		KeyCode:  int('A'),
		CharCode: 1,
	}, imecore.NewResponse(3, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected ctrl+a hotkey to remain unhandled, got %d", resp.ReturnValue)
	}
	if len(resp.ChangeButton) != 1 {
		t.Fatalf("expected one language button update, got %#v", resp.ChangeButton)
	}
	if resp.ChangeButton[0].ID != "switch-lang" {
		t.Fatalf("expected switch-lang button update, got %#v", resp.ChangeButton)
	}
	if got := filepath.Base(resp.ChangeButton[0].Icon); got != "eng.ico" {
		t.Fatalf("expected eng.ico update, got %q", got)
	}
}

func TestOnKeyDownReflectsBackendStateAfterFilter(t *testing.T) {
	ime := newIsolatedTestIME(t)

	ime.filterKeyDown(&imecore.Request{
		SeqNum:   1,
		KeyCode:  0x4E,
		CharCode: 'n',
	}, imecore.NewResponse(1, true))
	ime.filterKeyDown(&imecore.Request{
		SeqNum:   2,
		KeyCode:  0x49,
		CharCode: 'i',
	}, imecore.NewResponse(2, true))

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   3,
		KeyCode:  0x49,
		CharCode: 'i',
	}, imecore.NewResponse(3, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onKeyDown to succeed, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "ni" {
		t.Fatalf("expected composition ni, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) == 0 || resp.CandidateList[0] != "你" {
		t.Fatalf("expected first exact candidate 你, got %v", resp.CandidateList)
	}
}

func TestMobileReplayTextUsesSelectedPinyinVerbatim(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.MobileReplayText("ni'hao'ma", 1)
	if !resp.Success || resp.ReturnValue != 1 {
		t.Fatalf("expected replay to succeed, got %#v", resp)
	}
	if resp.CompositionString != "ni'hao'ma" {
		t.Fatalf("expected composition ni'hao'ma, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) == 0 || resp.CandidateList[0] != "你好吗" {
		t.Fatalf("expected first candidate 你好吗, got %#v", resp.CandidateList)
	}
}

func TestMobileReplayTextSendsApostropheAsPrintableSeparator(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.MobileReplayText("ni'hao", 1)
	if !resp.Success || resp.ReturnValue != 1 {
		t.Fatalf("expected replay to succeed, got %#v", resp)
	}
	if len(backend.translatedKeyCodes) < 3 {
		t.Fatalf("expected translated keys to be recorded, got %v", backend.translatedKeyCodes)
	}
	if got := backend.translatedKeyCodes[2]; got != int('\'') {
		t.Fatalf("expected apostrophe translated as printable separator %d, got %d (%v)", int('\''), got, backend.translatedKeyCodes)
	}
}

func TestOnKeyDownDownArrowMovesCandidateCursor(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  1,
		KeyCode: vkDown,
	}, imecore.NewResponse(1, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected filterKeyDown down arrow handled, got %#v", filterResp)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  2,
		KeyCode: vkDown,
	}, imecore.NewResponse(2, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected onKeyDown down arrow handled, got %#v", resp)
	}
	if !resp.HasCandidateCursor || resp.CandidateCursor != 1 {
		t.Fatalf("expected candidate cursor 1, got %#v", resp)
	}
}

func TestOnKeyDownDownArrowFallsBackToExplicitHighlight(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	backend.disableArrowHandling = true
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  1,
		KeyCode: vkDown,
	}, imecore.NewResponse(1, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected filterKeyDown down arrow handled by fallback, got %#v", filterResp)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  2,
		KeyCode: vkDown,
	}, imecore.NewResponse(2, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected onKeyDown down arrow handled by fallback, got %#v", resp)
	}
	if !resp.HasCandidateCursor || resp.CandidateCursor != 1 {
		t.Fatalf("expected candidate cursor 1 after fallback, got %#v", resp)
	}
}

func TestHighlightCandidateRequestUpdatesVisibleCursor(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	ime.keyComposing = true

	resp := ime.HandleRequest(&imecore.Request{
		Method:            "highlightCandidate",
		SeqNum:            1,
		CandidateIndex:    2,
		HasCandidateIndex: true,
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected highlightCandidate handled, got %#v", resp)
	}
	if !resp.HasCandidateCursor || resp.CandidateCursor != 2 {
		t.Fatalf("expected candidate cursor 2, got %#v", resp)
	}
}

func TestSelectCandidateRequestCommitsCandidate(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	ime.keyComposing = true

	resp := ime.HandleRequest(&imecore.Request{
		Method:            "selectCandidate",
		SeqNum:            1,
		CandidateIndex:    1,
		HasCandidateIndex: true,
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected selectCandidate handled, got %#v", resp)
	}
	if resp.CommitString != "呢" {
		t.Fatalf("expected commit 呢, got %#v", resp)
	}
	if resp.ShowCandidates {
		t.Fatalf("expected candidates hidden after selection, got %#v", resp)
	}
}

func TestChangePageRequestDelegatesToBackend(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}}
	ime.keyComposing = true

	resp := ime.HandleRequest(&imecore.Request{
		Method:       "changePage",
		SeqNum:       1,
		PageBackward: false,
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected changePage handled, got %#v", resp)
	}
	if backend.pageNo != 1 {
		t.Fatalf("expected pageNo 1, got %d", backend.pageNo)
	}
	if !resp.HasCandidateCursor || resp.CandidateCursor != 0 {
		t.Fatalf("expected candidate cursor reset to 0, got %#v", resp)
	}
}

func TestOnKeyDownNumberSelectsCandidate(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  4,
		KeyCode: 0x32,
	}, imecore.NewResponse(4, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected number selection to be handled, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  5,
		KeyCode: 0x32,
	}, imecore.NewResponse(5, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onKeyDown after selection to succeed, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "呢" {
		t.Fatalf("expected second candidate 呢, got %q", resp.CommitString)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected state reset after candidate selection")
	}
}

func TestOnKeyDownSemicolonSelectsSecondCandidate(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   41,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(41, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon selection to be handled, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   42,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(42, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon onKeyDown to succeed, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "呢" {
		t.Fatalf("expected semicolon to commit second candidate 呢, got %q", resp.CommitString)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected state reset after semicolon selection")
	}
}

func TestOnKeyDownSemicolonSelectsSecondCandidateWhenKeyEncodingDiffersBetweenStages(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "n"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "那么"}, {Text: "能不能"}, {Text: "哪里"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   43,
		KeyCode:  vkOem1,
		CharCode: 0,
	}, imecore.NewResponse(43, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon filterKeyDown handled with OEM keycode, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   44,
		KeyCode:  0,
		CharCode: int(';'),
	}, imecore.NewResponse(44, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon onKeyDown handled with charcode-only event, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "那么" {
		t.Fatalf("expected semicolon to commit second visible candidate 那么, got %q", resp.CommitString)
	}
}

func TestOnKeyDownSemicolonSelectsSecondCandidateWhenOEMKeyHasUnexpectedCharCode(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "n"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "那么"}, {Text: "能不能"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   45,
		KeyCode:  vkOem1,
		CharCode: int('；'),
	}, imecore.NewResponse(45, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon filterKeyDown handled even with unexpected charCode, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   46,
		KeyCode:  vkOem1,
		CharCode: int('；'),
	}, imecore.NewResponse(46, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon onKeyDown handled even with unexpected charCode, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "那么" {
		t.Fatalf("expected semicolon to commit second visible candidate 那么, got %q", resp.CommitString)
	}
}

func TestOnKeyDownBackspaceUpdatesComposition(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:  5,
		KeyCode: 0x08,
	}, imecore.NewResponse(5, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  6,
		KeyCode: 0x08,
	}, imecore.NewResponse(6, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected backspace to be handled, got %d", resp.ReturnValue)
	}
	if backend.composition != "n" {
		t.Fatalf("expected composition n after backspace, got %q", backend.composition)
	}
	if resp.CompositionString != "n" {
		t.Fatalf("expected response composition n, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) == 0 {
		t.Fatal("expected candidates to remain after backspace")
	}
}

func TestOnKeyDownEscapeClearsComposition(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:  6,
		KeyCode: 0x1B,
	}, imecore.NewResponse(6, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  7,
		KeyCode: 0x1B,
	}, imecore.NewResponse(7, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected escape to be handled, got %d", resp.ReturnValue)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected composition state cleared")
	}
	if resp.CompositionString != "" || resp.ShowCandidates {
		t.Fatalf("expected cleared UI, got %#v", resp)
	}
}

func TestOnKeyDownSpaceCommitsFirstCandidate(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:  7,
		KeyCode: 0x20,
	}, imecore.NewResponse(7, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  8,
		KeyCode: 0x20,
	}, imecore.NewResponse(8, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected space to be handled, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "你" {
		t.Fatalf("expected first candidate 你, got %q", resp.CommitString)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected state reset after commit")
	}
}

func TestOnKeyDownPunctuationCommitsComposition(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:   8,
		KeyCode:  int('.'),
		CharCode: int('.'),
	}, imecore.NewResponse(8, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   9,
		KeyCode:  int('.'),
		CharCode: int('.'),
	}, imecore.NewResponse(9, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected punctuation to be handled while composing, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "你." {
		t.Fatalf("expected punctuation commit 你., got %q", resp.CommitString)
	}
}

func TestOnKeyDownUnhandledKeyReturnsZero(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   9,
		KeyCode:  0x70,
		CharCode: 0,
	}, imecore.NewResponse(9, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected unrelated key to be ignored, got %d", resp.ReturnValue)
	}
}

func TestOnKeyDownAsciiModePassesThroughWhenIdle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.backend.SetOption("ascii_mode", true)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   10,
		KeyCode:  int('A'),
		CharCode: int('a'),
	}, imecore.NewResponse(10, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected ascii mode to pass through idle typing, got %d", resp.ReturnValue)
	}
}

func TestControlKeyPassesThroughWhenIdle(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  10,
		KeyCode: vkControl,
	}, imecore.NewResponse(10, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected bare ctrl to pass through, got %d", resp.ReturnValue)
	}
}

// Regression: if filterKeyDown does not handle a bare Ctrl key, onKeyDown must return
// unhandled as well; otherwise the host still thinks the IME consumed the modifier.
func TestOnKeyDownBareControlUnhandledWhenFilterDoesNotHandle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	const seq = 20
	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  seq,
		KeyCode: vkControl,
	}, imecore.NewResponse(seq, true))
	if filterResp.ReturnValue != 0 {
		t.Fatalf("expected filterKeyDown bare Ctrl unhandled, got %d", filterResp.ReturnValue)
	}
	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  seq + 1,
		KeyCode: vkControl,
	}, imecore.NewResponse(seq+1, true))
	if onResp.ReturnValue != 0 {
		t.Fatalf("expected onKeyDown bare Ctrl unhandled when filter did not handle, got %d", onResp.ReturnValue)
	}
}

func TestOnKeyDownControlShortcutUnhandledWhenFilterDoesNotHandle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	const seq = 22
	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   seq,
		KeyCode:  int('A'),
		CharCode: 1,
	}, imecore.NewResponse(seq, true))
	if filterResp.ReturnValue != 0 {
		t.Fatalf("expected filterKeyDown ctrl+a unhandled, got %d", filterResp.ReturnValue)
	}
	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   seq + 1,
		KeyCode:  int('A'),
		CharCode: 1,
	}, imecore.NewResponse(seq+1, true))
	if onResp.ReturnValue != 0 {
		t.Fatalf("expected onKeyDown ctrl+a unhandled when filter did not handle, got %d", onResp.ReturnValue)
	}
}

// Regression: same contract as TestOnKeyDownBareControlUnhandledWhenFilterDoesNotHandle for key-up / Alt.
func TestOnKeyUpBareAltUnhandledWhenFilterDoesNotHandle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	const seq = 21
	filterResp := ime.filterKeyUp(&imecore.Request{
		SeqNum:  seq,
		KeyCode: vkMenu,
	}, imecore.NewResponse(seq, true))
	if filterResp.ReturnValue != 0 {
		t.Fatalf("expected filterKeyUp bare Alt unhandled, got %d", filterResp.ReturnValue)
	}
	onResp := ime.onKeyUp(&imecore.Request{
		SeqNum:  seq + 1,
		KeyCode: vkMenu,
	}, imecore.NewResponse(seq+1, true))
	if onResp.ReturnValue != 0 {
		t.Fatalf("expected onKeyUp bare Alt unhandled when filter did not handle, got %d", onResp.ReturnValue)
	}
}

func TestOnCommandHandlesKnownAndMissingCommand(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	validResp := ime.onCommand(&imecore.Request{
		SeqNum: 11,
		ID:     imecore.FlexibleID{Int: ID_ASCII_MODE, IsInt: true},
	}, imecore.NewResponse(11, true))
	if validResp.ReturnValue != 1 {
		t.Fatalf("expected known command to be handled, got %d", validResp.ReturnValue)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii mode toggled on")
	}
	if backend.composition != "ni" {
		t.Fatalf("expected test composition preserved until backend handles key flow, got %q", backend.composition)
	}

	missingResp := ime.onCommand(&imecore.Request{
		SeqNum: 12,
	}, imecore.NewResponse(12, true))
	if missingResp.ReturnValue != 0 {
		t.Fatalf("expected missing commandId to be ignored, got %d", missingResp.ReturnValue)
	}
}

func TestOnCommandDeployRedeploysBackend(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 13,
		ID:     imecore.FlexibleID{Int: ID_DEPLOY, IsInt: true},
	}, imecore.NewResponse(13, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected deploy command to succeed, got %d", resp.ReturnValue)
	}
	if backend.redeployCalls != 1 {
		t.Fatalf("expected backend redeployed once, got %d", backend.redeployCalls)
	}
	if backend.redeploySharedDir == "" || backend.redeployUserDir == "" {
		t.Fatalf("expected redeploy paths to be populated, got shared=%q user=%q", backend.redeploySharedDir, backend.redeployUserDir)
	}
	if !backend.session {
		t.Fatal("expected session recreated after redeploy")
	}
	if resp.TrayNotification == nil {
		t.Fatal("expected deploy success tray notification")
	}
	if resp.TrayNotification.Title != "Rime" || resp.TrayNotification.Message != "Redeploy succeeded" {
		t.Fatalf("unexpected deploy success notification: %#v", resp.TrayNotification)
	}
	if resp.TrayNotification.Icon != imecore.TrayNotificationIconInfo {
		t.Fatalf("expected info tray notification, got %q", resp.TrayNotification.Icon)
	}
}

func TestOnCommandDeployFailureReturnsErrorTrayNotification(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.redeployOK = false

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 21,
		ID:     imecore.FlexibleID{Int: ID_DEPLOY, IsInt: true},
	}, imecore.NewResponse(21, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected deploy command to fail, got %d", resp.ReturnValue)
	}
	if backend.redeployCalls != 1 {
		t.Fatalf("expected backend redeployed once, got %d", backend.redeployCalls)
	}
	if resp.TrayNotification == nil {
		t.Fatal("expected deploy failure tray notification")
	}
	if resp.TrayNotification.Title != "Rime" || resp.TrayNotification.Message != "Redeploy failed" {
		t.Fatalf("unexpected deploy failure notification: %#v", resp.TrayNotification)
	}
	if resp.TrayNotification.Icon != imecore.TrayNotificationIconError {
		t.Fatalf("expected error tray notification, got %q", resp.TrayNotification.Icon)
	}
}

func TestOnCommandDeployReloadsAIConfig(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	writeTestAIConfig(t, appData, `{
  "api": {
    "base_url": "https://example.com/v1",
    "api_key": "test-key",
    "model": "test-model"
  },
  "actions": [
    {
      "name": "AI 改写",
      "hotkey": "Ctrl+Alt+K",
      "prompt": "请改写 {{composition}}"
    }
  ]
}`)

	ime := newIsolatedTestIME(t)
	ime.aiEnabled = false
	ime.aiActions = nil
	ime.aiReviewGenerator = nil

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 22,
		ID:     imecore.FlexibleID{Int: ID_DEPLOY, IsInt: true},
	}, imecore.NewResponse(22, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected deploy command to succeed, got %d", resp.ReturnValue)
	}
	if !ime.aiEnabled {
		t.Fatal("expected AI to be enabled after reloading ai_config.json")
	}
	if ime.aiReviewGenerator == nil {
		t.Fatal("expected AI review generator reloaded")
	}
	if len(ime.aiActions) != 1 {
		t.Fatalf("expected 1 AI action after reload, got %#v", ime.aiActions)
	}
	if ime.aiActions[0].Name != "AI 改写" || ime.aiActions[0].Hotkey != "Ctrl+Alt+K" {
		t.Fatalf("unexpected AI action after reload: %#v", ime.aiActions[0])
	}
}

func TestOnCommandDeployIgnoresInvalidAIConfig(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	writeTestAIConfig(t, appData, `{
  "api": {
    "base_url": "https://example.com/v1"
  }
}`)

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 23,
		ID:     imecore.FlexibleID{Int: ID_DEPLOY, IsInt: true},
	}, imecore.NewResponse(23, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected deploy command to succeed even when ai_config.json is invalid, got %d", resp.ReturnValue)
	}
	if backend.redeployCalls != 1 {
		t.Fatalf("expected backend redeployed once, got %d", backend.redeployCalls)
	}
	if resp.TrayNotification == nil {
		t.Fatal("expected deploy success tray notification")
	}
	if resp.TrayNotification.Title != "Rime" || resp.TrayNotification.Message != "Redeploy succeeded" {
		t.Fatalf("unexpected deploy success notification: %#v", resp.TrayNotification)
	}
	if resp.TrayNotification.Icon != imecore.TrayNotificationIconInfo {
		t.Fatalf("expected info tray notification, got %q", resp.TrayNotification.Icon)
	}
}

func TestOnCommandSyncUserData(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 14,
		ID:     imecore.FlexibleID{Int: ID_SYNC, IsInt: true},
	}, imecore.NewResponse(14, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected sync command to succeed, got %d", resp.ReturnValue)
	}
	if backend.syncCalls != 1 {
		t.Fatalf("expected sync_user_data called once, got %d", backend.syncCalls)
	}
}

func TestOnCommandSelectSchema(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 15,
		ID:     imecore.FlexibleID{Int: schemaCommandID(1), IsInt: true},
	}, imecore.NewResponse(15, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected schema command to succeed, got %d", resp.ReturnValue)
	}
	if backend.currentSchemaID != "rime_frost_double_pinyin" {
		t.Fatalf("expected schema switched to natural double pinyin, got %q", backend.currentSchemaID)
	}
	if len(backend.selectSchemaCalls) != 1 || backend.selectSchemaCalls[0] != "rime_frost_double_pinyin" {
		t.Fatalf("expected select schema call recorded, got %#v", backend.selectSchemaCalls)
	}
}

func TestOnMenuDoesNotExposeLegacySettingsMenu(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.onMenu(&imecore.Request{
		SeqNum: 16,
		ID:     imecore.FlexibleID{String: "settings"},
	}, imecore.NewResponse(16, true))

	items, ok := resp.ReturnData.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected empty settings menu payload, got %#v", resp.ReturnData)
	}
	if resp.ReturnValue != 0 || len(items) != 0 {
		t.Fatalf("onMenu should be unreachable in TypeDuck v1, ReturnValue=%d items=%#v", resp.ReturnValue, items)
	}
}

func TestBuildMenuIncludesHelpLinks(t *testing.T) {
	ime := newIsolatedTestIME(t)

	items := ime.buildMenu()
	want := map[string]int{
		"說明文件(&H) / Help":    ID_HELP_DOCS,
		"參與討論(&J) / Discuss": ID_DISCUSSIONS,
	}
	for text, id := range want {
		found := false
		for _, item := range items {
			if item["text"] == text && item["id"] == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected menu item %q with id %d, got %#v", text, id, items)
		}
	}
}

func TestOnCommandOpensHelpLinks(t *testing.T) {
	ime := newIsolatedTestIME(t)
	oldOpenURLFunc := openURLFunc
	var opened []string
	openURLFunc = func(target string) error {
		opened = append(opened, target)
		return nil
	}
	defer func() {
		openURLFunc = oldOpenURLFunc
	}()

	for _, commandID := range []int{ID_HELP_DOCS, ID_DISCUSSIONS} {
		resp := ime.onCommand(&imecore.Request{
			SeqNum: 17,
			ID:     imecore.FlexibleID{Int: commandID, IsInt: true},
		}, imecore.NewResponse(17, true))
		if resp.ReturnValue != 1 {
			t.Fatalf("expected command %d to succeed, got %d", commandID, resp.ReturnValue)
		}
	}

	want := []string{helpDocsURL, discussionURL}
	if len(opened) != len(want) {
		t.Fatalf("expected opened urls %#v, got %#v", want, opened)
	}
	for i := range want {
		if opened[i] != want[i] {
			t.Fatalf("expected opened urls %#v, got %#v", want, opened)
		}
	}
}

func TestBuildMenuIncludesSchemaSubmenu(t *testing.T) {
	ime := newIsolatedTestIME(t)

	items := ime.buildMenu()
	var schemaMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "輸入方案(&I) / Input Schema" {
			schemaMenu = item
			break
		}
	}
	if schemaMenu == nil {
		t.Fatalf("expected schema submenu in menu, got %#v", items)
	}

	submenu, ok := schemaMenu["submenu"].([]map[string]interface{})
	if !ok || len(submenu) != 3 {
		t.Fatalf("expected 3 schema submenu items, got %#v", schemaMenu["submenu"])
	}
	if submenu[0]["text"] != "白霜拼音" {
		t.Fatalf("expected first schema label, got %#v", submenu[0]["text"])
	}
	if checked, _ := submenu[0]["checked"].(bool); !checked {
		t.Fatalf("expected current schema checked, got %#v", submenu[0])
	}
	if checked, _ := submenu[1]["checked"].(bool); checked {
		t.Fatalf("expected non-current schema unchecked, got %#v", submenu[1])
	}
}

func TestBuildMenuIncludesSchemeSetSubmenuBeforeSchemaMenu(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	writeTestSchemeSetConfig(t, appData, "Work")

	ime := newIsolatedTestIME(t)
	items := ime.buildMenu()
	if len(items) < 2 {
		t.Fatalf("expected menu items, got %#v", items)
	}

	var schemeSetIndex = -1
	var schemaIndex = -1
	var schemeSetMenu map[string]interface{}
	for i, item := range items {
		text, _ := item["text"].(string)
		if text == "切換方案集 / Switch Scheme Set" {
			schemeSetIndex = i
			schemeSetMenu = item
		}
		if text == "輸入方案(&I) / Input Schema" {
			schemaIndex = i
		}
	}
	if schemeSetIndex == -1 || schemaIndex == -1 {
		t.Fatalf("expected both scheme set and schema submenus, got %#v", items)
	}
	if schemeSetIndex >= schemaIndex {
		t.Fatalf("expected scheme set submenu before schema submenu, got schemeSetIndex=%d schemaIndex=%d", schemeSetIndex, schemaIndex)
	}

	submenu, ok := schemeSetMenu["submenu"].([]map[string]interface{})
	if !ok || len(submenu) != 4 {
		t.Fatalf("expected 4 scheme set submenu items, got %#v", schemeSetMenu["submenu"])
	}
	if submenu[0]["text"] != defaultSchemeSetName {
		t.Fatalf("expected default scheme set first, got %#v", submenu[0]["text"])
	}
	if submenu[1]["text"] != "Work" {
		t.Fatalf("expected Work scheme set second, got %#v", submenu[1]["text"])
	}
	if checked, _ := submenu[1]["checked"].(bool); !checked {
		t.Fatalf("expected Work scheme set checked, got %#v", submenu[1])
	}
	if submenu[2]["text"] != "" {
		t.Fatalf("expected separator before download item, got %#v", submenu[2])
	}
	if submenu[3]["text"] != "下载方案集(&D)" || submenu[3]["id"] != ID_DOWNLOAD_SCHEME_SET {
		t.Fatalf("expected download item under scheme set submenu, got %#v", submenu[3])
	}
}

func TestBuildMenuPlacesUpdateConfigBeforeDeploy(t *testing.T) {
	ime := newIsolatedTestIME(t)
	items := ime.buildMenu()

	openIndex := -1
	superIndex := -1
	updateIndex := -1
	deployIndex := -1
	for i, item := range items {
		text, _ := item["text"].(string)
		if text == "打开置顶短语" {
			openIndex = i
		}
		if text == "開啟超級簡拼 / Open Super Abbrev" {
			superIndex = i
		}
		if text == "更新配置(&P)" {
			updateIndex = i
		}
		if text == "重新部署(&R) / Redeploy" {
			deployIndex = i
		}
	}
	if openIndex == -1 || superIndex == -1 || updateIndex == -1 || deployIndex == -1 {
		t.Fatalf("expected custom phrase, super abbrev, update and deploy menu items, got %#v", items)
	}
	if openIndex >= updateIndex {
		t.Fatalf("expected custom phrase item before update config, got openIndex=%d updateIndex=%d", openIndex, updateIndex)
	}
	if superIndex != openIndex+1 {
		t.Fatalf("expected super abbrev item right after custom phrase, got openIndex=%d superIndex=%d", openIndex, superIndex)
	}
	if updateIndex >= deployIndex {
		t.Fatalf("expected update config before deploy, got updateIndex=%d deployIndex=%d", updateIndex, deployIndex)
	}
}

func TestBuildMenuGroupsSchemeSetSchemaUpdateAndDeployWithoutSeparators(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}

	ime := newIsolatedTestIME(t)
	items := ime.buildMenu()

	indexByText := map[string]int{}
	for i, item := range items {
		text, _ := item["text"].(string)
		indexByText[text] = i
	}

	group := []string{"切換方案集 / Switch Scheme Set", "輸入方案(&I) / Input Schema", "打开置顶短语", "開啟超級簡拼 / Open Super Abbrev", "更新配置(&P)", "重新部署(&R) / Redeploy"}
	for _, text := range group {
		if _, ok := indexByText[text]; !ok {
			t.Fatalf("expected menu item %q, got %#v", text, items)
		}
	}
	for i := 0; i < len(group)-1; i++ {
		current := indexByText[group[i]]
		next := indexByText[group[i+1]]
		if next != current+1 {
			t.Fatalf("expected %q and %q to be consecutive, got %d and %d", group[i], group[i+1], current, next)
		}
	}
}

func TestOnCommandOpenCustomPhraseCreatesFileAndOpensIt(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	oldOpen := openCustomPhraseTargetFunc
	defer func() {
		openCustomPhraseTargetFunc = oldOpen
	}()

	var openedPath string
	openCustomPhraseTargetFunc = func(path string) error {
		openedPath = path
		return nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 100,
		ID:     imecore.FlexibleID{Int: ID_OPEN_CUSTOM_PHRASE, IsInt: true},
	}, imecore.NewResponse(100, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected open custom phrase command handled, got %d", resp.ReturnValue)
	}
	wantPath := filepath.Join(appData, APP, customPhraseFileName)
	if openedPath != wantPath {
		t.Fatalf("expected opened custom phrase path %q, got %q", wantPath, openedPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected custom phrase file created: %v", err)
	}
	if !strings.Contains(string(data), "词汇<Tab>编码<Tab>权重") {
		t.Fatalf("expected default custom phrase template, got %q", string(data))
	}
}

func TestOnCommandOpenSuperAbbrevCreatesFileAndOpensIt(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	oldOpen := openSuperAbbrevTargetFunc
	defer func() {
		openSuperAbbrevTargetFunc = oldOpen
	}()

	var openedPath string
	openSuperAbbrevTargetFunc = func(path string) error {
		openedPath = path
		return nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 101,
		ID:     imecore.FlexibleID{Int: ID_OPEN_SUPER_ABBREV, IsInt: true},
	}, imecore.NewResponse(101, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected open super abbrev command handled, got %d", resp.ReturnValue)
	}
	wantPath := filepath.Join(appData, APP, superAbbrevFileName)
	if openedPath != wantPath {
		t.Fatalf("expected opened super abbrev path %q, got %q", wantPath, openedPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected super abbrev file created: %v", err)
	}
	if !strings.Contains(string(data), "词汇<Tab>编码") {
		t.Fatalf("expected default super abbrev template, got %q", string(data))
	}
}

func TestOnCommandOpenAutoPairSymbolsCreatesFileAndOpensIt(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetAutoPairSymbolsStateForTest()
	oldOpen := openAutoPairSymbolsTargetFunc
	defer func() {
		openAutoPairSymbolsTargetFunc = oldOpen
	}()

	var openedPath string
	openAutoPairSymbolsTargetFunc = func(path string) error {
		openedPath = path
		return nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 102,
		ID:     imecore.FlexibleID{Int: ID_OPEN_AUTO_PAIR_SYMBOLS, IsInt: true},
	}, imecore.NewResponse(102, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected open auto pair symbols command handled, got %d", resp.ReturnValue)
	}
	wantPath := filepath.Join(appData, APP, autoPairSymbolsFileName)
	if openedPath != wantPath {
		t.Fatalf("expected opened auto pair symbols path %q, got %q", wantPath, openedPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected auto pair symbols file created: %v", err)
	}
	if !strings.Contains(string(data), "每行一对符号") || !strings.Contains(string(data), "“”") {
		t.Fatalf("expected default auto pair symbols content, got %q", string(data))
	}
}

func TestCustomizeUIMapLoadsAutoPairRulesFromFile(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetAutoPairSymbolsStateForTest()

	path := filepath.Join(appData, APP, autoPairSymbolsFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create auto pair symbols dir: %v", err)
	}
	content := "# 成对符号\n{}\n《》\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write auto pair symbols file: %v", err)
	}

	ime := newIsolatedTestIME(t)
	customizeUI := ime.customizeUIMap()
	rules, ok := customizeUI["autoPairRules"].([]imecore.AutoPairRule)
	if !ok {
		t.Fatalf("expected autoPairRules slice, got %#v", customizeUI["autoPairRules"])
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 auto pair rules, got %#v", rules)
	}
	if rules[0].Open != "{" || rules[0].Close != "}" {
		t.Fatalf("unexpected first auto pair rule: %#v", rules[0])
	}
	if rules[1].Open != "《" || rules[1].Close != "》" {
		t.Fatalf("unexpected second auto pair rule: %#v", rules[1])
	}
}

func TestHandleRequestRefreshesCustomizeUIWhenAutoPairRulesFileChanges(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetAutoPairSymbolsStateForTest()

	path := filepath.Join(appData, APP, autoPairSymbolsFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create auto pair symbols dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("(\t)\n"), 0o644); err != nil {
		t.Fatalf("write initial auto pair symbols file: %v", err)
	}

	ime := newIsolatedTestIME(t)
	first := ime.HandleRequest(&imecore.Request{
		Method: "onMenu",
		SeqNum: 200,
	})
	firstRules, ok := first.CustomizeUI["autoPairRules"].([]imecore.AutoPairRule)
	if !ok || len(firstRules) != 1 || firstRules[0].Open != "(" || firstRules[0].Close != ")" {
		t.Fatalf("unexpected first auto pair rules: %#v", first.CustomizeUI)
	}

	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, []byte("[\t]\n"), 0o644); err != nil {
		t.Fatalf("write updated auto pair symbols file: %v", err)
	}

	second := ime.HandleRequest(&imecore.Request{
		Method: "onMenu",
		SeqNum: 201,
	})
	secondRules, ok := second.CustomizeUI["autoPairRules"].([]imecore.AutoPairRule)
	if !ok || len(secondRules) != 1 || secondRules[0].Open != "[" || secondRules[0].Close != "]" {
		t.Fatalf("unexpected refreshed auto pair rules: %#v", second.CustomizeUI)
	}
}

func TestBuildMenuUsesSwitcherSaveOptions(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.saveOptions = []string{"emoji", "full_shape"}
	backend.schemaSwitches[backend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "full_shape", States: []string{"半角", "全角"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}

	items := ime.buildMenu()
	if len(items) < 3 {
		t.Fatalf("expected switch items in menu, got %#v", items)
	}
	if text, _ := items[0]["text"].(string); text != "常规 → Emoji" {
		t.Fatalf("expected emoji switch from save_options first, got %#v", items[0])
	}
	if text, _ := items[1]["text"].(string); text != "半角 → 全角" {
		t.Fatalf("expected full_shape switch from save_options second, got %#v", items[1])
	}
	if _, ok := items[2]["text"].(string); !ok || items[2]["text"] != "" {
		t.Fatalf("expected separator after dynamic switches, got %#v", items[2])
	}
}

func TestOnCommandTogglesDynamicSwitcherOption(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.saveOptions = []string{"emoji"}
	backend.schemaSwitches[backend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
	}

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 99,
		ID:     imecore.FlexibleID{Int: switchCommandID(0), IsInt: true},
	}, imecore.NewResponse(99, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected dynamic switch command handled, got %d", resp.ReturnValue)
	}
	if !backend.GetOption("emoji") {
		t.Fatal("expected dynamic switch option toggled on")
	}
}

func TestOnCommandSwitchesSchemeSetAndRedeploysWithSelectedUserDir(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 100,
		ID:     imecore.FlexibleID{Int: schemeSetCommandID(1), IsInt: true},
	}, imecore.NewResponse(100, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected scheme set command handled, got %d", resp.ReturnValue)
	}
	if backend.redeployCalls != 1 {
		t.Fatalf("expected scheme set switch to redeploy once, got %d", backend.redeployCalls)
	}
	wantUserDir := filepath.Join(appData, APP, "Work")
	if backend.redeployUserDir != wantUserDir {
		t.Fatalf("expected redeploy user dir %q, got %q", wantUserDir, backend.redeployUserDir)
	}
	if got := currentSchemeSetName(); got != "Work" {
		t.Fatalf("expected current scheme set Work after switch, got %q", got)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "Scheme-set switch succeeded" {
		t.Fatalf("unexpected tray notification: %#v", resp.TrayNotification)
	}
}

func TestHandleRequestRecreatesSessionAfterSchemeSetSwitchAcrossInstances(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()
	resetSchemeSetVersionForTest()
	t.Cleanup(resetSchemeSetVersionForTest)
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}

	first := newTestIME()
	second := newTestIME()
	secondResp := second.HandleRequest(&imecore.Request{
		SeqNum: 200,
		Method: "onActivate",
	})
	if secondResp.ReturnValue != 1 {
		t.Fatalf("expected second instance activation handled, got %d", secondResp.ReturnValue)
	}
	secondBackend := second.backend.(*testBackend)
	if !secondBackend.session {
		t.Fatal("expected second instance to have an active session before scheme set switch")
	}

	switchResp := first.onCommand(&imecore.Request{
		SeqNum: 201,
		ID:     imecore.FlexibleID{Int: schemeSetCommandID(1), IsInt: true},
	}, imecore.NewResponse(201, true))
	if switchResp.ReturnValue != 1 {
		t.Fatalf("expected scheme set command handled, got %d", switchResp.ReturnValue)
	}
	if currentSchemeSetVersion() == second.schemeSetVersion {
		t.Fatal("expected global scheme set version to advance after switch")
	}

	menuResp := second.HandleRequest(&imecore.Request{
		SeqNum: 202,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})
	if menuResp.ReturnValue != 0 {
		t.Fatalf("expected v1 onMenu to stay unreachable, got %d", menuResp.ReturnValue)
	}
	if secondBackend.destroySessionCalls != 1 {
		t.Fatalf("expected second instance to destroy stale session once, got %d", secondBackend.destroySessionCalls)
	}
	if !secondBackend.session {
		t.Fatal("expected second instance to recreate session after scheme set switch")
	}
	if second.schemeSetVersion != currentSchemeSetVersion() {
		t.Fatalf("expected second instance scheme set version synced to %d, got %d", currentSchemeSetVersion(), second.schemeSetVersion)
	}
}

func TestOnCommandSwitchesSchemeSetShowsProgressAndAsyncSuccessNotification(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}

	ime := newIsolatedTestIME(t)
	asyncResponses := make(chan *imecore.Response, 1)
	ime.SetAsyncResponseSender(func(resp *imecore.Response) {
		asyncResponses <- resp
	})

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 100,
		ID:     imecore.FlexibleID{Int: schemeSetCommandID(1), IsInt: true},
	}, imecore.NewResponse(100, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected scheme set command handled, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "Switching scheme set..." {
		t.Fatalf("unexpected initial tray notification: %#v", resp.TrayNotification)
	}

	select {
	case asyncResp := <-asyncResponses:
		if asyncResp.TrayNotification == nil {
			t.Fatalf("expected async tray notification, got %#v", asyncResp)
		}
		if asyncResp.TrayNotification.Message != "Scheme-set switch succeeded" {
			t.Fatalf("unexpected async tray notification: %#v", asyncResp.TrayNotification)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async scheme set tray notification")
	}
}

func TestOnCommandKnownSchemeSetFailureDoesNotLogUnknownCommand(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.redeployOK = false

	var buf bytes.Buffer
	oldWriter := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldWriter)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 101,
		ID:     imecore.FlexibleID{Int: schemeSetCommandID(1), IsInt: true},
	}, imecore.NewResponse(101, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected scheme set command to fail when redeploy fails, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "Scheme-set switch failed" {
		t.Fatalf("unexpected tray notification: %#v", resp.TrayNotification)
	}
	if got := buf.String(); strings.Contains(got, "unknown command") {
		t.Fatalf("expected known scheme set command failure not to log unknown command, got %q", got)
	}
}

func TestOnCommandUpdateConfigFailsWithoutGit(t *testing.T) {
	oldLookPath := gitLookPathFunc
	oldIsRepo := gitIsRepoFunc
	defer func() {
		gitLookPathFunc = oldLookPath
		gitIsRepoFunc = oldIsRepo
	}()
	resetConfigUpdateStateForTest()

	gitLookPathFunc = func(file string) (string, error) {
		return "", errors.New("missing git")
	}
	gitIsRepoFunc = func(dir string) (bool, error) {
		t.Fatalf("did not expect git repo check when git is missing")
		return false, nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 101,
		ID:     imecore.FlexibleID{Int: ID_UPDATE_CONFIG, IsInt: true},
	}, imecore.NewResponse(101, true))
	if resp.ReturnValue != 0 {
		t.Fatalf("expected update config command to fail without git, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "未检测到 Git 命令" {
		t.Fatalf("unexpected tray notification: %#v", resp.TrayNotification)
	}
}

func TestOnCommandUpdateConfigFailsWhenUserDirNotGitRepo(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	oldLookPath := gitLookPathFunc
	oldIsRepo := gitIsRepoFunc
	defer func() {
		gitLookPathFunc = oldLookPath
		gitIsRepoFunc = oldIsRepo
	}()
	resetConfigUpdateStateForTest()

	gitLookPathFunc = func(file string) (string, error) {
		return "git", nil
	}
	gitIsRepoFunc = func(dir string) (bool, error) {
		want := filepath.Join(appData, APP, defaultSchemeSetName)
		if dir != want {
			t.Fatalf("expected git repo check dir %q, got %q", want, dir)
		}
		return false, nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 102,
		ID:     imecore.FlexibleID{Int: ID_UPDATE_CONFIG, IsInt: true},
	}, imecore.NewResponse(102, true))
	if resp.ReturnValue != 0 {
		t.Fatalf("expected update config command to fail for non-git dir, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "Current scheme-set directory is not a Git repository" {
		t.Fatalf("unexpected tray notification: %#v", resp.TrayNotification)
	}
}

func TestOnCommandUpdateConfigRunsGitPullAsyncAndNotifies(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	oldLookPath := gitLookPathFunc
	oldIsRepo := gitIsRepoFunc
	oldPull := gitPullFunc
	defer func() {
		gitLookPathFunc = oldLookPath
		gitIsRepoFunc = oldIsRepo
		gitPullFunc = oldPull
		resetConfigUpdateStateForTest()
	}()
	resetConfigUpdateStateForTest()

	userDir := filepath.Join(appData, APP, defaultSchemeSetName)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("create user dir: %v", err)
	}

	gitLookPathFunc = func(file string) (string, error) {
		return "git", nil
	}
	gitIsRepoFunc = func(dir string) (bool, error) {
		return dir == userDir, nil
	}
	pullDone := make(chan struct{})
	var pullDir string
	gitPullFunc = func(dir string) (string, error) {
		pullDir = dir
		time.Sleep(120 * time.Millisecond)
		close(pullDone)
		return "Already up to date.", nil
	}

	ime := newIsolatedTestIME(t)
	asyncResponses := make(chan *imecore.Response, 1)
	ime.SetAsyncResponseSender(func(resp *imecore.Response) {
		asyncResponses <- resp
	})

	start := time.Now()
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 103,
		ID:     imecore.FlexibleID{Int: ID_UPDATE_CONFIG, IsInt: true},
	}, imecore.NewResponse(103, true))
	if elapsed := time.Since(start); elapsed > 60*time.Millisecond {
		t.Fatalf("expected update config command to return quickly, took %s", elapsed)
	}
	if resp.ReturnValue != 1 {
		t.Fatalf("expected update config command to succeed, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "Starting configuration update..." {
		t.Fatalf("unexpected initial tray notification: %#v", resp.TrayNotification)
	}

	select {
	case <-pullDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async git pull")
	}

	if pullDir != userDir {
		t.Fatalf("expected git pull dir %q, got %q", userDir, pullDir)
	}

	select {
	case asyncResp := <-asyncResponses:
		if asyncResp.TrayNotification == nil {
			t.Fatalf("expected async tray notification, got %#v", asyncResp)
		}
		if asyncResp.TrayNotification.Message != "配置已是最新" {
			t.Fatalf("unexpected async tray notification: %#v", asyncResp.TrayNotification)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async tray notification")
	}
}

func TestApplyAppearanceCommandChangesCandidateLayout(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 1

	if !ime.applyAppearanceCommand(ID_APPEARANCE_LAYOUT_HORIZONTAL) {
		t.Fatal("expected horizontal layout command handled")
	}
	if ime.style.CandidatePerRow != 9 {
		t.Fatalf("expected horizontal layout to default to 9 per row, got %d", ime.style.CandidatePerRow)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_PER_ROW_5) {
		t.Fatal("expected per-row command handled")
	}
	if ime.style.CandidatePerRow != 5 {
		t.Fatalf("expected 5 candidates per row, got %d", ime.style.CandidatePerRow)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_PER_ROW_9) {
		t.Fatal("expected per-row 9 command handled")
	}
	if ime.style.CandidatePerRow != 9 {
		t.Fatalf("expected 9 candidates per row, got %d", ime.style.CandidatePerRow)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_LAYOUT_VERTICAL) {
		t.Fatal("expected vertical layout command handled")
	}
	if ime.style.CandidatePerRow != 1 {
		t.Fatalf("expected vertical layout to force 1 per row, got %d", ime.style.CandidatePerRow)
	}
}

func TestApplyAppearanceCommandChangesCandidateSpacing(t *testing.T) {
	ime := newIsolatedTestIME(t)

	if !ime.applyAppearanceCommand(ID_APPEARANCE_SPACING_0) {
		t.Fatal("expected spacing command handled")
	}
	if ime.style.CandidateSpacing != 0 {
		t.Fatalf("expected candidate spacing 0, got %d", ime.style.CandidateSpacing)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_SPACING_30) {
		t.Fatal("expected spacing command handled")
	}
	if ime.style.CandidateSpacing != 30 {
		t.Fatalf("expected candidate spacing 30, got %d", ime.style.CandidateSpacing)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_SPACING_50) {
		t.Fatal("expected spacing command handled")
	}
	if ime.style.CandidateSpacing != 50 {
		t.Fatalf("expected candidate spacing 50, got %d", ime.style.CandidateSpacing)
	}
}

func TestEffectiveCandidatePerRowIsCappedByCandidateCount(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 9
	ime.style.CandidateCount = 3

	if got := ime.effectiveCandidatePerRow(); got != 3 {
		t.Fatalf("expected effective per-row capped to 3, got %d", got)
	}

	customizeUI := ime.customizeUIMap()
	candPerRow, ok := customizeUI["candPerRow"].(int)
	if !ok {
		t.Fatalf("expected candPerRow int, got %#v", customizeUI["candPerRow"])
	}
	if candPerRow != 3 {
		t.Fatalf("expected customizeUI candPerRow capped to 3, got %d", candPerRow)
	}
	if got, ok := customizeUI["candSpacing"].(int); !ok || got != ime.style.CandidateSpacing {
		t.Fatalf("expected customizeUI candSpacing %d, got %#v", ime.style.CandidateSpacing, customizeUI["candSpacing"])
	}
	if got, ok := customizeUI["candCommentColor"].(string); !ok || got != ime.style.CandidateCommentColor {
		t.Fatalf("expected customizeUI candCommentColor %q, got %#v", ime.style.CandidateCommentColor, customizeUI["candCommentColor"])
	}
	if got, ok := customizeUI["candCommentHighlightColor"].(string); !ok || got != ime.style.CandidateCommentHighlightColor {
		t.Fatalf("expected customizeUI candCommentHighlightColor %q, got %#v", ime.style.CandidateCommentHighlightColor, customizeUI["candCommentHighlightColor"])
	}
	rules, ok := customizeUI["autoPairRules"].([]imecore.AutoPairRule)
	if !ok {
		t.Fatalf("expected autoPairRules slice, got %#v", customizeUI["autoPairRules"])
	}
	if len(rules) == 0 {
		t.Fatal("expected autoPairRules to be populated")
	}
	if rules[0].Open != "“" || rules[0].Close != "”" {
		t.Fatalf("unexpected first autoPairRule: %#v", rules[0])
	}
}

func TestOnCommandAppearanceRefreshesCurrentCandidates(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 17,
		ID:     imecore.FlexibleID{Int: ID_APPEARANCE_PER_ROW_5, IsInt: true},
	}, imecore.NewResponse(17, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected appearance command handled, got %d", resp.ReturnValue)
	}
	if ime.style.CandidatePerRow != 5 {
		t.Fatalf("expected per-row updated to 5, got %d", ime.style.CandidatePerRow)
	}
	if resp.CompositionString != "ni" {
		t.Fatalf("expected current composition returned, got %#v", resp.CompositionString)
	}
	if !resp.ShowCandidates || len(resp.CandidateList) == 0 {
		t.Fatalf("expected current candidates returned for immediate refresh, got %#v", resp)
	}
}

func TestOnCommandCandidateCountWritesConfigAndDeploysConfigFile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	oldDeployConfigFileFunc := deployConfigFileFunc
	oldStartMaintenanceFunc := startMaintenanceFunc
	oldJoinMaintenanceThreadFunc := joinMaintenanceThreadFunc
	deployCalls := 0
	var deployFile string
	var deployKey string
	maintenanceCalls := 0
	joinCalls := 0
	deployConfigFileFunc = func(filePath, key string) bool {
		deployCalls++
		deployFile = filePath
		deployKey = key
		return true
	}
	startMaintenanceFunc = func(fullcheck bool) bool {
		maintenanceCalls++
		return true
	}
	joinMaintenanceThreadFunc = func() {
		joinCalls++
	}
	defer func() {
		deployConfigFileFunc = oldDeployConfigFileFunc
		startMaintenanceFunc = oldStartMaintenanceFunc
		joinMaintenanceThreadFunc = oldJoinMaintenanceThreadFunc
	}()

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{
		{Text: "你"},
		{Text: "呢"},
		{Text: "泥"},
		{Text: "尼"},
		{Text: "拟"},
		{Text: "逆"},
	}

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 18,
		ID:     imecore.FlexibleID{Int: ID_APPEARANCE_CAND_COUNT_3, IsInt: true},
	}, imecore.NewResponse(18, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected candidate count command handled, got %d", resp.ReturnValue)
	}
	if ime.style.CandidateCount != 3 {
		t.Fatalf("expected candidate count updated to 3, got %d", ime.style.CandidateCount)
	}
	if backend.redeployCalls != 0 {
		t.Fatalf("expected candidate count change not to trigger full redeploy, got %d", backend.redeployCalls)
	}
	if deployCalls != 1 {
		t.Fatalf("expected deploy config file called once, got %d", deployCalls)
	}
	if maintenanceCalls != 1 || joinCalls != 1 {
		t.Fatalf("expected maintenance start/join once, got start=%d join=%d", maintenanceCalls, joinCalls)
	}
	if deployFile != "default.yaml" || deployKey != "config_version" {
		t.Fatalf("unexpected deploy config args file=%q key=%q", deployFile, deployKey)
	}
	configPath := filepath.Join(os.Getenv("APPDATA"), APP, defaultSchemeSetName, rimeDefaultCustomConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected %s written, got err=%v", configPath, err)
	}
	if string(data) != "config_version: '3'\npatch:\n  menu/page_size: 3\n" {
		t.Fatalf("unexpected config content: %q", string(data))
	}
}

func TestOnCommandCandidateCountUsesRuntimePageSizeWhenAvailable(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	oldDeployConfigFileFunc := deployConfigFileFunc
	oldStartMaintenanceFunc := startMaintenanceFunc
	oldJoinMaintenanceThreadFunc := joinMaintenanceThreadFunc
	deployConfigFileFunc = func(filePath, key string) bool {
		t.Fatalf("did not expect deploy config file call, got file=%q key=%q", filePath, key)
		return false
	}
	startMaintenanceFunc = func(fullcheck bool) bool {
		t.Fatalf("did not expect maintenance start")
		return false
	}
	joinMaintenanceThreadFunc = func() {
		t.Fatalf("did not expect maintenance join")
	}
	defer func() {
		deployConfigFileFunc = oldDeployConfigFileFunc
		startMaintenanceFunc = oldStartMaintenanceFunc
		joinMaintenanceThreadFunc = oldJoinMaintenanceThreadFunc
	}()

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.pageSizeOK = true
	backend.composition = "ni"
	backend.candidates = []candidateItem{
		{Text: "你"},
		{Text: "呢"},
		{Text: "泥"},
		{Text: "尼"},
	}

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 20,
		ID:     imecore.FlexibleID{Int: ID_APPEARANCE_CAND_COUNT_3, IsInt: true},
	}, imecore.NewResponse(20, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected candidate count command handled, got %d", resp.ReturnValue)
	}
	if len(backend.pageSizeCalls) == 0 || backend.pageSizeCalls[len(backend.pageSizeCalls)-1] != 3 {
		t.Fatalf("expected runtime page size call with 3, got %#v", backend.pageSizeCalls)
	}
	configPath := filepath.Join(os.Getenv("APPDATA"), APP, defaultSchemeSetName, rimeDefaultCustomConfigFileName)
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected no config file write on runtime success, got err=%v", err)
	}
}

func TestTypeduckSettingsUpdateCustomizesAndRedeploys(t *testing.T) {
	oldCustomize := customizeTypeDuckSettingsFunc
	oldReloadTypeDuckSettingsFunc := reloadTypeDuckSettingsFunc
	var gotPrefs typeDuckRimePreferences
	customizeCalls := 0
	reloadCalls := 0
	customizeTypeDuckSettingsFunc = func(prefs typeDuckRimePreferences) bool {
		customizeCalls++
		gotPrefs = prefs
		return true
	}
	reloadTypeDuckSettingsFunc = func(sharedDir, userDir, appname, appver string) bool {
		reloadCalls++
		return true
	}
	defer func() {
		customizeTypeDuckSettingsFunc = oldCustomize
		reloadTypeDuckSettingsFunc = oldReloadTypeDuckSettingsFunc
	}()

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.HandleRequest(&imecore.Request{
		Method: "typeduckSettingsUpdate",
		SeqNum: 42,
		TypeDuckSettings: &imecore.TypeDuckSettingsUpdate{
			PageSize:         8,
			EnableCompletion: false,
			EnableCorrection: true,
			EnableSentence:   false,
			EnableLearning:   false,
			IsCangjie5:       false,
		},
	})

	if !resp.Success || resp.ReturnValue != 1 {
		t.Fatalf("expected settings update success, got %#v", resp)
	}
	if customizeCalls != 1 {
		t.Fatalf("expected one customize call, got %d", customizeCalls)
	}
	if gotPrefs.PageSize != 8 || gotPrefs.EnableCompletion || !gotPrefs.EnableCorrection ||
		gotPrefs.EnableSentence || gotPrefs.EnableLearning || gotPrefs.IsCangjie5 {
		t.Fatalf("unexpected customized prefs: %#v", gotPrefs)
	}
	if backend.redeployCalls != 0 {
		t.Fatalf("expected settings update to use maintenance instead of full backend redeploy, got %d", backend.redeployCalls)
	}
	if reloadCalls != 1 {
		t.Fatalf("expected incremental reload once, got %d", reloadCalls)
	}
	if resp.TrayNotification != nil {
		t.Fatalf("expected settings success to avoid tray notification, got %#v", resp.TrayNotification)
	}
}

func TestTypeduckSettingsUpdateStaysSilentForLauncherClient(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	oldCustomize := customizeTypeDuckSettingsFunc
	oldReloadTypeDuckSettingsFunc := reloadTypeDuckSettingsFunc
	customizeTypeDuckSettingsFunc = func(prefs typeDuckRimePreferences) bool {
		return true
	}
	reloadTypeDuckSettingsFunc = func(sharedDir, userDir, appname, appver string) bool {
		return true
	}
	defer func() {
		customizeTypeDuckSettingsFunc = oldCustomize
		reloadTypeDuckSettingsFunc = oldReloadTypeDuckSettingsFunc
	}()

	ime := newIsolatedTestIME(t)
	ime.Client.ID = "launcher-tray"

	resp := ime.HandleRequest(&imecore.Request{
		Method: "typeduckSettingsUpdate",
		SeqNum: 142,
		TypeDuckSettings: &imecore.TypeDuckSettingsUpdate{
			PageSize:         8,
			EnableCompletion: false,
			EnableCorrection: true,
			EnableSentence:   false,
			EnableLearning:   false,
			IsCangjie5:       false,
		},
	})

	if !resp.Success || resp.ReturnValue != 1 {
		t.Fatalf("expected launcher settings update success, got %#v", resp)
	}
	if resp.TrayNotification != nil {
		t.Fatalf("expected launcher settings update to avoid tray notification, got %#v", resp.TrayNotification)
	}
}

func TestTypeduckSettingsUpdateWritesCustomYamlFallbackWhenLeversFails(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	oldCustomize := customizeTypeDuckSettingsFunc
	oldReloadTypeDuckSettingsFunc := reloadTypeDuckSettingsFunc
	customizeTypeDuckSettingsFunc = func(prefs typeDuckRimePreferences) bool {
		return false
	}
	reloadTypeDuckSettingsFunc = func(sharedDir, userDir, appname, appver string) bool {
		return true
	}
	defer func() {
		customizeTypeDuckSettingsFunc = oldCustomize
		reloadTypeDuckSettingsFunc = oldReloadTypeDuckSettingsFunc
	}()

	ime := newIsolatedTestIME(t)
	resp := ime.HandleRequest(&imecore.Request{
		Method: "typeduckSettingsUpdate",
		SeqNum: 43,
		TypeDuckSettings: &imecore.TypeDuckSettingsUpdate{
			PageSize:         7,
			EnableCompletion: false,
			EnableCorrection: false,
			EnableSentence:   false,
			EnableLearning:   false,
			IsCangjie5:       true,
		},
	})
	if !resp.Success || resp.ReturnValue != 1 {
		t.Fatalf("expected fallback settings update success, got %#v", resp)
	}
	userDir := filepath.Join(os.Getenv("APPDATA"), APP, defaultSchemeSetName)
	defaultData, err := os.ReadFile(filepath.Join(userDir, rimeDefaultCustomConfigFileName))
	if err != nil {
		t.Fatalf("expected default custom yaml written: %v", err)
	}
	if !strings.Contains(string(defaultData), "menu/page_size: 7") {
		t.Fatalf("expected page size in default custom yaml, got %q", string(defaultData))
	}
	commonData, err := os.ReadFile(filepath.Join(userDir, rimeCommonCustomConfigFileName))
	if err != nil {
		t.Fatalf("expected common custom yaml written: %v", err)
	}
	commonText := string(commonData)
	for _, want := range []string{"common:/show_cangjie_roots", "common:/disable_completion", "common:/disable_sentence", "common:/disable_learning"} {
		if !strings.Contains(commonText, want) {
			t.Fatalf("expected %q in common custom yaml, got %q", want, commonText)
		}
	}
}

func TestTypeDuckPreferencesPathUsesRoamingAppData(t *testing.T) {
	appData := t.TempDir()
	localAppData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", localAppData)

	got := typeDuckPreferencesPath()
	want := filepath.Join(appData, APP, typeDuckPreferencesFileName)
	if got != want {
		t.Fatalf("expected TypeDuck preferences under roaming APPDATA, got %q want %q", got, want)
	}
}

func TestFillResponseIncludesTypeDuckCandidatePageMetadata(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "hou"
	backend.pageNo = 2
	backend.pageSize = 6
	backend.isLastPage = false
	backend.candidates = []candidateItem{{Text: "好"}, {Text: "號"}}

	resp := imecore.NewResponse(51, true)
	if !ime.fillResponseFromBackendState(resp, false) {
		t.Fatal("expected backend state to fill response")
	}
	page := resp.TypeDuckCandidatePage
	if page == nil {
		t.Fatal("expected TypeDuck candidate page metadata")
	}
	if page.PageIndex != 2 || page.PageSize != 6 || !page.HasPrevious || !page.HasNext {
		t.Fatalf("unexpected page metadata: %#v", page)
	}
}

func TestBuildMenuIncludesCandidateLayoutSubmenus(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 5
	ime.style.CandidateCommentFontPoint = 18

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外觀(&A) / Appearance" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var layoutMenu map[string]interface{}
	var perRowMenu map[string]interface{}
	var spacingMenu map[string]interface{}
	var commentFontMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "候选排列" {
			layoutMenu = item
		}
		if text == "每行候选数" {
			perRowMenu = item
		}
		if text == "候选间距" {
			spacingMenu = item
		}
		if text == "注释文字大小" {
			commentFontMenu = item
		}
	}
	if layoutMenu == nil || perRowMenu == nil || spacingMenu == nil || commentFontMenu == nil {
		t.Fatalf("expected layout menus, got %#v", submenu)
	}

	layoutItems, ok := layoutMenu["submenu"].([]map[string]interface{})
	if !ok || len(layoutItems) != 2 {
		t.Fatalf("expected 2 layout items, got %#v", layoutMenu["submenu"])
	}
	if checked, _ := layoutItems[1]["checked"].(bool); !checked {
		t.Fatalf("expected horizontal layout checked, got %#v", layoutItems[1])
	}

	perRowItems, ok := perRowMenu["submenu"].([]map[string]interface{})
	if !ok || len(perRowItems) != 4 {
		t.Fatalf("expected 4 per-row items, got %#v", perRowMenu["submenu"])
	}
	if checked, _ := perRowItems[1]["checked"].(bool); !checked {
		t.Fatalf("expected 5-per-row checked, got %#v", perRowItems[1])
	}
	if enabled, _ := perRowMenu["enabled"].(bool); !enabled {
		t.Fatalf("expected per-row menu enabled in horizontal mode, got %#v", perRowMenu)
	}

	spacingItems, ok := spacingMenu["submenu"].([]map[string]interface{})
	if !ok || len(spacingItems) != 6 {
		t.Fatalf("expected 6 spacing items, got %#v", spacingMenu["submenu"])
	}
	if checked, _ := spacingItems[2]["checked"].(bool); !checked {
		t.Fatalf("expected spacing 20 checked, got %#v", spacingItems[2])
	}

	commentFontItems, ok := commentFontMenu["submenu"].([]map[string]interface{})
	if !ok || len(commentFontItems) != 5 {
		t.Fatalf("expected 5 comment font items, got %#v", commentFontMenu["submenu"])
	}
	if checked, _ := commentFontItems[2]["checked"].(bool); !checked {
		t.Fatalf("expected comment font 18 checked, got %#v", commentFontItems[2])
	}
}

func TestBuildMenuCapsPerRowHighlightByCandidateCount(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 9
	ime.style.CandidateCount = 3

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外觀(&A) / Appearance" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var perRowMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "每行候选数" {
			perRowMenu = item
			break
		}
	}
	if perRowMenu == nil {
		t.Fatalf("expected per-row menu, got %#v", submenu)
	}

	perRowItems, ok := perRowMenu["submenu"].([]map[string]interface{})
	if !ok || len(perRowItems) != 4 {
		t.Fatalf("expected 4 per-row items, got %#v", perRowMenu["submenu"])
	}

	if checked, _ := perRowItems[0]["checked"].(bool); !checked {
		t.Fatalf("expected 3-per-row checked when candidate count is 3, got %#v", perRowItems[0])
	}
	if checked, _ := perRowItems[3]["checked"].(bool); checked {
		t.Fatalf("expected 9-per-row unchecked when candidate count is 3, got %#v", perRowItems[3])
	}
}

func TestBuildMenuDisablesPerRowSubmenuInVerticalLayout(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 1

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外觀(&A) / Appearance" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var perRowMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "每行候选数" {
			perRowMenu = item
			break
		}
	}
	if perRowMenu == nil {
		t.Fatalf("expected per-row menu, got %#v", submenu)
	}

	if enabled, _ := perRowMenu["enabled"].(bool); enabled {
		t.Fatalf("expected per-row menu disabled in vertical mode, got %#v", perRowMenu)
	}
	perRowItems, ok := perRowMenu["submenu"].([]map[string]interface{})
	if !ok || len(perRowItems) != 4 {
		t.Fatalf("expected 4 per-row items, got %#v", perRowMenu["submenu"])
	}
	for _, item := range perRowItems {
		if enabled, _ := item["enabled"].(bool); enabled {
			t.Fatalf("expected per-row item disabled in vertical mode, got %#v", item)
		}
	}
}

func TestBuildMenuIncludesCandidateCountSubmenu(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidateCount = 7

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外觀(&A) / Appearance" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var countMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "总候选数量" {
			countMenu = item
			break
		}
	}
	if countMenu == nil {
		t.Fatalf("expected candidate count menu, got %#v", submenu)
	}

	countItems, ok := countMenu["submenu"].([]map[string]interface{})
	if !ok || len(countItems) != 4 {
		t.Fatalf("expected 4 candidate count items, got %#v", countMenu["submenu"])
	}
	if checked, _ := countItems[2]["checked"].(bool); !checked {
		t.Fatalf("expected 7-count checked, got %#v", countItems[2])
	}
}

func TestBuildMenuIncludesSharedInputStateToggle(t *testing.T) {
	ime := newIsolatedTestIME(t)

	items := ime.buildMenu()
	found := false
	for _, item := range items {
		if text, _ := item["text"].(string); text == "輸入狀態共享 / Share Input State" {
			found = true
			if checked, _ := item["checked"].(bool); checked {
				t.Fatalf("expected input state sharing disabled by default, got %#v", item)
			}
		}
	}
	if !found {
		t.Fatalf("expected shared input state toggle in menu, got %#v", items)
	}
}

func TestBuildMenuIncludesInputSettingsSubmenu(t *testing.T) {
	ime := newIsolatedTestIME(t)

	items := ime.buildMenu()
	var inputSettingsMenu map[string]interface{}
	for _, item := range items {
		if text, _ := item["text"].(string); text == "輸入設定 / Input Settings" {
			inputSettingsMenu = item
			break
		}
	}
	if inputSettingsMenu == nil {
		t.Fatalf("expected input settings menu, got %#v", items)
	}

	submenu, ok := inputSettingsMenu["submenu"].([]map[string]interface{})
	if !ok || len(submenu) != 3 {
		t.Fatalf("expected three input settings items, got %#v", inputSettingsMenu["submenu"])
	}
	if text, _ := submenu[0]["text"].(string); text != "自动插入成对符号" {
		t.Fatalf("unexpected input settings item: %#v", submenu[0])
	}
	if checked, _ := submenu[0]["checked"].(bool); checked {
		t.Fatalf("expected auto pair quotes disabled by default, got %#v", submenu[0])
	}
	if text, _ := submenu[1]["text"].(string); text != "成对符号" {
		t.Fatalf("unexpected second input settings item: %#v", submenu[1])
	}
	if text, _ := submenu[2]["text"].(string); text != "分号键次选" {
		t.Fatalf("unexpected third input settings item: %#v", submenu[2])
	}
	if checked, _ := submenu[2]["checked"].(bool); checked {
		t.Fatalf("expected semicolon select second disabled by default, got %#v", submenu[2])
	}
}

func TestOnCommandTogglesAutoPairQuotes(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 21,
		ID:     imecore.FlexibleID{Int: ID_INPUT_AUTO_PAIR_QUOTES, IsInt: true},
	}, imecore.NewResponse(21, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected auto pair quotes command handled, got %d", resp.ReturnValue)
	}
	if !ime.autoPairQuotes {
		t.Fatal("expected auto pair quotes enabled")
	}
	if got, ok := resp.CustomizeUI["autoPairQuotes"].(bool); !ok || !got {
		t.Fatalf("expected customizeUI autoPairQuotes true, got %#v", resp.CustomizeUI["autoPairQuotes"])
	}

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected appearance config written to disk: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("expected valid appearance config json: %v", err)
	}
	if got := persisted["auto_pair_quotes"]; got != true {
		t.Fatalf("expected persisted auto_pair_quotes true, got %#v", got)
	}
}

func TestOnCommandTogglesSemicolonSelectSecond(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 22,
		ID:     imecore.FlexibleID{Int: ID_INPUT_SEMICOLON_SELECT_SECOND, IsInt: true},
	}, imecore.NewResponse(22, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon select second command handled, got %d", resp.ReturnValue)
	}
	if !ime.semicolonSelectSecond {
		t.Fatal("expected semicolon select second enabled after toggle")
	}
	if got, ok := resp.CustomizeUI["semicolonSelectSecond"].(bool); !ok || !got {
		t.Fatalf("expected customizeUI semicolonSelectSecond true, got %#v", resp.CustomizeUI["semicolonSelectSecond"])
	}

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected appearance config written to disk: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("expected valid appearance config json: %v", err)
	}
	if got := persisted["semicolon_select_second"]; got != true {
		t.Fatalf("expected persisted semicolon_select_second true, got %#v", got)
	}
}

func TestFillResponseFromBackendStatePrependsCustomPhraseCandidates(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\nalps\tal\t5\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{
		{Text: "阿"},
		{Text: "啊"},
	}

	resp := imecore.NewResponse(30, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 3 {
		t.Fatalf("expected custom and backend candidates, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "alpha" || resp.CandidateList[1] != "alps" {
		t.Fatalf("expected custom phrase candidates prepended, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[2] != "阿" {
		t.Fatalf("expected backend candidates appended after custom phrases, got %#v", resp.CandidateList)
	}
	if resp.CandidateCursor != 0 {
		t.Fatalf("expected custom phrase candidate cursor default to 0, got %d", resp.CandidateCursor)
	}
	if resp.SetSelKeys != "1234" {
		t.Fatalf("expected select keys recomputed for combined candidates, got %q", resp.SetSelKeys)
	}
}

func TestFillResponseFromBackendStateDeduplicatesCustomPhraseAgainstBackendCandidates(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n传\tc\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "c"
	backend.candidates = []candidateItem{
		{Text: "传"},
		{Text: "船"},
		{Text: "串"},
	}

	resp := imecore.NewResponse(31, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 3 {
		t.Fatalf("expected deduplicated custom and backend candidates, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "传" {
		t.Fatalf("expected custom phrase to stay first, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[1] != "船" || resp.CandidateList[2] != "串" {
		t.Fatalf("expected backend duplicates removed after custom phrase, got %#v", resp.CandidateList)
	}
	for i := 1; i < len(resp.CandidateList); i++ {
		if resp.CandidateList[i] == "传" {
			t.Fatalf("expected duplicated backend candidate removed, got %#v", resp.CandidateList)
		}
	}
}

func TestFillResponseFromBackendStateMatchesCustomPhraseByRawInput(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n娘\tnl\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "niang"
	backend.rawInput = "nl"
	backend.candidates = []candidateItem{
		{Text: "娘"},
		{Text: "酿"},
	}

	resp := imecore.NewResponse(301, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 1 {
		t.Fatalf("expected custom phrase candidate, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "娘" {
		t.Fatalf("expected raw input nl to match custom phrase before backend candidates, got %#v", resp.CandidateList)
	}
}

func TestFillResponseFromBackendStateMatchesCustomPhraseByTrackedRawInput(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n娘\tnl\t10\n")

	ime := newIsolatedTestIME(t)
	ime.rawInputTracked = "nl"
	backend := ime.backend.(*testBackend)
	backend.composition = "niang"
	backend.rawInput = ""
	backend.candidates = []candidateItem{
		{Text: "娘"},
		{Text: "酿"},
	}

	resp := imecore.NewResponse(302, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 1 {
		t.Fatalf("expected custom phrase candidate, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "娘" {
		t.Fatalf("expected tracked raw input nl to match custom phrase before backend candidates, got %#v", resp.CandidateList)
	}
}

func TestFillResponseFromBackendStateShowsSuperAbbrevInFirstComment(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{
		{Text: "发"},
		{Text: "法"},
	}

	resp := imecore.NewResponse(3021, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) < 1 {
		t.Fatalf("expected candidate entries, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "发" || resp.CandidateEntries[0].Comment != "法" {
		t.Fatalf("expected first candidate comment to show super abbrev, got %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateShowsSuperAbbrevOnPrependedCustomPhrase(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n发生\tfa\t10\n")
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{
		{Text: "发"},
		{Text: "法"},
	}

	resp := imecore.NewResponse(30215, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) < 1 {
		t.Fatalf("expected candidate entries, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "发生" || resp.CandidateEntries[0].Comment != "法" {
		t.Fatalf("expected super abbrev comment on prepended custom phrase, got %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateInjectsSuperAbbrevCandidateWhenNoBackendCandidate(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"

	resp := imecore.NewResponse(3022, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) != 1 {
		t.Fatalf("expected synthetic super abbrev candidate, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "法" || resp.CandidateEntries[0].Comment != superAbbrevCommitMark {
		t.Fatalf("unexpected synthetic super abbrev candidate %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateSkipsSuperAbbrevForLuaFilterComposition(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n的\td\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa`d"
	backend.rawInput = "d"
	backend.candidates = []candidateItem{
		{Text: "法"},
		{Text: "珐"},
	}

	resp := imecore.NewResponse(3023, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) < 1 {
		t.Fatalf("expected backend candidates to remain visible, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "法" || resp.CandidateEntries[0].Comment != "" {
		t.Fatalf("expected lua filter composition to skip super abbrev overlay, got %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateSkipsCustomPhraseOverlayForLuaFilterComposition(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n的\td\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa`d"
	backend.rawInput = "d"
	backend.candidates = []candidateItem{
		{Text: "法"},
		{Text: "珐"},
	}

	resp := imecore.NewResponse(303, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 1 {
		t.Fatalf("expected backend candidates to remain visible, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "法" {
		t.Fatalf("expected lua filter candidates to stay ahead of custom phrases, got %#v", resp.CandidateList)
	}
}

func TestSuperAbbrevTabCommitsOverlayText(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{{Text: "发"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  401,
		KeyCode: vkTab,
	}, imecore.NewResponse(401, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected tab intercepted for super abbrev, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  402,
		KeyCode: vkTab,
	}, imecore.NewResponse(402, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected tab commit handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "法" {
		t.Fatalf("expected super abbrev committed by tab, got %q", onResp.CommitString)
	}
	if backend.composition != "" {
		t.Fatalf("expected backend composition cleared after super abbrev commit, got %q", backend.composition)
	}
}

func TestSuperAbbrevPeriodCommitsOverlayText(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{{Text: "发"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   403,
		KeyCode:  vkOemPeriod,
		CharCode: int('.'),
	}, imecore.NewResponse(403, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected period intercepted for super abbrev, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   404,
		KeyCode:  vkOemPeriod,
		CharCode: int('.'),
	}, imecore.NewResponse(404, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected period commit handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "法" {
		t.Fatalf("expected super abbrev committed by period, got %q", onResp.CommitString)
	}
}

func TestSuperAbbrevTabCommitsOverlayTextWhenCustomPhraseIsFirst(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n发生\tfa\t10\n")
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{{Text: "发"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  405,
		KeyCode: vkTab,
	}, imecore.NewResponse(405, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected tab intercepted for super abbrev with custom phrase first, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  406,
		KeyCode: vkTab,
	}, imecore.NewResponse(406, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected tab commit handled with custom phrase first, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "法" {
		t.Fatalf("expected super abbrev committed by tab with custom phrase first, got %q", onResp.CommitString)
	}
}

func TestFillResponseFromBackendStateDoesNotShowCustomPhraseAfterPaging(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.rawInput = "a"
	backend.pageNo = 1
	backend.candidates = []candidateItem{
		{Text: "阿"},
		{Text: "啊"},
	}

	resp := imecore.NewResponse(302, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) != 2 {
		t.Fatalf("expected only backend candidates after paging, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "阿" || resp.CandidateList[1] != "啊" {
		t.Fatalf("expected paged results to exclude custom phrases, got %#v", resp.CandidateList)
	}
}

func TestCustomPhraseSelectionCommitsFirstCustomCandidate(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   31,
		KeyCode:  int('1'),
		CharCode: int('1'),
	}, imecore.NewResponse(31, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected number key intercepted for custom phrase, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   32,
		KeyCode:  int('1'),
		CharCode: int('1'),
	}, imecore.NewResponse(32, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected custom phrase number selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "alpha" {
		t.Fatalf("expected custom phrase committed, got %q", onResp.CommitString)
	}
	if backend.composition != "" {
		t.Fatalf("expected backend composition cleared after custom phrase commit, got %q", backend.composition)
	}
}

func TestCustomPhraseOverlayCanSelectBackendCandidateAfterCustomOnes(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   33,
		KeyCode:  int('2'),
		CharCode: int('2'),
	}, imecore.NewResponse(33, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected second number key intercepted, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   34,
		KeyCode:  int('2'),
		CharCode: int('2'),
	}, imecore.NewResponse(34, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected backend overlay candidate selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "阿" {
		t.Fatalf("expected backend first candidate committed after custom phrase slot, got %q", onResp.CommitString)
	}
}

func TestCustomPhraseOverlayNumberSelectionMatchesVisibleDeduplicatedCandidates(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n传\tc\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "c"
	backend.rawInput = "c"
	backend.candidates = []candidateItem{
		{Text: "传"},
		{Text: "船"},
		{Text: "串"},
	}

	showResp := imecore.NewResponse(341, true)
	ime.fillResponseFromBackendState(showResp, false)
	if len(showResp.CandidateList) != 3 {
		t.Fatalf("expected visible candidates to be deduplicated to 3 entries, got %#v", showResp.CandidateList)
	}
	if showResp.CandidateList[0] != "传" || showResp.CandidateList[1] != "船" || showResp.CandidateList[2] != "串" {
		t.Fatalf("expected visible candidates [传 船 串], got %#v", showResp.CandidateList)
	}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   342,
		KeyCode:  int('2'),
		CharCode: int('2'),
	}, imecore.NewResponse(342, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected second number key intercepted for deduplicated overlay, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   343,
		KeyCode:  int('2'),
		CharCode: int('2'),
	}, imecore.NewResponse(343, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected deduplicated overlay selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "船" {
		t.Fatalf("expected visible second candidate 船 to be committed, got %q", onResp.CommitString)
	}
}

func TestCustomPhraseOverlayDoesNotInterceptEnter(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  35,
		KeyCode: vkReturn,
	}, imecore.NewResponse(35, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected enter key handled by backend flow, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  36,
		KeyCode: vkReturn,
	}, imecore.NewResponse(36, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected enter keydown handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "阿" {
		t.Fatalf("expected enter to commit backend first candidate 阿, got %q", onResp.CommitString)
	}
}

func TestCustomPhraseOverlaySemicolonSelectsSecondVisibleCandidate(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   35,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(35, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon key intercepted for custom phrase overlay, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   36,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(36, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon overlay selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "阿" {
		t.Fatalf("expected semicolon to select second visible candidate 阿, got %q", onResp.CommitString)
	}
}

func TestCustomPhraseOverlaySemicolonSelectionMatchesVisibleDeduplicatedCandidates(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n传\tc\t10\n")

	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "c"
	backend.rawInput = "c"
	backend.candidates = []candidateItem{
		{Text: "传"},
		{Text: "船"},
		{Text: "串"},
	}

	showResp := imecore.NewResponse(361, true)
	ime.fillResponseFromBackendState(showResp, false)
	if len(showResp.CandidateList) != 3 {
		t.Fatalf("expected visible candidates to be deduplicated to 3 entries, got %#v", showResp.CandidateList)
	}
	if showResp.CandidateList[0] != "传" || showResp.CandidateList[1] != "船" || showResp.CandidateList[2] != "串" {
		t.Fatalf("expected visible candidates [传 船 串], got %#v", showResp.CandidateList)
	}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   362,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(362, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon key intercepted for deduplicated overlay, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   363,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(363, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon deduplicated overlay selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "船" {
		t.Fatalf("expected semicolon to select visible second candidate 船, got %q", onResp.CommitString)
	}
}

func TestFillResponseFromBackendStateAppliesCandidateCount(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidateCount = 5
	backend := ime.backend.(*testBackend)
	backend.pageSize = 5
	backend.composition = "ni"
	backend.candidates = []candidateItem{
		{Text: "你", Comment: "pron"},
		{Text: "呢"},
		{Text: "泥"},
		{Text: "尼"},
		{Text: "拟"},
		{Text: "逆"},
	}

	resp := imecore.NewResponse(19, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) != 5 {
		t.Fatalf("expected 5 candidates after truncation, got %#v", resp.CandidateList)
	}
	if len(resp.CandidateEntries) != 5 {
		t.Fatalf("expected 5 candidate entries after truncation, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "你" || resp.CandidateEntries[0].Comment != "pron" {
		t.Fatalf("expected first candidate entry to preserve comment, got %#v", resp.CandidateEntries[0])
	}
	if resp.SetSelKeys != "12345" {
		t.Fatalf("expected select keys truncated to 12345, got %q", resp.SetSelKeys)
	}
}

func TestFillResponseFromBackendStateAllowsSchemaPageSizeAboveSettingsRange(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidateCount = 9
	backend := ime.backend.(*testBackend)
	backend.pageSize = 12
	backend.selectKeys = "1234567890ab"
	backend.composition = "hou"
	for i := 0; i < 12; i++ {
		backend.candidates = append(backend.candidates, candidateItem{
			Text:    fmt.Sprintf("候%d", i+1),
			Comment: fmt.Sprintf("code%d", i+1),
		})
	}

	resp := imecore.NewResponse(119, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) != 12 {
		t.Fatalf("expected schema page size to display 12 candidates, got %#v", resp.CandidateList)
	}
	if len(resp.CandidateEntries) != 12 {
		t.Fatalf("expected 12 candidate entries, got %#v", resp.CandidateEntries)
	}
	if resp.TypeDuckCandidatePage == nil || resp.TypeDuckCandidatePage.PageSize != 12 {
		t.Fatalf("expected page metadata size 12, got %#v", resp.TypeDuckCandidatePage)
	}
	if resp.SetSelKeys != "1234567890ab" {
		t.Fatalf("expected select keys to preserve 12 keys, got %q", resp.SetSelKeys)
	}
}

func TestFillResponseFromBackendStateRestoresDefaultSelectKeysWhenBackendOmitsThem(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.selectKeys = "123"
	backend := ime.backend.(*testBackend)
	backend.selectKeys = ""
	backend.composition = "w"
	backend.candidates = []candidateItem{
		{Text: "未"},
		{Text: "完"},
		{Text: "万"},
		{Text: "我"},
		{Text: "为"},
		{Text: "玩"},
		{Text: "问"},
		{Text: "外"},
	}

	resp := imecore.NewResponse(20, true)
	ime.fillResponseFromBackendState(resp, false)

	if resp.SetSelKeys != "12345678" {
		t.Fatalf("expected default select keys restored to candidate count, got %q", resp.SetSelKeys)
	}
	if ime.selectKeys != "12345678" {
		t.Fatalf("expected cached select keys restored, got %q", ime.selectKeys)
	}
}

func TestHandleRequestCompositionTerminatedPreservesBackendStateWhenNotForced(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 13,
		Method: "onCompositionTerminated",
	})

	if !resp.Success {
		t.Fatal("expected composition termination response to succeed")
	}
	if backend.composition != "ni" {
		t.Fatalf("expected non-forced termination to preserve backend composition, got %q", backend.composition)
	}
	if len(backend.candidates) == 0 {
		t.Fatal("expected non-forced termination to preserve backend candidates")
	}
}

func TestHandleRequestCompositionTerminatedForcedResetsState(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 13,
		Method: "onCompositionTerminated",
		Forced: true,
	})

	if !resp.Success {
		t.Fatal("expected forced composition termination response to succeed")
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected forced composition termination to reset backend state")
	}
}

func TestHandleRequestOnDeactivateReturnsHandled(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 14,
		Method: "onDeactivate",
	})

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onDeactivate to return 1, got %d", resp.ReturnValue)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected deactivate to clear composition state")
	}
}

func TestHandleRequestSyncsAppearanceAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	second := newTestIME()

	if !first.applyAppearanceCommand(themeCommandForTheme(t, "purple")) {
		t.Fatal("expected theme command handled")
	}

	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 15,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})

	if resp.ReturnValue != 0 {
		t.Fatalf("expected v1 onMenu to stay unreachable, got %d", resp.ReturnValue)
	}
	if second.style.CandidateTheme != "purple" {
		t.Fatalf("expected second instance theme synced to purple, got %q", second.style.CandidateTheme)
	}
	if second.style.CandidateBackgroundColor != "#f3e8ff" {
		t.Fatalf("expected synced background color, got %q", second.style.CandidateBackgroundColor)
	}
}

func TestHandleRequestSyncsSharedInputStateAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	first.inputStateShared = true
	first.backend.SetOption("ascii_mode", true)
	first.backend.SetOption("ascii_punct", true)
	first.backend.SetOption("traditionalization", true)
	first.backend.SetOption("full_shape", true)
	first.captureSharedInputStateFromBackend()
	first.saveAppearancePrefs()

	second := newTestIME()
	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 16,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})

	if resp.ReturnValue != 0 {
		t.Fatalf("expected v1 onMenu to stay unreachable, got %d", resp.ReturnValue)
	}
	if !second.inputStateShared {
		t.Fatal("expected second instance to enable shared input state")
	}
	second.createSession(nil)
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode synced to enabled")
	}
	if !second.backend.GetOption("ascii_punct") {
		t.Fatal("expected ascii_punct synced to enabled")
	}
	if !second.backend.GetOption("traditionalization") {
		t.Fatal("expected traditionalization synced to enabled")
	}
	if !second.backend.GetOption("full_shape") {
		t.Fatal("expected full_shape synced to enabled")
	}
}

func TestHandleRequestSyncsDynamicSharedOptionsAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	first.inputStateShared = true
	firstBackend := first.backend.(*testBackend)
	firstBackend.saveOptions = []string{"emoji", "ascii_mode"}
	firstBackend.schemaSwitches[firstBackend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}
	first.backend.SetOption("emoji", true)
	first.backend.SetOption("ascii_mode", true)
	first.captureSharedInputStateFromBackend()
	first.captureSyncedOptionsFromBackend()
	first.saveAppearancePrefs()

	second := newTestIME()
	secondBackend := second.backend.(*testBackend)
	secondBackend.saveOptions = []string{"emoji", "ascii_mode"}
	secondBackend.schemaSwitches[secondBackend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}

	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 17,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})

	if resp.ReturnValue != 0 {
		t.Fatalf("expected v1 onMenu to stay unreachable, got %d", resp.ReturnValue)
	}
	second.createSession(nil)
	if !second.backend.GetOption("emoji") {
		t.Fatal("expected dynamic emoji option synced to enabled")
	}
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode synced to enabled")
	}
}

func TestAlwaysSyncedSwitcherOptionsIgnoreInputStateSharedToggle(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	firstBackend := first.backend.(*testBackend)
	firstBackend.saveOptions = []string{"emoji", "ascii_mode"}
	firstBackend.schemaSwitches[firstBackend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}
	first.inputStateShared = false
	firstResp := first.onCommand(&imecore.Request{
		SeqNum: 171,
		ID:     imecore.FlexibleID{Int: switchCommandID(0), IsInt: true},
	}, imecore.NewResponse(171, true))
	if firstResp.ReturnValue != 1 {
		t.Fatalf("expected dynamic switch command handled, got %d", firstResp.ReturnValue)
	}
	first.backend.SetOption("ascii_mode", true)
	first.captureSharedInputStateFromBackend()
	first.saveAppearancePrefs()

	second := newTestIME()
	secondBackend := second.backend.(*testBackend)
	secondBackend.saveOptions = []string{"emoji", "ascii_mode"}
	secondBackend.schemaSwitches[secondBackend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}

	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 171,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})
	if resp.ReturnValue != 0 {
		t.Fatalf("expected v1 onMenu to stay unreachable, got %d", resp.ReturnValue)
	}
	if !second.backend.GetOption("emoji") {
		t.Fatal("expected non-input-state emoji option synced even with input state sharing disabled")
	}
	if second.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode not synced while input state sharing is disabled")
	}
}

func TestHandleRequestSyncsSelectedSchemaAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	resp := first.onCommand(&imecore.Request{
		SeqNum: 172,
		ID:     imecore.FlexibleID{Int: schemaCommandID(1), IsInt: true},
	}, imecore.NewResponse(172, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected schema command handled, got %d", resp.ReturnValue)
	}

	second := newTestIME()
	secondResp := second.HandleRequest(&imecore.Request{
		SeqNum: 173,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})
	if secondResp.ReturnValue != 0 {
		t.Fatalf("expected v1 onMenu to stay unreachable, got %d", secondResp.ReturnValue)
	}
	secondBackend := second.backend.(*testBackend)
	if secondBackend.currentSchemaID != "rime_frost_double_pinyin" {
		t.Fatalf("expected schema synced to double pinyin, got %q", secondBackend.currentSchemaID)
	}
}

func TestHandleRequestIgnoresSyncedSchemaMissingFromCurrentSchemeSet(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	backend := ime.backend.(*testBackend)
	backend.currentSchemaID = "missing_schema"
	ime.syncedSchemaID = "old_scheme_schema"

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 174,
		Method: "onActivate",
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected activation handled, got %d", resp.ReturnValue)
	}
	if backend.currentSchemaID != "rime_frost" {
		t.Fatalf("expected fallback to first available schema, got %q", backend.currentSchemaID)
	}
	if ime.syncedSchemaID != "rime_frost" {
		t.Fatalf("expected synced schema updated to fallback, got %q", ime.syncedSchemaID)
	}
	for _, schemaID := range backend.selectSchemaCalls {
		if schemaID == "old_scheme_schema" {
			t.Fatalf("should not select schema missing from current scheme set, calls=%#v", backend.selectSchemaCalls)
		}
	}
}

func TestHandleRequestUsesSchemaRecordedForCurrentSchemeSet(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}
	if !saveCurrentSchemeSetName("Work") {
		t.Fatal("save current scheme set")
	}

	ime := newTestIME()
	backend := ime.backend.(*testBackend)
	backend.schemas = []RimeSchema{
		{ID: "work_default", Name: "Work Default"},
		{ID: "work_double", Name: "Work Double"},
	}
	backend.currentSchemaID = "work_default"
	ime.syncedSchemaID = "rime_frost_double_pinyin"
	ime.syncedSchemaBySchemeSet = map[string]string{
		defaultSchemeSetName: "rime_frost_double_pinyin",
		"Work":               "work_double",
	}

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 175,
		Method: "onActivate",
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected activation handled, got %d", resp.ReturnValue)
	}
	if backend.currentSchemaID != "work_double" {
		t.Fatalf("expected Work scheme set schema selected, got %q", backend.currentSchemaID)
	}
	if len(backend.selectSchemaCalls) != 1 || backend.selectSchemaCalls[0] != "work_double" {
		t.Fatalf("unexpected select schema calls: %#v", backend.selectSchemaCalls)
	}
}

func TestProcessKeySyncsSharedInputStateAfterShiftAndCapsToggle(t *testing.T) {
	testCases := []struct {
		name     string
		toggle   func(t *testing.T, ime *IME)
		expected bool
	}{
		{
			// Bare Shift is intercepted by the toggle state machine before
			// processKey; drive the filter events instead.
			name: "shift",
			toggle: func(t *testing.T, ime *IME) {
				t.Helper()
				ime.filterKeyDown(shiftKeyRequest(180, false, false), imecore.NewResponse(180, true))
				resp := ime.filterKeyUp(shiftKeyRequest(181, false, false), imecore.NewResponse(181, true))
				if resp.ReturnValue != 1 {
					t.Fatalf("expected toggling Shift-up eaten, got %d", resp.ReturnValue)
				}
			},
			expected: true,
		},
		{
			name: "caps",
			toggle: func(t *testing.T, ime *IME) {
				t.Helper()
				ime.processKey(&imecore.Request{
					KeyCode:   vkCapital,
					KeyStates: make(imecore.KeyStates, 256),
				}, false)
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			resetSharedAppearanceConfigForTest()

			first := newTestIME()
			first.inputStateShared = true
			first.captureSharedInputStateFromBackend()
			first.saveAppearancePrefs()

			tc.toggle(t, first)
			if got := first.backend.GetOption("ascii_mode"); got != tc.expected {
				t.Fatalf("expected first instance ascii_mode=%t after %s toggle, got %t", tc.expected, tc.name, got)
			}

			second := newTestIME()
			resp := second.HandleRequest(&imecore.Request{
				SeqNum: 18,
				Method: "onMenu",
				ID:     imecore.FlexibleID{String: "settings"},
			})
			if resp.ReturnValue != 0 {
				t.Fatalf("expected v1 onMenu to stay unreachable, got %d", resp.ReturnValue)
			}
			second.createSession(nil)

			if got := second.backend.GetOption("ascii_mode"); got != tc.expected {
				t.Fatalf("expected second instance ascii_mode=%t after %s shared sync, got %t", tc.expected, tc.name, got)
			}
		})
	}
}

func TestCreateSessionAppliesSharedInputStateOnlyForNewSession(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	ime.inputStateShared = true
	ime.sharedOptions = map[string]bool{
		"ascii_mode":  true,
		"full_shape":  true,
		"ascii_punct": true,
	}
	backend := ime.backend.(*testBackend)

	ime.createSession(nil)
	firstApplyCalls := backend.setOptionCalls
	if firstApplyCalls != len(ime.sharedOptions) {
		t.Fatalf("expected first createSession to apply %d shared options, got %d", len(ime.sharedOptions), firstApplyCalls)
	}

	ime.createSession(nil)
	if backend.setOptionCalls != firstApplyCalls {
		t.Fatalf("expected repeated createSession to skip shared apply, got %d total SetOption calls", backend.setOptionCalls)
	}
}

func TestProcessKeySkipsSharedStateSyncForRegularTyping(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	ime.inputStateShared = true
	ime.sharedOptions = map[string]bool{
		"ascii_mode": false,
	}
	backend := ime.backend.(*testBackend)

	ime.createSession(nil)
	backend.setOptionCalls = 0
	backend.getOptionCalls = 0

	handled := ime.processKey(&imecore.Request{
		KeyCode:   'N',
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	}, false)
	if !handled {
		t.Fatal("expected regular typing key to be handled")
	}
	if backend.setOptionCalls != 0 {
		t.Fatalf("expected regular typing to avoid shared SetOption calls, got %d", backend.setOptionCalls)
	}
	if backend.getOptionCalls != 0 {
		t.Fatalf("expected regular typing to avoid shared GetOption calls, got %d", backend.getOptionCalls)
	}
}

func TestCreateSessionAppliesSharedInputStateAfterSharedConfigUpdateWithExistingSession(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	second := newTestIME()
	second.inputStateShared = true
	second.createSession(nil)
	secondBackend := second.backend.(*testBackend)
	secondBackend.setOptionCalls = 0

	first := newTestIME()
	first.inputStateShared = true
	first.backend.SetOption("ascii_mode", true)
	first.captureSharedInputStateFromBackend()
	first.saveAppearancePrefs()

	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 19,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})
	if resp.ReturnValue != 0 {
		t.Fatalf("expected v1 onMenu to stay unreachable, got %d", resp.ReturnValue)
	}
	second.createSession(nil)
	if second.sharedInputStateNeedsApply {
		t.Fatal("expected shared input state apply marker cleared after createSession")
	}
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected existing session to receive updated ascii_mode")
	}
	if secondBackend.setOptionCalls == 0 {
		t.Fatal("expected createSession to reapply shared state for existing session after config update")
	}
}

func TestLoadAppearancePrefsKeepsPresetThemeAfterPersist(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	if !first.applyAppearanceCommand(themeCommandForTheme(t, "purple")) {
		t.Fatal("expected theme command handled")
	}

	resetSharedAppearanceConfigForTest()
	second := newTestIME()
	second.loadAppearancePrefs()

	if second.style.CandidateTheme != "purple" {
		t.Fatalf("expected persisted preset theme purple, got %q", second.style.CandidateTheme)
	}
	if second.style.CandidateBackgroundColor != "#f3e8ff" {
		t.Fatalf("expected persisted preset background color, got %q", second.style.CandidateBackgroundColor)
	}
}

func TestAppearanceSettingsPersistToDisk(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	if !ime.applyAppearanceCommand(ID_APPEARANCE_FONT_FAMILY_YAHEI_UI) {
		t.Fatal("expected candidate font family command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_COMMENT_FONT_FAMILY_DENGXIAN) {
		t.Fatal("expected comment font family command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_FONT_30) {
		t.Fatal("expected font size command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_COMMENT_FONT_28) {
		t.Fatal("expected comment font size command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_LAYOUT_HORIZONTAL) {
		t.Fatal("expected layout command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_PER_ROW_7) {
		t.Fatal("expected per-row command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_SPACING_30) {
		t.Fatal("expected spacing command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_CAND_COUNT_5) {
		t.Fatal("expected candidate count command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_BG_BLUE) {
		t.Fatal("expected background color command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_HL_GREEN) {
		t.Fatal("expected highlight color command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_TEXT_BLUE) {
		t.Fatal("expected text color command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_HLTEXT_WHITE) {
		t.Fatal("expected highlight text color command handled")
	}
	ime.autoPairQuotes = true
	ime.semicolonSelectSecond = true
	ime.saveAppearancePrefs()

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected appearance config written to disk: %v", err)
	}

	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("expected valid appearance config json: %v", err)
	}
	if got := persisted["font_point"]; got != float64(30) {
		t.Fatalf("expected persisted font_point 30, got %#v", got)
	}
	if got := persisted["font_face"]; got != "Microsoft YaHei UI" {
		t.Fatalf("expected persisted font_face Microsoft YaHei UI, got %#v", got)
	}
	if got := persisted["candidate_comment_font_face"]; got != "DengXian" {
		t.Fatalf("expected persisted candidate_comment_font_face DengXian, got %#v", got)
	}
	if got := persisted["candidate_comment_font_point"]; got != float64(28) {
		t.Fatalf("expected persisted candidate_comment_font_point 28, got %#v", got)
	}
	if got := persisted["candidate_per_row"]; got != float64(7) {
		t.Fatalf("expected persisted candidate_per_row 7, got %#v", got)
	}
	if got := persisted["candidate_count"]; got != float64(5) {
		t.Fatalf("expected persisted candidate_count 5, got %#v", got)
	}
	if got := persisted["candidate_spacing"]; got != float64(30) {
		t.Fatalf("expected persisted candidate_spacing 30, got %#v", got)
	}
	if got := persisted["candidate_background_color"]; got != "#f3f8ff" {
		t.Fatalf("expected persisted background color, got %#v", got)
	}
	if got := persisted["candidate_highlight_color"]; got != "#d9f2e6" {
		t.Fatalf("expected persisted highlight color, got %#v", got)
	}
	if got := persisted["candidate_text_color"]; got != "#1d4ed8" {
		t.Fatalf("expected persisted text color, got %#v", got)
	}
	if got := persisted["candidate_highlight_text_color"]; got != "#ffffff" {
		t.Fatalf("expected persisted highlight text color, got %#v", got)
	}
	if got := persisted["candidate_comment_color"]; got != "#1d4ed8" {
		t.Fatalf("expected persisted comment color, got %#v", got)
	}
	if got := persisted["candidate_comment_highlight_color"]; got != "#ffffff" {
		t.Fatalf("expected persisted comment highlight color, got %#v", got)
	}
	if got := persisted["auto_pair_quotes"]; got != true {
		t.Fatalf("expected persisted auto_pair_quotes true, got %#v", got)
	}
	if got := persisted["semicolon_select_second"]; got != true {
		t.Fatalf("expected persisted semicolon_select_second true, got %#v", got)
	}

	resetSharedAppearanceConfigForTest()
	reloaded := newTestIME()
	reloaded.loadAppearancePrefs()

	if reloaded.style.FontPoint != 30 {
		t.Fatalf("expected reloaded font size 30, got %d", reloaded.style.FontPoint)
	}
	if reloaded.style.FontFace != "Microsoft YaHei UI" {
		t.Fatalf("expected reloaded font face Microsoft YaHei UI, got %q", reloaded.style.FontFace)
	}
	if reloaded.style.CandidateCommentFontPoint != 28 {
		t.Fatalf("expected reloaded comment font size 28, got %d", reloaded.style.CandidateCommentFontPoint)
	}
	if reloaded.style.CandidateCommentFontFace != "DengXian" {
		t.Fatalf("expected reloaded comment font face DengXian, got %q", reloaded.style.CandidateCommentFontFace)
	}
	if reloaded.style.CandidatePerRow != 7 {
		t.Fatalf("expected reloaded per-row 7, got %d", reloaded.style.CandidatePerRow)
	}
	if reloaded.style.CandidateCount != 5 {
		t.Fatalf("expected reloaded candidate count 5, got %d", reloaded.style.CandidateCount)
	}
	if reloaded.style.CandidateSpacing != 30 {
		t.Fatalf("expected reloaded candidate spacing 30, got %d", reloaded.style.CandidateSpacing)
	}
	if reloaded.style.CandidateBackgroundColor != "#f3f8ff" {
		t.Fatalf("expected reloaded background color, got %q", reloaded.style.CandidateBackgroundColor)
	}
	if reloaded.style.CandidateHighlightColor != "#d9f2e6" {
		t.Fatalf("expected reloaded highlight color, got %q", reloaded.style.CandidateHighlightColor)
	}
	if reloaded.style.CandidateTextColor != "#1d4ed8" {
		t.Fatalf("expected reloaded text color, got %q", reloaded.style.CandidateTextColor)
	}
	if reloaded.style.CandidateHighlightTextColor != "#ffffff" {
		t.Fatalf("expected reloaded highlight text color, got %q", reloaded.style.CandidateHighlightTextColor)
	}
	if reloaded.style.CandidateCommentColor != "#1d4ed8" {
		t.Fatalf("expected reloaded comment color, got %q", reloaded.style.CandidateCommentColor)
	}
	if reloaded.style.CandidateCommentHighlightColor != "#ffffff" {
		t.Fatalf("expected reloaded comment highlight color, got %q", reloaded.style.CandidateCommentHighlightColor)
	}
	if reloaded.style.CandidateTheme != "custom" {
		t.Fatalf("expected reloaded theme custom for custom colors, got %q", reloaded.style.CandidateTheme)
	}
	if !reloaded.autoPairQuotes {
		t.Fatal("expected reloaded auto pair quotes enabled")
	}
	if !reloaded.semicolonSelectSecond {
		t.Fatal("expected reloaded semicolon select second enabled")
	}
}

func TestLoadAppearancePrefsDoesNotCreateAppDataConfig(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	ime.loadAppearancePrefs()

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected no AppData appearance config, stat err=%v", err)
	}
	if ime.style.CandidatePerRow != 1 {
		t.Fatalf("expected in-memory style to stay vertical by default, got %d", ime.style.CandidatePerRow)
	}
	if ime.inputStateShared {
		t.Fatal("expected shared input state disabled by default")
	}
}

func TestCustomPhraseLookupDoesNotCreateAppDataFile(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()

	got := lookupCustomPhraseCandidates("aa", 5)
	if len(got) != 0 {
		t.Fatalf("expected no custom phrase candidates for v1, got %#v", got)
	}

	configPath := filepath.Join(appData, APP, customPhraseFileName)
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected no AppData custom phrase file, stat err=%v", err)
	}
}

func TestRimeLogDirUsesTypeDuckLogUnderLocalAppData(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)

	got := rimeLogDir()
	want := filepath.Join(localAppData, "TypeDuckIME", "Log")
	if got != want {
		t.Fatalf("expected rime log dir %q, got %q", want, got)
	}
}

func TestRimeLogDirReturnsEmptyWithoutLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")

	if got := rimeLogDir(); got != "" {
		t.Fatalf("expected empty rime log dir when LOCALAPPDATA unset, got %q", got)
	}
}

func TestNativeBackendRedeployRunsAsync(t *testing.T) {
	resetNativeRuntimeStateForTest()
	oldRedeployFunc := rimeRedeployFunc
	oldInitOK := rimeInitOK
	rimeInitOK = true
	done := make(chan struct{})
	rimeRedeployFunc = func(datadir, userdir, appname, appver string) bool {
		time.Sleep(120 * time.Millisecond)
		close(done)
		return true
	}
	defer func() {
		rimeRedeployFunc = oldRedeployFunc
		rimeInitOK = oldInitOK
		resetNativeRuntimeStateForTest()
	}()

	backend := &nativeBackend{}
	start := time.Now()
	if !backend.Redeploy("shared", "user") {
		t.Fatal("expected redeploy to start")
	}
	if elapsed := time.Since(start); elapsed > 60*time.Millisecond {
		t.Fatalf("expected async redeploy to return quickly, took %s", elapsed)
	}
	if backend.Available() {
		t.Fatal("expected backend unavailable while redeploy is in progress")
	}
	if notification := backend.ConsumeNotification(); notification != nil {
		t.Fatalf("expected no notification before async redeploy completes, got %#v", notification)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async redeploy to finish")
	}

	if !backend.Available() {
		t.Fatal("expected backend available after redeploy completes")
	}
	notification := backend.ConsumeNotification()
	if notification == nil {
		t.Fatal("expected completion notification after async redeploy")
	}
	if notification.Message != "Redeploy succeeded" {
		t.Fatalf("unexpected completion notification: %#v", notification)
	}
	if backend.ConsumeNotification() != nil {
		t.Fatal("expected completion notification to be consumed once")
	}
}

func TestCompositionCaretRuneIndex(t *testing.T) {
	if got := compositionCaretRuneIndex("nihao", 2); got != 2 {
		t.Fatalf("ASCII caret: got %d want 2", got)
	}
	const mixed = "你hao"
	byteAfterNi := len("你")
	if got := compositionCaretRuneIndex(mixed, byteAfterNi); got != 1 {
		t.Fatalf("UTF-8 caret after CJK: got %d want 1", got)
	}
	if got := compositionCaretRuneIndex(mixed, len(mixed)); got != 4 {
		t.Fatalf("end caret: got %d want 4", got)
	}
}

func TestCompositionSelectionUsesRuneIndexes(t *testing.T) {
	const mixed = "〔倉頡五代〕onf"
	byteAfterPrefix := len("〔倉頡五代〕")
	if got := compositionCaretRuneIndex(mixed, byteAfterPrefix); got != 6 {
		t.Fatalf("selection start after CJK prefix: got %d want 6", got)
	}
	if got := compositionCaretRuneIndex(mixed, len(mixed)); got != 9 {
		t.Fatalf("selection end after ASCII lookup code: got %d want 9", got)
	}
}

func TestTypeDuckRimeModulesIncludeDictionaryLookup(t *testing.T) {
	modules := typeDuckRimeModules()
	if !slices.Contains(modules, "default") {
		t.Fatalf("expected default module group, got %#v", modules)
	}
	if !slices.Contains(modules, "dictionary_lookup") {
		t.Fatalf("expected dictionary_lookup module for dictionary_lookup_filter, got %#v", modules)
	}
}

func TestBareShiftDownPassesThroughAndIsNotForwardedToRime(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.filterKeyDown(shiftKeyRequest(300, false, false), imecore.NewResponse(300, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected bare Shift-down to pass through, got %d", resp.ReturnValue)
	}
	for _, code := range backend.translatedKeyCodes {
		if code == rimeShiftL || code == rimeShiftR {
			t.Fatalf("expected bare Shift-down not forwarded to rime, got %v", backend.translatedKeyCodes)
		}
	}
	if !ime.shiftTogglePending {
		t.Fatal("expected bare Shift-down to arm the toggle")
	}
}

func TestLoneLeftShiftTogglesAsciiMode(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.inputStateShared = true

	iconDir := t.TempDir()
	for _, name := range []string{"chi.ico", "eng.ico"} {
		if err := os.WriteFile(filepath.Join(iconDir, name), []byte{}, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	ime.iconDir = iconDir

	downResp := ime.filterKeyDown(shiftKeyRequest(301, false, false), imecore.NewResponse(301, true))
	if downResp.ReturnValue != 0 {
		t.Fatalf("expected Shift-down to pass through, got %d", downResp.ReturnValue)
	}

	upResp := ime.filterKeyUp(shiftKeyRequest(302, false, false), imecore.NewResponse(302, true))
	if upResp.ReturnValue != 1 {
		t.Fatalf("expected toggling Shift-up eaten, got %d", upResp.ReturnValue)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode enabled after lone Shift")
	}
	if upResp.TypeDuckSettingsUpdate == nil || upResp.TypeDuckSettingsUpdate.AsciiMode == nil || !*upResp.TypeDuckSettingsUpdate.AsciiMode {
		t.Fatalf("expected settings update with ascii_mode=true, got %#v", upResp.TypeDuckSettingsUpdate)
	}
	langIcon := ""
	for _, btn := range upResp.ChangeButton {
		if btn.ID == "switch-lang" {
			langIcon = filepath.Base(btn.Icon)
		}
	}
	if langIcon != "eng.ico" {
		t.Fatalf("expected eng.ico language button update, got %q (%#v)", langIcon, upResp.ChangeButton)
	}

	onResp := ime.onKeyUp(shiftKeyRequest(303, false, false), imecore.NewResponse(303, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected onKeyUp to eat the toggling Shift-up, got %d", onResp.ReturnValue)
	}
	if onResp.CompositionString != "" || len(onResp.CandidateList) != 0 || onResp.ShowCandidates {
		t.Fatalf("expected empty composition and candidates, got %#v", onResp)
	}
}

func TestLoneRightShiftTogglesAsciiMode(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.inputStateShared = true

	down := shiftKeyRequest(304, false, false)
	down.KeyStates[vkRShift] |= 1
	if resp := ime.filterKeyDown(down, imecore.NewResponse(304, true)); resp.ReturnValue != 0 {
		t.Fatalf("expected right Shift-down to pass through, got %d", resp.ReturnValue)
	}

	up := shiftKeyRequest(305, false, false)
	up.KeyStates[vkRShift] |= 1
	upResp := ime.filterKeyUp(up, imecore.NewResponse(305, true))
	if upResp.ReturnValue != 1 {
		t.Fatalf("expected right Shift-up eaten, got %d", upResp.ReturnValue)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode enabled after lone right Shift")
	}

	onUp := shiftKeyRequest(306, false, false)
	onUp.KeyStates[vkRShift] |= 1
	if resp := ime.onKeyUp(onUp, imecore.NewResponse(306, true)); resp.ReturnValue != 1 {
		t.Fatalf("expected onKeyUp eaten for right Shift, got %d", resp.ReturnValue)
	}
}

func TestShiftWithLetterDoesNotToggle(t *testing.T) {
	ime := newIsolatedTestIME(t)

	ime.filterKeyDown(shiftKeyRequest(310, false, false), imecore.NewResponse(310, true))

	letterStates := make(imecore.KeyStates, 256)
	letterStates[vkShift] |= 0x80
	letterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:    311,
		KeyCode:   int('N'),
		CharCode:  int('N'),
		KeyStates: letterStates,
	}, imecore.NewResponse(311, true))
	if letterResp.ReturnValue != 0 {
		t.Fatalf("expected uppercase letter to pass through in Chinese mode, got %d", letterResp.ReturnValue)
	}

	upResp := ime.filterKeyUp(shiftKeyRequest(312, false, false), imecore.NewResponse(312, true))
	if upResp.ReturnValue != 0 {
		t.Fatalf("expected cancelled Shift-up to pass through, got %d", upResp.ReturnValue)
	}
	if ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode unchanged after Shift+letter")
	}
	if upResp.TypeDuckSettingsUpdate != nil {
		t.Fatalf("expected no settings update on cancelled Shift, got %#v", upResp.TypeDuckSettingsUpdate)
	}
}

func TestCtrlShiftDoesNotToggle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	downResp := ime.filterKeyDown(shiftKeyRequest(315, true, false), imecore.NewResponse(315, true))
	upResp := ime.filterKeyUp(shiftKeyRequest(316, true, false), imecore.NewResponse(316, true))

	if ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode unchanged after Ctrl+Shift")
	}
	if !slices.Contains(backend.translatedKeyCodes, rimeShiftL) {
		t.Fatalf("expected chorded Shift forwarded to rime, got %v", backend.translatedKeyCodes)
	}
	if downResp.TypeDuckSettingsUpdate != nil || upResp.TypeDuckSettingsUpdate != nil {
		t.Fatal("expected no settings update on Ctrl+Shift chord")
	}
}

func TestShiftAutoRepeatTogglesOnce(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.inputStateShared = true

	for seq := 320; seq <= 322; seq++ {
		ime.filterKeyDown(shiftKeyRequest(seq, false, false), imecore.NewResponse(seq, true))
	}

	upResp := ime.filterKeyUp(shiftKeyRequest(323, false, false), imecore.NewResponse(323, true))
	if upResp.ReturnValue != 1 {
		t.Fatalf("expected Shift-up after auto-repeat downs to toggle once, got %d", upResp.ReturnValue)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode enabled")
	}
	if onResp := ime.onKeyUp(shiftKeyRequest(324, false, false), imecore.NewResponse(324, true)); onResp.ReturnValue != 1 {
		t.Fatalf("expected onKeyUp eaten, got %d", onResp.ReturnValue)
	}

	secondUp := ime.filterKeyUp(shiftKeyRequest(325, false, false), imecore.NewResponse(325, true))
	if secondUp.ReturnValue != 0 {
		t.Fatalf("expected second Shift-up without a down to pass through, got %d", secondUp.ReturnValue)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected second Shift-up not to toggle back")
	}
}

func TestShiftToggleWithCompositionCommitsRawInput(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.inputStateShared = true
	backend := ime.backend.(*testBackend)

	typeTestLetters(t, ime, 330, "nihao")
	backend.rawInput = "nihao"
	if len(backend.candidates) == 0 || backend.candidates[0].Text != "你好" {
		t.Fatalf("expected 你好 highlighted before toggling, got %#v", backend.candidates)
	}

	ime.filterKeyDown(shiftKeyRequest(340, false, false), imecore.NewResponse(340, true))
	upResp := ime.filterKeyUp(shiftKeyRequest(341, false, false), imecore.NewResponse(341, true))
	if upResp.ReturnValue != 1 {
		t.Fatalf("expected toggling Shift-up eaten, got %d", upResp.ReturnValue)
	}

	onResp := ime.onKeyUp(shiftKeyRequest(342, false, false), imecore.NewResponse(342, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected onKeyUp eaten, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "nihao" {
		t.Fatalf("expected raw letters committed, got %q", onResp.CommitString)
	}
	if onResp.CompositionString != "" || len(onResp.CandidateList) != 0 || onResp.ShowCandidates {
		t.Fatalf("expected cleared composition and candidates, got %#v", onResp)
	}
	if backend.composition != "" {
		t.Fatalf("expected backend composition cleared, got %q", backend.composition)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode enabled")
	}
}

func TestShiftToggleBackToChineseAllowsComposing(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.inputStateShared = true
	ime.backend.SetOption("ascii_mode", true)

	ime.filterKeyDown(shiftKeyRequest(350, false, false), imecore.NewResponse(350, true))
	upResp := ime.filterKeyUp(shiftKeyRequest(351, false, false), imecore.NewResponse(351, true))
	if upResp.ReturnValue != 1 {
		t.Fatalf("expected toggling Shift-up eaten, got %d", upResp.ReturnValue)
	}
	if ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode disabled after toggle back")
	}
	ime.onKeyUp(shiftKeyRequest(352, false, false), imecore.NewResponse(352, true))

	letterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:    353,
		KeyCode:   0x4E,
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	}, imecore.NewResponse(353, true))
	if letterResp.ReturnValue != 1 {
		t.Fatalf("expected composing to resume in Chinese mode, got %d", letterResp.ReturnValue)
	}
}

func TestShiftToggleResponseCarriesSettingsUpdateOnlyOnToggle(t *testing.T) {
	ime := newIsolatedTestIME(t)

	upResp := ime.filterKeyUp(shiftKeyRequest(360, false, false), imecore.NewResponse(360, true))
	if upResp.ReturnValue != 0 {
		t.Fatalf("expected non-toggling Shift-up to pass through, got %d", upResp.ReturnValue)
	}
	if upResp.TypeDuckSettingsUpdate != nil {
		t.Fatalf("expected no settings update on non-toggling Shift-up, got %#v", upResp.TypeDuckSettingsUpdate)
	}

	letterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:    361,
		KeyCode:   0x4E,
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	}, imecore.NewResponse(361, true))
	if letterResp.TypeDuckSettingsUpdate != nil {
		t.Fatalf("expected no settings update on plain letter, got %#v", letterResp.TypeDuckSettingsUpdate)
	}
}

func TestSettingsUpdateAsciiModeAppliesWithoutRedeploy(t *testing.T) {
	oldCustomize := customizeTypeDuckSettingsFunc
	oldReload := reloadTypeDuckSettingsFunc
	customizeCalls := 0
	reloadCalls := 0
	customizeTypeDuckSettingsFunc = func(prefs typeDuckRimePreferences) bool {
		customizeCalls++
		return true
	}
	reloadTypeDuckSettingsFunc = func(sharedDir, userDir, appname, appver string) bool {
		reloadCalls++
		return true
	}
	defer func() {
		customizeTypeDuckSettingsFunc = oldCustomize
		reloadTypeDuckSettingsFunc = oldReload
	}()

	ime := newIsolatedTestIME(t)
	ime.inputStateShared = true
	backend := ime.backend.(*testBackend)

	asciiMode := true
	resp := ime.HandleRequest(&imecore.Request{
		Method:           "typeduckSettingsUpdate",
		SeqNum:           370,
		TypeDuckSettings: &imecore.TypeDuckSettingsUpdate{AsciiMode: &asciiMode},
	})

	if !resp.Success || resp.ReturnValue != 1 {
		t.Fatalf("expected ascii-only settings update success, got %#v", resp)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode applied as live engine option")
	}
	if backend.redeployCalls != 0 {
		t.Fatalf("expected no redeploy for ascii-only update, got %d", backend.redeployCalls)
	}
	if customizeCalls != 0 || reloadCalls != 0 {
		t.Fatalf("expected no yaml customize/reload for ascii-only update, got customize=%d reload=%d", customizeCalls, reloadCalls)
	}
	// Master-plan override: the applied value is echoed so the launcher
	// persist hook converges (idempotent load-compare-save, no ping-pong).
	if resp.TypeDuckSettingsUpdate == nil || resp.TypeDuckSettingsUpdate.AsciiMode == nil || !*resp.TypeDuckSettingsUpdate.AsciiMode {
		t.Fatalf("expected applied ascii_mode echoed in response, got %#v", resp.TypeDuckSettingsUpdate)
	}
}

func TestSettingsUpdateAsciiModePropagatesToExistingInstance(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()
	resetSchemeSetVersionForTest()

	second := newTestIME()
	second.inputStateShared = true
	second.createSession(nil)

	first := newTestIME()
	first.inputStateShared = true
	asciiMode := true
	resp := first.HandleRequest(&imecore.Request{
		Method:           "typeduckSettingsUpdate",
		SeqNum:           380,
		TypeDuckSettings: &imecore.TypeDuckSettingsUpdate{AsciiMode: &asciiMode},
	})
	if !resp.Success || resp.ReturnValue != 1 {
		t.Fatalf("expected settings update success, got %#v", resp)
	}

	keyResp := second.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    381,
		KeyCode:   0x4E,
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	})
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected existing instance to pick up pushed ascii_mode")
	}
	if keyResp.ReturnValue != 0 {
		t.Fatalf("expected letter to pass through in English mode, got %d", keyResp.ReturnValue)
	}
}

func TestShiftTogglePropagatesAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()
	resetSchemeSetVersionForTest()

	second := newTestIME()
	second.inputStateShared = true
	second.createSession(nil)

	first := newTestIME()
	first.inputStateShared = true
	first.filterKeyDown(shiftKeyRequest(390, false, false), imecore.NewResponse(390, true))
	upResp := first.filterKeyUp(shiftKeyRequest(391, false, false), imecore.NewResponse(391, true))
	if upResp.ReturnValue != 1 {
		t.Fatalf("expected toggling Shift-up eaten, got %d", upResp.ReturnValue)
	}

	second.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    392,
		KeyCode:   0x4E,
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	})
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected second instance to pick up Shift toggle")
	}
}

func TestShiftTogglePublishesAsciiModeWithoutSchemaSaveOptions(t *testing.T) {
	// The deployed TypeDuck schema's switcher/save_options is
	// [full_shape, ascii_punct] with no ascii_mode; cross-session publish
	// must not depend on the schema listing it.
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()
	resetSchemeSetVersionForTest()

	second := newTestIME()
	second.inputStateShared = true
	second.backend.(*testBackend).saveOptions = []string{"full_shape", "ascii_punct"}
	second.createSession(nil)

	first := newTestIME()
	first.inputStateShared = true
	first.backend.(*testBackend).saveOptions = []string{"full_shape", "ascii_punct"}
	first.filterKeyDown(shiftKeyRequest(480, false, false), imecore.NewResponse(480, true))
	upResp := first.filterKeyUp(shiftKeyRequest(481, false, false), imecore.NewResponse(481, true))
	if upResp.ReturnValue != 1 {
		t.Fatalf("expected toggling Shift-up eaten, got %d", upResp.ReturnValue)
	}

	second.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    482,
		KeyCode:   0x4E,
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	})
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected Shift toggle to propagate without schema save_options listing ascii_mode")
	}
}

func TestOnCommandModeIconTogglePersistsWithoutExistingSession(t *testing.T) {
	// The mode-icon click may arrive before any key event created a session
	// (forced composition termination, scheme-set version bump); the launcher
	// persistence signal must still be attached.
	ime := newIsolatedTestIME(t)
	ime.backend.(*testBackend).saveOptions = []string{"full_shape", "ascii_punct"}

	resp := ime.HandleRequest(&imecore.Request{
		Method: "onCommand",
		SeqNum: 470,
		ID:     imecore.FlexibleID{Int: ID_MODE_ICON, IsInt: true},
	})
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected mode icon click to toggle ascii_mode")
	}
	if resp.TypeDuckSettingsUpdate == nil || resp.TypeDuckSettingsUpdate.AsciiMode == nil || !*resp.TypeDuckSettingsUpdate.AsciiMode {
		t.Fatalf("expected settings update with ascii_mode=true, got %#v", resp.TypeDuckSettingsUpdate)
	}
}

func TestNewSessionInheritsAsciiMode(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()
	resetSchemeSetVersionForTest()

	first := newTestIME()
	first.inputStateShared = true
	first.filterKeyDown(shiftKeyRequest(400, false, false), imecore.NewResponse(400, true))
	first.filterKeyUp(shiftKeyRequest(401, false, false), imecore.NewResponse(401, true))
	if !first.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode enabled in first instance")
	}

	second := newTestIME()
	second.loadAppearancePrefs()
	if !second.inputStateShared {
		t.Fatal("expected published config to enable input state sharing")
	}
	second.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    402,
		KeyCode:   0x4E,
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	})
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected new session to inherit ascii_mode")
	}
}

func TestAsciiModeSurvivesRedeployAndSchemaSwitch(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()
	resetSchemeSetVersionForTest()

	ime := newTestIME()
	ime.inputStateShared = true
	backend := ime.backend.(*testBackend)

	ime.filterKeyDown(shiftKeyRequest(410, false, false), imecore.NewResponse(410, true))
	ime.filterKeyUp(shiftKeyRequest(411, false, false), imecore.NewResponse(411, true))
	if !backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode enabled before redeploy")
	}

	deployResp := ime.HandleRequest(&imecore.Request{Method: "typeduckDeploy", SeqNum: 412})
	if !deployResp.Success || deployResp.ReturnValue != 1 {
		t.Fatalf("expected redeploy success, got %#v", deployResp)
	}
	if !backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode to survive redeploy")
	}

	// Simulate the deploy-time switch reset (ascii_mode reset: 0); schema
	// switching must reapply the shared value.
	backend.SetOption("ascii_mode", false)
	if !ime.selectSchemaByIDLocked("rime_frost_double_pinyin") {
		t.Fatal("expected schema switch to succeed")
	}
	if !backend.GetOption("ascii_mode") {
		t.Fatal("expected schema switch to reapply shared ascii_mode")
	}
}

func TestShiftToggleDoesNotLogTypedContent(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.inputStateShared = true
	backend := ime.backend.(*testBackend)

	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	SetDebugLoggingEnabled(true)
	defer func() {
		SetDebugLoggingEnabled(false)
		log.SetOutput(oldOutput)
	}()

	typeTestLetters(t, ime, 420, "nihao")
	backend.rawInput = "nihao"
	ime.filterKeyDown(shiftKeyRequest(430, false, false), imecore.NewResponse(430, true))
	ime.filterKeyUp(shiftKeyRequest(431, false, false), imecore.NewResponse(431, true))
	onResp := ime.onKeyUp(shiftKeyRequest(432, false, false), imecore.NewResponse(432, true))
	if onResp.CommitString != "nihao" {
		t.Fatalf("expected raw letters committed, got %q", onResp.CommitString)
	}

	logText := buf.String()
	if strings.Contains(logText, "nihao") {
		t.Fatalf("expected typed letters not logged, got %q", logText)
	}
	if strings.Contains(logText, "你好") {
		t.Fatalf("expected candidate text not logged, got %q", logText)
	}
}

func TestCapsLockStillForwardedToRime(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:    440,
		KeyCode:   vkCapital,
		KeyStates: make(imecore.KeyStates, 256),
	}, imecore.NewResponse(440, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected Caps Lock handled by rime, got %d", resp.ReturnValue)
	}
	if !slices.Contains(backend.translatedKeyCodes, rimeCapsLock) {
		t.Fatalf("expected Caps Lock forwarded to rime, got %v", backend.translatedKeyCodes)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected Caps Lock to toggle ascii inside rime")
	}
}

func TestOnCommandAsciiToggleAttachesSettingsUpdate(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.createSession(nil) // lang-bar clicks always happen with a live session

	resp := ime.HandleRequest(&imecore.Request{
		Method: "onCommand",
		SeqNum: 450,
		ID:     imecore.FlexibleID{Int: ID_ASCII_MODE, IsInt: true},
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected ascii command handled, got %d", resp.ReturnValue)
	}
	if resp.TypeDuckSettingsUpdate == nil || resp.TypeDuckSettingsUpdate.AsciiMode == nil || !*resp.TypeDuckSettingsUpdate.AsciiMode {
		t.Fatalf("expected settings update with ascii_mode=true, got %#v", resp.TypeDuckSettingsUpdate)
	}

	other := ime.HandleRequest(&imecore.Request{
		Method: "onCommand",
		SeqNum: 451,
		ID:     imecore.FlexibleID{Int: ID_FULL_SHAPE, IsInt: true},
	})
	if other.TypeDuckSettingsUpdate != nil {
		t.Fatalf("expected no settings update for non-ascii command, got %#v", other.TypeDuckSettingsUpdate)
	}
}
