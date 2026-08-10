package ai

type Session struct {
	client *Client
	msgs   []Message
}

func NewSession(c *Client) *Session {
	return &Session{client: c}
}

func (s *Session) AddTurn(role, content string) {
	s.msgs = append(s.msgs, Message{Role: role, Content: content})
}

func (s *Session) AddToolResult(id, name, result string) {
	s.msgs = append(s.msgs, Message{Role: "tool", ToolCallID: id, Content: result})
}

func (s *Session) Messages() []Message { return s.msgs }

func (s *Session) Reset() { s.msgs = s.msgs[:0] }
