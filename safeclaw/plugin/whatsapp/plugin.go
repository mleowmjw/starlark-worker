package whatsapp

import (

	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cadence-workflow/starlark-worker/safeclaw"
	"github.com/cadence-workflow/starlark-worker/safeclaw/star"
	"go.starlark.net/starlark"
)

// Plugin is a Safeclaw Starlark plugin providing a WhatsApp-like interface.
//
// IMPORTANT: This implementation is intentionally MOCK ONLY.
// It exists to lock down a Starlark-facing interface and expected data shapes.
//
// Scenario selection:
// - WHATSAPP_SCENARIO=mixed|work|family|otp_spam
// - WHATSAPP_SELF="Your Name" (used for outgoing messages)
//
// Starlark API (module = `whatsapp`):
// - whatsapp.scenarios() -> [str]
// - whatsapp.info() -> dict
// - whatsapp.chats() -> [dict(chat)]
// - whatsapp.latest_messages(chat_id, limit=10, include_system=False) -> [dict(message)]
// - whatsapp.latest(limit=10, include_system=False) -> [dict(message)]
// - whatsapp.search(query, limit=20, include_system=False) -> [dict(message)]
// - whatsapp.send_message(chat_id, text) -> dict(message)
var Plugin safeclaw.Plugin = &plugin{}

type plugin struct{}

func (p *plugin) ID() string { return "whatsapp" }

func (p *plugin) Module(ctx interface{}, info safeclaw.RunInfo) starlark.Value {
	scenario := strings.TrimSpace(info.Environ["WHATSAPP_SCENARIO"])
	if scenario == "" {
		scenario = "mixed"
	}
	self := strings.TrimSpace(info.Environ["WHATSAPP_SELF"])
	if self == "" {
		self = "me"
	}
	return &Module{
		scenario:  scenario,
		self:      self,
		startUnix: info.StartTime.Unix(),
	}
}

type Module struct {
	scenario  string
	self      string
	startUnix int64
	seq       int64
}

var _ starlark.HasAttrs = (*Module)(nil)

func (m *Module) String() string        { return "whatsapp" }
func (m *Module) Type() string          { return "whatsapp" }
func (m *Module) Freeze()               {}
func (m *Module) Truth() starlark.Bool  { return true }
func (m *Module) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: whatsapp") }
func (m *Module) Attr(n string) (starlark.Value, error) {
	return star.Attr(m, n, builtins, properties)
}
func (m *Module) AttrNames() []string { return star.AttrNames(builtins, properties) }

var properties = map[string]star.PropertyFactory{}

var builtins = map[string]*starlark.Builtin{
	"scenarios":       starlark.NewBuiltin("scenarios", _scenarios),
	"info":            starlark.NewBuiltin("info", _info),
	"chats":           starlark.NewBuiltin("chats", _chats),
	"latest_messages": starlark.NewBuiltin("latest_messages", _latestMessages),
	"latest":          starlark.NewBuiltin("latest", _latest),
	"search":          starlark.NewBuiltin("search", _search),
	"send_message":    starlark.NewBuiltin("send_message", _sendMessage),
}

type chat struct {
	ID           string
	Name         string
	IsGroup      bool
	Participants []string
}

type message struct {
	ID        string
	ChatID    string
	ChatName  string
	Timestamp int64 // unix seconds
	Sender    string
	Direction string // "in" | "out" | "system"
	Text      string
}

func _scenarios(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("scenarios", args, kwargs); err != nil {
		return nil, err
	}
	return starlark.NewList([]starlark.Value{
		starlark.String("mixed"),
		starlark.String("work"),
		starlark.String("family"),
		starlark.String("otp_spam"),
	}), nil
}

func _info(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("info", args, kwargs); err != nil {
		return nil, err
	}
	m := fn.Receiver().(*Module)
	d := starlark.NewDict(4)
	_ = d.SetKey(starlark.String("mode"), starlark.String("mock"))
	_ = d.SetKey(starlark.String("scenario"), starlark.String(m.scenario))
	_ = d.SetKey(starlark.String("self"), starlark.String(m.self))
	_ = d.SetKey(starlark.String("start_unix"), starlark.MakeInt64(m.startUnix))
	return d, nil
}

