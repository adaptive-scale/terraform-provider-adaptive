package integrations

// WireType maps the `type` written in configuration onto the integration type
// string the API stores. Two differ: the registry keys the service list as
// "servicelist", and ZeroTier as "zero_tier" (both the type constant and the
// integrations-table row use that spelling, so a resource stored as "zerotier"
// resolves to no implementation server-side and is dropped from the resource
// listing join).
func WireType(t string) string {
	switch t {
	case "services":
		return "servicelist"
	case "zerotier":
		return "zero_tier"
	}
	return t
}

// ProviderType is the inverse of WireType: the `type` a configuration writes for
// the integration type the API reports. The legacy "zerotier" spelling still
// appears on resources created before the rename, and maps to itself.
func ProviderType(w string) string {
	switch w {
	case "servicelist":
		return "services"
	case "zero_tier":
		return "zerotier"
	}
	return w
}
