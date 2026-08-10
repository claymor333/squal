package tui

import "github.com/claymor333/squal/internal/config"

type connectField int

const (
	hostField connectField = iota
	portField
	userField
	passField
	dbField
	numConnectFields
)

type connectRequestMsg struct {
	Profile config.Profile
}

type connectView struct {
	fields [numConnectFields]string
	cur    connectField
}

func newConnectView() *connectView {
	return &connectView{fields: [numConnectFields]string{portField: "3306"}}
}

func (c *connectView) setField(f connectField, v string) {
	if int(f) < len(c.fields) {
		c.fields[f] = v
	}
}

func (c *connectView) value(f connectField) string {
	return c.fields[f]
}

func (c *connectView) buildProfile(name string) (config.Profile, bool) {
	p := config.Profile{
		Name: name, Host: c.fields[hostField],
		User: c.fields[userField], Password: c.fields[passField],
		Database: c.fields[dbField], Timeout: 5,
	}
	if p.Host == "" || p.User == "" {
		return p, false
	}
	return p, true
}
