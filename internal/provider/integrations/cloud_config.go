package integrations

// Cloud configuration nulls are incomplete observations, not explicit clears.
// Keep this bounded to AWS/Keyspaces instead of changing legacy mapping semantics.
func (a *configApplier) cloudString(key, attr string) {
	if a.collect != nil || a.cfg[key] != nil {
		a.str(key, attr)
	}
}
