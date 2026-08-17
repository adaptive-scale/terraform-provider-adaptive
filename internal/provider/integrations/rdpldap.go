package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type RDPLdapIntegrationConfiguration struct {
	Version            string `yaml:"version"`
	Name               string `yaml:"name"`
	Hostname           string `yaml:"hostname"`
	Port               string `yaml:"port"`
	LdapHost           string `yaml:"ldap-hostname"`
	LdapPort           string `yaml:"ldap-port"`
	EncryptionMethod   string `yaml:"ldap-encryption-method"`
	SearchBindDN       string `yaml:"ldap-search-bind-dn"`
	SearchBindPassword string `yaml:"ldap-search-bind-password"`
	UserBaseDN         string `yaml:"ldap-user-base-dn"`
	UserNameAttribute  string `yaml:"ldap-user-name-attribute"`
}

func SchemaToRDPLdapIntegrationConfiguration(d *schema.ResourceData) RDPLdapIntegrationConfiguration {
	return RDPLdapIntegrationConfiguration{
		Version:            "1.0",
		Name:               attrString(d, "name"),
		Hostname:           attrString(d, "hostname"),
		Port:               attrString(d, "port"),
		LdapHost:           attrString(d, "ldap_hostname"),
		LdapPort:           attrString(d, "ldap_port"),
		EncryptionMethod:   attrString(d, "ldap_encryption_method"),
		SearchBindDN:       attrString(d, "ldap_search_bind_dn"),
		SearchBindPassword: attrString(d, "ldap_search_bind_password"),
		UserBaseDN:         attrString(d, "ldap_user_base_dn"),
		UserNameAttribute:  attrString(d, "ldap_user_name_attribute"),
	}
}
