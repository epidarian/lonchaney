package plugin

import "lonchan/internal/model"

// Builtin is the connection instructions and fields for a built-in plugin.
func Builtin(id string) (string, []model.Field, bool) {
	switch id {
	case model.PluginLocal:
		return "Messages stay on this machine.", nil, true
	case model.PluginDirect:
		return "Markdown-direct listens for peers. Relay lets this connection forward a group.", []model.Field{
			{Key: "bindHost", Label: "Bind host", Placeholder: "127.0.0.1"},
			{Key: "bindPort", Label: "Bind port", Placeholder: "any free port"},
			{Key: "relay", Label: "Relay messages for groups", Bool: true},
		}, true
	case model.PluginIRC:
		return "Connect to an IRC server. The nick is the name this account uses on that server.", []model.Field{
			{Key: "host", Label: "Host", Placeholder: "irc.example.org"},
			{Key: "port", Label: "Port", Placeholder: "6697"},
			{Key: "tls", Label: "Use TLS", Bool: true},
			{Key: "nick", Label: "Nick"},
			{Key: "password", Label: "Password", Secret: true},
		}, true
	case model.PluginXMPP:
		return "Connect to an XMPP server with the JID and password for this account.", []model.Field{
			{Key: "host", Label: "Server host"},
			{Key: "port", Label: "Port", Placeholder: "5222"},
			{Key: "jid", Label: "JID", Placeholder: "name@example.org"},
			{Key: "password", Label: "Password", Secret: true},
		}, true
	default:
		return "", nil, false
	}
}