func _chats(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("chats", args, kwargs); err != nil {
		return nil, err
	}
	m := fn.Receiver().(*Module)
	chats, _ := mockData(m)

	out := make([]starlark.Value, 0, len(chats))
	for _, c := range chats {
		out = append(out, chatToValue(c))
	}
	return starlark.NewList(out), nil
}

func _latestMessages(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var chatID starlark.String
	limit := 10
	includeSystem := false
	if err := starlark.UnpackArgs(
		"latest_messages",
		args, kwargs,
		"chat_id", &chatID,
		"limit?", &limit,
		"include_system?", &includeSystem,
	); err != nil {
		logger.Error("whatsapp.latest_messages: unpack args failed", "error", err)
		return nil, err
	}

	lim := int64(limit)
	if lim <= 0 {
		lim = 10
	}
	incSys := includeSystem

	m := fn.Receiver().(*Module)
	_, msgs := mockData(m)

	filtered := make([]message, 0, len(msgs))
	for _, mm := range msgs {
		if mm.ChatID != chatID.GoString() {
			continue
		}
		if !incSys && mm.Direction == "system" {
			continue
		}
		filtered = append(filtered, mm)
	}

	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Timestamp > filtered[j].Timestamp })
	if int64(len(filtered)) > lim {
		filtered = filtered[:lim]
	}

	out := make([]starlark.Value, 0, len(filtered))
	for _, mm := range filtered {
		out = append(out, messageToValue(mm))
	}
	return starlark.NewList(out), nil
}

func _latest(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	limit := 10
	includeSystem := false
	if err := starlark.UnpackArgs("latest", args, kwargs, "limit?", &limit, "include_system?", &includeSystem); err != nil {
		logger.Error("whatsapp.latest: unpack args failed", "error", err)
		return nil, err
	}

	lim := int64(limit)
	if lim <= 0 {
		lim = 10
	}
	incSys := includeSystem

	m := fn.Receiver().(*Module)
	_, msgs := mockData(m)

	filtered := make([]message, 0, len(msgs))
	for _, mm := range msgs {
		if !incSys && mm.Direction == "system" {
			continue
		}
		filtered = append(filtered, mm)
	}

	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Timestamp > filtered[j].Timestamp })
	if int64(len(filtered)) > lim {
		filtered = filtered[:lim]
	}

	out := make([]starlark.Value, 0, len(filtered))
	for _, mm := range filtered {
		out = append(out, messageToValue(mm))
	}
	return starlark.NewList(out), nil
}

func _search(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var query starlark.String
	limit := 20
	includeSystem := false
	if err := starlark.UnpackArgs("search", args, kwargs, "query", &query, "limit?", &limit, "include_system?", &includeSystem); err != nil {
		logger.Error("whatsapp.search: unpack args failed", "error", err)
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(query.GoString()))
	if q == "" {
		return starlark.NewList(nil), nil
	}

	lim := int64(limit)
	if lim <= 0 {
		lim = 20
	}
	incSys := includeSystem

	m := fn.Receiver().(*Module)
	chats, msgs := mockData(m)

	chatNameByID := map[string]string{}
	for _, c := range chats {
		chatNameByID[c.ID] = c.Name
	}

	matches := make([]message, 0, 32)
	for _, mm := range msgs {
		if !incSys && mm.Direction == "system" {
			continue
		}
		hay := strings.ToLower(mm.Sender + " " + mm.Text + " " + chatNameByID[mm.ChatID])
		if strings.Contains(hay, q) {
			matches = append(matches, mm)
		}
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].Timestamp > matches[j].Timestamp })
	if int64(len(matches)) > lim {
		matches = matches[:lim]
	}

	out := make([]starlark.Value, 0, len(matches))
	for _, mm := range matches {
		out = append(out, messageToValue(mm))
	}
	return starlark.NewList(out), nil
}

