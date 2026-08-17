package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type LDAPIntegrationConfiguration struct {
	Name               string `yaml:"name"`
	Hostname           string `yaml:"hostname"`
	Port               string `yaml:"port"`
	EncryptionMethod   string `yaml:"ldap-encryption-method"`
	SearchBindDN       string `yaml:"ldap-search-bind-dn"`
	SearchBindPassword string `yaml:"ldap-search-bind-password"`
	UserNameAttribute  string `yaml:"ldap-user-name-attribute"`
	UserBaseDN         string `yaml:"ldap-user-base-dn"`
}

func SchemaToLDAPIntegrationConfiguration(d *schema.ResourceData) LDAPIntegrationConfiguration {
	return LDAPIntegrationConfiguration{
		Name:               attrString(d, "name"),
		Hostname:           attrString(d, "hostname"),
		Port:               attrString(d, "port"),
		EncryptionMethod:   attrString(d, "ldap_encryption_method"),
		SearchBindDN:       attrString(d, "ldap_search_bind_dn"),
		SearchBindPassword: attrString(d, "ldap_search_bind_password"),
		UserNameAttribute:  attrString(d, "ldap_user_name_attribute"),
		UserBaseDN:         attrString(d, "ldap_user_base_dn"),
	}
}
