package components

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
	"net/http"
	"strings"
	"testing"
)

func TestIntegrationParitySecretGuard(t *testing.T) {
	cases := []struct{ typ, key, attr string }{
		{"mysql", "clientCert", "client_cert"}, {"mysql", "clientKey", "client_key"}, {"mysql", "rootCert", "root_cert"}, {"mysql", "awsSecretAccessKey", "aws_secret_access_key"},
		{"postgres", "awsSecretAccessKey", "aws_secret_access_key"}, {"kafka", "awsSecretAccessKey", "aws_secret_access_key"},
		{"mysql", "awsAccessKeyId", "aws_access_key_id"}, {"postgres", "awsAccessKeyId", "aws_access_key_id"}, {"kafka", "awsAccessKeyId", "aws_access_key_id"},
		{"mongodb", "clientCertificate", "client_certificate"},
		{"syslog", "clientCertificate", "client_certificate"}, {"syslog", "clientKey", "client_key"},
		{"google", "serviceAccount", "service_account"},
	}
	for _, typ := range parityDeviceTypes {
		cases = append(cases, struct{ typ, key, attr string }{typ, "fields", "fields"}, struct{ typ, key, attr string }{typ, "script", "script"}, struct{ typ, key, attr string }{typ, "sshKey", "key"})
	}
	for _, tc := range cases {
		t.Run(tc.typ+"/"+tc.key, func(t *testing.T) {
			if !ResourceAdaptiveResource().Schema[tc.attr].Sensitive {
				t.Errorf("%s must be Sensitive", tc.attr)
			}
			for _, mode := range []string{"omitted", "provided", "empty", "ignored", "inactive"} {
				t.Run(mode, func(t *testing.T) {
					if mode == "inactive" && tc.key != "awsSecretAccessKey" {
						return
					}
					api := &fakeAPI{}
					client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fixture", "integrationType": tc.typ, "redactedDigests": map[string]string{tc.key: "opaque"}})
					})
					raw := map[string]cty.Value{"type": cty.StringVal(tc.typ)}
					values := map[string]interface{}{"name": "fixture", "type": tc.typ}
					configured := " synthetic-secret\n"
					if mode != "omitted" {
						if mode == "empty" {
							configured = ""
						}
						raw[tc.attr] = cty.StringVal(configured)
						values[tc.attr] = configured
					}
					if tc.typ == "mysql" || tc.typ == "postgres" || tc.typ == "kafka" {
						attr := "use_rds_iam"
						if tc.typ == "kafka" {
							attr = "use_msk_iam"
						}
						raw[attr] = cty.BoolVal(mode != "inactive")
						values[attr] = mode != "inactive"
					}
					if mode == "ignored" {
						values[tc.attr] = "opaque-marker"
					}
					d := secretUpdateData(t, values, raw)
					cfg, err := schemaToResourceIntegrationConfiguration(d, tc.typ)
					if err != nil {
						t.Fatal(err)
					}
					payload, err := yaml.Marshal(cfg)
					if err != nil {
						t.Fatal(err)
					}
					err = validateResourceSecretUpdate(context.Background(), d, client, payload)
					allow := mode == "provided" || mode == "empty"
					if (err == nil) != allow {
						t.Errorf("unexpected full replacement guard decision (values withheld)")
					}
					if err != nil && (strings.Contains(err.Error(), configured) && configured != "" || strings.Contains(err.Error(), "opaque-marker")) {
						t.Error("diagnostic disclosed configured/planned secret")
					}
				})
			}
		})
	}
}
func TestIntegrationGoogleUnredactedHeldAccountGuard(t *testing.T) {
	// The backend currently allowlists serviceAccount in some formats. Provider
	// safety must not assume that missing digests means no directory credential.
	for _, value := range []interface{}{"synthetic-account-document", nil} {
		t.Run(map[bool]string{true: "plaintext", false: "null"}[value != nil], func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "r-1", "name": "fixture", "integrationType": "google", "configuration": map[string]interface{}{"serviceAccount": value}})
			})
			d := secretUpdateData(t, map[string]interface{}{"name": "fixture", "type": "google"}, map[string]cty.Value{"type": cty.StringVal("google")})
			if err := validateResourceSecretUpdate(context.Background(), d, client, []byte("serviceAccount: ''\n")); err == nil {
				t.Error("omitted known directory account was not guarded")
			}
		})
	}
}
func TestIntegrationParityLegacySchemaPolicy(t *testing.T) {
	s := ResourceAdaptiveResource().Schema
	for _, a := range []string{"app_id", "database_account", "database_username", "database_password"} {
		if s[a].Deprecated == "" {
			t.Errorf("dead argument %s needs actionable deprecation", a)
		}
	}
	for _, a := range []string{"app_key", "version"} {
		if s[a].Deprecated != "" {
			t.Errorf("valid %s must not be globally deprecated", a)
		}
	}
}

func TestIntegrationParityNewSchema(t *testing.T) {
	s := ResourceAdaptiveResource().Schema
	for _, a := range []string{"auth_mode", "bootstrap_servers", "cache_name", "cache_type", "admin_email"} {
		if s[a] == nil || s[a].Type != schema.TypeString || s[a].Computed {
			t.Errorf("%s must be Optional string, not globally Computed", a)
		}
	}
	for _, a := range []string{"use_msk_iam", "use_iam_auth", "allow_file_transfer"} {
		if s[a] == nil || s[a].Type != schema.TypeBool || s[a].Computed {
			t.Errorf("%s must be Optional bool, not globally Computed", a)
		}
	}
}