func _sendMessage(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	logger := safeclaw.GetLogger(t)

	var chatID starlark.String
	var text starlark.String
	if err := starlark.UnpackArgs("send_message", args, kwargs, "chat_id", &chatID, "text", &text); err != nil {
		logger.Error("whatsapp.send_message: unpack args failed", "error", err)
		return nil, err
	}

	m := fn.Receiver().(*Module)
	m.seq++
	id := fmt.Sprintf("mock-out-%d-%d", m.startUnix, m.seq)

	// Best-effort: attach chat name if known
	chats, _ := mockData(m)
	chatName := ""
	for _, c := range chats {
		if c.ID == chatID.GoString() {
			chatName = c.Name
			break
		}
	}

	msg := message{
		ID:        id,
		ChatID:    chatID.GoString(),
		ChatName:  chatName,
		Timestamp: m.startUnix + m.seq, // monotonic-ish
		Sender:    m.self,
		Direction: "out",
		Text:      text.GoString(),
	}
	return messageToValue(msg), nil
}

func chatToValue(c chat) starlark.Value {
	d := starlark.NewDict(4)
	_ = d.SetKey(starlark.String("id"), starlark.String(c.ID))
	_ = d.SetKey(starlark.String("name"), starlark.String(c.Name))
	_ = d.SetKey(starlark.String("is_group"), starlark.Bool(c.IsGroup))

	parts := make([]starlark.Value, 0, len(c.Participants))
	for _, p := range c.Participants {
		parts = append(parts, starlark.String(p))
	}
	_ = d.SetKey(starlark.String("participants"), starlark.NewList(parts))
	return d
}

func messageToValue(m message) starlark.Value {
	d := starlark.NewDict(8)
	_ = d.SetKey(starlark.String("id"), starlark.String(m.ID))
	_ = d.SetKey(starlark.String("chat_id"), starlark.String(m.ChatID))
	if m.ChatName != "" {
		_ = d.SetKey(starlark.String("chat_name"), starlark.String(m.ChatName))
	} else {
		_ = d.SetKey(starlark.String("chat_name"), starlark.String(""))
	}
	_ = d.SetKey(starlark.String("timestamp_unix"), starlark.MakeInt64(m.Timestamp))
	_ = d.SetKey(starlark.String("sender"), starlark.String(m.Sender))
	_ = d.SetKey(starlark.String("direction"), starlark.String(m.Direction))
	_ = d.SetKey(starlark.String("text"), starlark.String(m.Text))
	return d
}

