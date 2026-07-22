package tmux

// EscapeTmuxSeparator returns str unchanged unless it is ";", which is a tmux
// command separator. A lone ";" argument would be interpreted as a separator,
// breaking the command chain. Escaped to "\;" to suppress this.
func EscapeTmuxSeparator(str string) string {
	if str == ";" {
		return `\;`
	}
	return str
}
