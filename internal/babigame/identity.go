package babigame

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DisplayNameFromSession builds a useful local label from the HTTP login
// session. It is a fallback until WS index.login returns the in-game user row.
func DisplayNameFromSession(s *Session, username string) string {
	if s == nil {
		return FormatDisplayName("", 0, username)
	}
	nick := firstString(s.AccountRaw,
		"nickName", "nickname", "nick",
		"roleName", "role_name",
		"usrName", "userName", "displayName",
	)
	if nick == "" && strings.TrimSpace(username) == "" {
		nick = firstString(s.AccountRaw, "name")
	}
	return FormatDisplayName(nick, s.GsIdx, username)
}

// DisplayNameFromState reads the in-game nickname from an index.login state
// delta and appends the server area.
func DisplayNameFromState(rawV json.RawMessage, gsIdx int, fallback string) string {
	nick := UserNameFromState(rawV)
	return FormatDisplayName(nick, gsIdx, fallback)
}

// UserNameFromState extracts G.IUsr.name (namespace 7, usrTot.data.name).
func UserNameFromState(rawV json.RawMessage) string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(rawV, &top); err != nil {
		return ""
	}
	for _, key := range []string{"7", "$usrTot", "usrTot"} {
		if raw, ok := top[key]; ok {
			if name := userNameFromUsrTot(raw); name != "" {
				return name
			}
		}
	}
	return ""
}

// FormatDisplayName joins a nickname/fallback with a stable area label.
func FormatDisplayName(nick string, gsIdx int, fallback string) string {
	nick = cleanDisplayName(nick)
	if nick == "" {
		nick = cleanDisplayName(fallback)
	}
	if nick == "" {
		nick = "账号"
	}
	area := AreaLabel(gsIdx)
	if area == "" || strings.Contains(nick, area) {
		return nick
	}
	return fmt.Sprintf("%s · %s", nick, area)
}

func AreaLabel(gsIdx int) string {
	if gsIdx <= 0 {
		return ""
	}
	return fmt.Sprintf("第%d区", gsIdx)
}

func userNameFromUsrTot(raw json.RawMessage) string {
	if name := stringField(raw, "5", "name", "Name"); name != "" {
		return name
	}
	var tot map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tot); err != nil {
		return ""
	}
	for _, key := range []string{"0", "data", "usr", "Usr"} {
		if child, ok := tot[key]; ok {
			if name := stringField(child, "5", "name", "Name"); name != "" {
				return name
			}
		}
	}
	return ""
}

func stringField(raw json.RawMessage, keys ...string) string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ""
	}
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(value, &s); err == nil {
			if s = cleanDisplayName(s); s != "" {
				return s
			}
		}
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			if s = cleanDisplayName(s); s != "" {
				return s
			}
		}
	}
	return ""
}

func cleanDisplayName(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
