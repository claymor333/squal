package state

type Turn struct {
	Connection string
	Tool       string
	ArgsJSON   string
	Result     string
}

// AddTurn persists one tool call to the transcript table.
func (s *Store) AddTurn(conn, tool, argsJSON, result string) error {
	_, err := s.db.Exec(`INSERT INTO turns (connection, tool, args_json, result)
		VALUES (?, ?, ?, ?)`, conn, tool, argsJSON, result)
	return err
}

func (s *Store) ListTurns(conn string, limit int) ([]Turn, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT connection, tool, args_json, result
		FROM turns WHERE connection = ? ORDER BY rowid DESC LIMIT ?`, conn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Turn
	for rows.Next() {
		var t Turn
		if err := rows.Scan(&t.Connection, &t.Tool, &t.ArgsJSON, &t.Result); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
