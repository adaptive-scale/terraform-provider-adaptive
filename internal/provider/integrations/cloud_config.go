package integrations

// Presence-aware configuration strings treat null as an incomplete/withheld
// observation, not a clear. The historical cloudString name is retained, but
// callers opt in per integration/field; legacy mappings are not changed globally.
func (a *configApplier) cloudString(key, attr string) {
	if a.collect != nil || a.cfg[key] != nil {
		a.str(key, attr)
	}
}
