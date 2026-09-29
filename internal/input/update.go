package input

import (
	"fmt"
	"strings"
)

// Update builds a parameterized UPDATE statement from trusted column names.
type Update struct {
	sets []string
	args []any
}

// Set adds one column assignment.
func (u *Update) Set(column string, value any) {
	u.SetCast(column, "", value)
}

// SetCast adds one column assignment with a PostgreSQL cast.
func (u *Update) SetCast(column, cast string, value any) {
	u.args = append(u.args, value)
	placeholder := fmt.Sprintf("$%d", len(u.args))
	if cast != "" {
		placeholder += "::" + cast
	}
	u.sets = append(u.sets, fmt.Sprintf("%s = %s", column, placeholder))
}

// Empty reports whether no columns were set.
func (u *Update) Empty() bool {
	return len(u.sets) == 0
}

// SQL returns an UPDATE statement and its arguments, including the id.
func (u *Update) SQL(table, id string) (string, []any) {
	args := append([]any{}, u.args...)
	args = append(args, id)
	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE id = $%d::uuid",
		table,
		strings.Join(u.sets, ", "),
		len(args),
	)
	return query, args
}
