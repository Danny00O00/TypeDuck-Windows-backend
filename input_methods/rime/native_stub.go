//go:build !windows && !android

package rime

import "github.com/gaboolic/moqi-ime/imecore"

type RimeSessionId uintptr

type RimeComposition struct {
	Length    int
	CursorPos int
	SelStart  int
	SelEnd    int
	Preedit   string
}

type RimeCandidate struct {
	Text    string
	Comment string
}

type RimeMenu struct {
	PageSize                  int
	PageNo                    int
	IsLastPage                bool
	HighlightedCandidateIndex int
	NumCandidates             int
	Candidates                []RimeCandidate
	SelectKeys                string
}

type RimeCommit struct {
	Text string
}

type RimeSchema struct {
	ID   string
	Name string
}

type RimeSwitch struct {
	Name   string
	States []string
}

type NotificationHandler func(session RimeSessionId, messageType, messageValue string)

type nativeBackend struct{}

func newNativeBackend() rimeBackend {
	return nil
}

func (b *nativeBackend) Initialize(sharedDir, userDir string, firstRun bool) bool { return false }
func (b *nativeBackend) SyncUserData() bool                                       { return false }
func (b *nativeBackend) HasSession() bool                                         { return false }
func (b *nativeBackend) EnsureSession() bool                                      { return false }
func (b *nativeBackend) DestroySession()                                          {}
func (b *nativeBackend) ClearComposition()                                        {}
func (b *nativeBackend) ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool {
	return false
}
func (b *nativeBackend) State() rimeState                            { return rimeState{} }
func (b *nativeBackend) SetOption(name string, value bool)           {}
func (b *nativeBackend) GetOption(name string) bool                  { return false }
func (b *nativeBackend) SaveOptions() []string                       { return nil }
func (b *nativeBackend) SchemaSwitches() []RimeSwitch                { return nil }
func (b *nativeBackend) SchemaList() []RimeSchema                    { return nil }
func (b *nativeBackend) CurrentSchemaID() string                     { return "" }
func (b *nativeBackend) SelectSchema(schemaID string) bool           { return false }
func (b *nativeBackend) SetCandidatePageSize(pageSize int) bool      { return false }
func (b *nativeBackend) SelectCandidate(index int) bool              { return false }
func (b *nativeBackend) HighlightCandidate(index int) bool           { return false }
func (b *nativeBackend) ChangePage(backward bool) bool               { return false }
func (b *nativeBackend) DeleteCandidateOnCurrentPage(index int) bool { return false }
func (b *nativeBackend) Available() bool                             { return false }

func (b *nativeBackend) Redeploy(sharedDir, userDir string) bool { return false }

func (b *nativeBackend) ConsumeNotification() *imecore.TrayNotification { return nil }

func ProcessKey(sessionId RimeSessionId, keyCode, modifiers int) bool { return false }
func ClearComposition(sessionId RimeSessionId)                        {}
func GetInput(sessionId RimeSessionId) string                         { return "" }
func GetComposition(sessionId RimeSessionId) (RimeComposition, bool) {
	return RimeComposition{}, false
}
func GetMenu(sessionId RimeSessionId) (RimeMenu, bool) { return RimeMenu{}, false }
func GetCommit(sessionId RimeSessionId) (RimeCommit, bool) {
	return RimeCommit{}, false
}
func SetOption(sessionId RimeSessionId, option string, value bool) {}
func GetOption(sessionId RimeSessionId, option string) bool        { return false }
func GetSchemaList() []RimeSchema                                  { return nil }
func GetCurrentSchema(sessionId RimeSessionId) string              { return "" }
func SelectSchema(sessionId RimeSessionId, schemaID string) bool   { return false }
func GetConfigStringList(configID, key string) []string            { return nil }
func GetSchemaConfigStringList(schemaID, key string) []string      { return nil }
func GetSchemaSwitches(schemaID string) []RimeSwitch               { return nil }
func SetSchemaPageSize(schemaID string, pageSize int) bool         { return false }
func SelectCandidate(sessionId RimeSessionId, index int) bool      { return false }
func HighlightCandidate(sessionId RimeSessionId, index int) bool   { return false }
func ChangePage(sessionId RimeSessionId, backward bool) bool       { return false }
func DeployConfigFile(filePath, key string) bool                   { return false }
func CustomizeTypeDuckSettings(prefs typeDuckRimePreferences) bool { return false }
func StartMaintenance(fullcheck bool) bool                         { return false }
func JoinMaintenanceThread()                                       {}
func SyncUserData() bool                                           { return false }
func SetNotificationHandler(handler NotificationHandler)           {}
func GetName() string                                              { return "" }
func GetVersion() string                                           { return "" }
func RimeInit(datadir, userdir, appname, appver string, fullcheck bool) bool {
	return false
}
func RimeRedeploy(datadir, userdir, appname, appver string) bool { return false }
func RimeReloadIncremental(datadir, userdir, appname, appver string) bool {
	return false
}

var rimeRedeployFunc = RimeRedeploy

var rimeInitOK bool

func resetNativeRuntimeStateForTest() {}

func typeDuckRimeModules() []string {
	return []string{"default", "levers", "dictionary_lookup"}
}
