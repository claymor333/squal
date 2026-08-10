package db

import "fmt"

// keysetCondition builds a composite-key strictly-greater predicate for keyset
// pagination over ORDER BY pk. lastVals must align with the pk columns.
func keysetCondition(pk, lastVals []string) string {
	if len(pk) == 0 || len(pk) != len(lastVals) {
		return ""
	}
	if len(pk) == 1 {
		return fmt.Sprintf("(%s > '%s')", QuoteIdentifier(pk[0]), escape(lastVals[0]))
	}
	parts := make([]string, 0, len(pk))
	for i := range pk {
		eq := make([]string, 0, i+1)
		for j := 0; j < i; j++ {
			eq = append(eq, fmt.Sprintf("%s = '%s'", QuoteIdentifier(pk[j]), escape(lastVals[j])))
		}
		eq = append(eq, fmt.Sprintf("%s > '%s'", QuoteIdentifier(pk[i]), escape(lastVals[i])))
		parts = append(parts, "("+joinAnd(eq)+")")
	}
	return "(" + joinOr(parts) + ")"
}

func escape(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\'' || r == '\\' {
			out = append(out, '\\')
		}
		out = append(out, r)
	}
	return string(out)
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

func joinOr(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " OR "
		}
		out += p
	}
	return out
}

// LoadNextSQL returns the query for the next keyset page after lastVals.
func LoadNextSQL(dbName, table string, pk, lastVals []string, limit int) (string, error) {
	cond := keysetCondition(pk, lastVals)
	if cond == "" {
		return "", fmt.Errorf("keyset pagination requires a primary key on %s.%s", dbName, table)
	}
	return fmt.Sprintf("SELECT * FROM %s.%s WHERE %s ORDER BY %s LIMIT %d",
		QuoteIdentifier(dbName), QuoteIdentifier(table), cond, quotedList(pk), limit), nil
}

func quotedList(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += QuoteIdentifier(n)
	}
	return out
}