func mockData(m *Module) ([]chat, []message) {
	// Base time is derived from runner start time, but data is deterministic per run.
	base := time.Unix(m.startUnix, 0).UTC()
	ts := func(delta time.Duration) int64 { return base.Add(delta).Unix() }

	switch m.scenario {
	case "work":
		ch := []chat{
			{ID: "chat_work", Name: "Work - Team", IsGroup: true, Participants: []string{m.self, "Alex", "Priya", "Jordan"}},
			{ID: "chat_boss", Name: "Manager", IsGroup: false, Participants: []string{m.self, "Jordan"}},
		}
		msgs := []message{
			{ID: "m1", ChatID: "chat_work", ChatName: "Work - Team", Timestamp: ts(-55 * time.Minute), Sender: "Priya", Direction: "in", Text: "Standup moved to 10:30."},
			{ID: "m2", ChatID: "chat_work", ChatName: "Work - Team", Timestamp: ts(-40 * time.Minute), Sender: "Alex", Direction: "in", Text: "Can someone review my PR?"},
			{ID: "m3", ChatID: "chat_boss", ChatName: "Manager", Timestamp: ts(-25 * time.Minute), Sender: "Jordan", Direction: "in", Text: "Need a status update on the deploy ASAP."},
			{ID: "m4", ChatID: "chat_work", ChatName: "Work - Team", Timestamp: ts(-15 * time.Minute), Sender: m.self, Direction: "out", Text: "I can take the PR review in 20."},
			{ID: "m5", ChatID: "chat_work", ChatName: "Work - Team", Timestamp: ts(-5 * time.Minute), Sender: "Priya", Direction: "in", Text: "Incident is resolved. Postmortem tomorrow."},
		}
		return ch, msgs

	case "family":
		ch := []chat{
			{ID: "chat_family", Name: "Family", IsGroup: true, Participants: []string{m.self, "Mom", "Dad", "Sam"}},
			{ID: "chat_sam", Name: "Sam", IsGroup: false, Participants: []string{m.self, "Sam"}},
		}
		msgs := []message{
			{ID: "m1", ChatID: "chat_family", ChatName: "Family", Timestamp: ts(-3 * time.Hour), Sender: "", Direction: "system", Text: "Messages to this group are now secured with end-to-end encryption."},
			{ID: "m2", ChatID: "chat_family", ChatName: "Family", Timestamp: ts(-70 * time.Minute), Sender: "Mom", Direction: "in", Text: "What time are we meeting today?"},
			{ID: "m3", ChatID: "chat_family", ChatName: "Family", Timestamp: ts(-50 * time.Minute), Sender: "Dad", Direction: "in", Text: "Address again please."},
			{ID: "m4", ChatID: "chat_sam", ChatName: "Sam", Timestamp: ts(-20 * time.Minute), Sender: "Sam", Direction: "in", Text: "ETA? I'm outside."},
			{ID: "m5", ChatID: "chat_sam", ChatName: "Sam", Timestamp: ts(-10 * time.Minute), Sender: m.self, Direction: "out", Text: "Coming down now."},
		}
		return ch, msgs

	case "otp_spam":
		ch := []chat{
			{ID: "chat_auth", Name: "Service Alerts", IsGroup: false, Participants: []string{"Service", m.self}},
			{ID: "chat_promo", Name: "Promo", IsGroup: false, Participants: []string{"PromoBot", m.self}},
		}
		msgs := []message{
			{ID: "m1", ChatID: "chat_auth", ChatName: "Service Alerts", Timestamp: ts(-12 * time.Minute), Sender: "Service", Direction: "in", Text: "Your verification code is 123456. Do not share it."},
			{ID: "m2", ChatID: "chat_promo", ChatName: "Promo", Timestamp: ts(-11 * time.Minute), Sender: "PromoBot", Direction: "in", Text: "Limited time discount — 50% OFF today only!"},
			{ID: "m3", ChatID: "chat_auth", ChatName: "Service Alerts", Timestamp: ts(-9 * time.Minute), Sender: "Service", Direction: "in", Text: "OTP: 789012"},
			{ID: "m4", ChatID: "chat_promo", ChatName: "Promo", Timestamp: ts(-7 * time.Minute), Sender: "PromoBot", Direction: "in", Text: "Win a prize! Click to claim."},
		}
		return ch, msgs

	case "mixed":
		fallthrough
	default:
		ch := []chat{
			{ID: "chat_work", Name: "Work - Team", IsGroup: true, Participants: []string{m.self, "Alex", "Priya"}},
			{ID: "chat_family", Name: "Family", IsGroup: true, Participants: []string{m.self, "Mom", "Dad"}},
			{ID: "chat_friend", Name: "Riley", IsGroup: false, Participants: []string{m.self, "Riley"}},
		}
		msgs := []message{
			{ID: "m1", ChatID: "chat_family", ChatName: "Family", Timestamp: ts(-5 * time.Hour), Sender: "", Direction: "system", Text: "Riley was added."},
			{ID: "m2", ChatID: "chat_work", ChatName: "Work - Team", Timestamp: ts(-2 * time.Hour), Sender: "Priya", Direction: "in", Text: "Meeting at 2pm re: release timeline."},
			{ID: "m3", ChatID: "chat_friend", ChatName: "Riley", Timestamp: ts(-80 * time.Minute), Sender: "Riley", Direction: "in", Text: "Want to grab dinner tonight?"},
			{ID: "m4", ChatID: "chat_family", ChatName: "Family", Timestamp: ts(-60 * time.Minute), Sender: "Mom", Direction: "in", Text: "Where are you?"},
			{ID: "m5", ChatID: "chat_work", ChatName: "Work - Team", Timestamp: ts(-45 * time.Minute), Sender: "Alex", Direction: "in", Text: "PR is ready for code review."},
			{ID: "m6", ChatID: "chat_work", ChatName: "Work - Team", Timestamp: ts(-20 * time.Minute), Sender: m.self, Direction: "out", Text: "On it. Will review in 10."},
			{ID: "m7", ChatID: "chat_friend", ChatName: "Riley", Timestamp: ts(-10 * time.Minute), Sender: "Riley", Direction: "in", Text: "Also—did you see that invoice?"},
			{ID: "m8", ChatID: "chat_friend", ChatName: "Riley", Timestamp: ts(-8 * time.Minute), Sender: "Riley", Direction: "in", Text: "Payment link looks sketchy; might be spam."},
		}
		return ch, msgs
	}
}

