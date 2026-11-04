package components

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func secretUpdateData(t *testing.T, values map[string]interface{}, config map[string]cty.Value) *schema.ResourceData {
	t.Helper()
	res := ResourceAdaptiveResource()
	d := schema.TestResourceDataRaw(t, res.Schema, values)
	d.SetId("r-1")
	state := d.State()
	state.RawConfig = cty.ObjectVal(config)
	return res.Data(state)
}

func TestResourceUpdateRejectsOmittedHeldSecretBeforeWriting(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","configuration":{"hostname":"h"},"redactedKeys":["password"],"redactedDigests":{"password":"held"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "db", "type": "postgres", "host": "h",
		"tags":            []interface{}{"changed"},
		"applied_digests": map[string]interface{}{"password": "held"},
	})
	d.SetId("r-1")
	diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client)
	if !diags.HasError() {
		t.Error("update accepted omission of a server-held password")
	}
	if api.sawPath("/update/") {
		t.Error("unsafe update reached the mutation endpoint")
	}
}

func TestResourceSecretUpdateConfigurationPresence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]cty.Value
		value  string
		allow  bool
	}{
		{"omitted", map[string]cty.Value{}, "", false},
		{"null", map[string]cty.Value{"password": cty.NullVal(cty.String)}, "", false},
		{"unknown", map[string]cty.Value{"password": cty.UnknownVal(cty.String)}, "", false},
		{"stale state marker is not configuration", map[string]cty.Value{}, "held", false},
		{"explicit empty authorizes clear", map[string]cty.Value{"password": cty.StringVal("")}, "", true},
		{"provided value", map[string]cty.Value{"password": cty.StringVal("synthetic-configured")}, "synthetic-configured", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","configuration":{"hostname":"h"},"redactedKeys":["password"],"redactedDigests":{"password":"held"}}`))
					return
				}
				_, _ = w.Write([]byte(`{"success":true}`))
			})
			d := secretUpdateData(t, map[string]interface{}{
				"name": "db", "type": "postgres", "host": "h", "password": tc.value,
				"tags":            []interface{}{"changed"},
				"applied_digests": map[string]interface{}{"password": "held"},
			}, tc.config)
			before := d.State().DeepCopy()
			diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client)
			if diags.HasError() == tc.allow {
				t.Fatal("unexpected credential-presence decision")
			}
			if api.sawPath("/update/") != tc.allow {
				t.Fatal("mutation endpoint did not match credential-presence decision")
			}
			if !tc.allow {
				if !reflect.DeepEqual(before, d.State()) {
					t.Fatal("rejected update mutated state")
				}
				for _, diag := range diags {
					if strings.Contains(diag.Summary+diag.Detail, "held") && !strings.Contains(diag.Summary+diag.Detail, "server-held") {
						t.Error("diagnostic disclosed a fingerprint")
					}
				}
				return
			}
			for i, path := range api.paths {
				if !strings.Contains(path, "/update/") {
					continue
				}
				var cfg map[string]interface{}
				if err := yaml.Unmarshal([]byte(api.body[i]["config"].(string)), &cfg); err != nil {
					t.Fatal("invalid configuration payload")
				}
				if cfg["password"] != tc.value {
					t.Fatal("update did not serialize the configured credential")
				}
			}
		})
	}
}

func TestResourceSecretUpdateRejectsArbitraryPasswordNormalization(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","redactedDigests":{"password":"held"}}`))
	})
	configured := " synthetic-password "
	d := secretUpdateData(t, map[string]interface{}{
		"name": "db", "type": "postgres", "password": configured,
	}, map[string]cty.Value{"password": cty.StringVal(configured)})
	if err := validateResourceSecretUpdate(context.Background(), d, client, []byte("password: synthetic-password\n")); err == nil {
		t.Error("guard accepted a password transformation not supported by its integration")
	}
}

func TestResourceSecretUpdateAllowsExistingBuilderNormalization(t *testing.T) {
	for _, tc := range []struct{ typ, key, attr string }{
		{"gcp", "key_file", "key_file"},
		{"big_query", "credential_json", "credential_json"},
		{"kubernetes", "token", "cluster_token"},
		{"kubernetes", "cacrt", "cluster_cert"},
		{"mongodb36", "clientCertificate", "client_certificate"},
		{"proxysql", "rootCert", "root_cert"},
		{"proxysql", "clientCert", "client_cert"},
		{"proxysql", "clientKey", "client_key"},
		{"cockroachdb", "rootCert", "tls_root_cert"},
		{"cockroachdb_aws_secrets_manager", "db_rootCert", "root_cert"},
		{"redis", "crtText", "tls_cert_file"},
		{"redis", "keyText", "tls_key_file"},
		{"redis", "caText", "tls_ca_cert"},
	} {
		t.Run(tc.typ+"/"+tc.key, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/update/") {
					_, _ = w.Write([]byte(`{"success":true}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"id": "r-1", "name": "normalized", "integrationType": tc.typ,
					"redactedDigests": map[string]string{tc.key: "held"},
				})
			})
			configured := "\n  synthetic-credential-document\n"
			d := secretUpdateData(t, map[string]interface{}{
				"name": "normalized", "type": tc.typ, tc.attr: configured,
			}, map[string]cty.Value{tc.attr: cty.StringVal(configured)})
			if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); diags.HasError() {
				t.Fatal("guard rejected the builder's existing whitespace normalization")
			}
			if !api.sawPath("/update/") {
				t.Error("normalized credential update did not reach mutation endpoint")
			}
			for i, path := range api.paths {
				if strings.Contains(path, "/update/") {
					var payload map[string]interface{}
					if err := yaml.Unmarshal([]byte(api.body[i]["config"].(string)), &payload); err != nil {
						t.Fatal("could not parse serialized credential payload")
					}
					if payload[tc.key] != strings.TrimSpace(configured) {
						t.Error("builder normalization contract changed")
					}
				}
			}
		})
	}
}

func TestResourceSecretUpdateRejectsWhitespaceOnlyCredentialClear(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r-1","name":"cloud","integrationType":"gcp","redactedDigests":{"key_file":"held"}}`))
	})
	configured := " \n "
	d := secretUpdateData(t, map[string]interface{}{
		"name": "cloud", "type": "gcp", "key_file": configured,
	}, map[string]cty.Value{"key_file": cty.StringVal(configured)})
	if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() {
		t.Error("whitespace-only input authorized an empty credential payload")
	}
	if api.sawPath("/update/") {
		t.Error("unauthorized clear reached mutation endpoint")
	}
}

func TestResourceSecretUpdateRejectsPlannedRawDisagreement(t *testing.T) {
	for _, planned := range []string{"rotated-fingerprint", "different-planned-value"} {
		t.Run(planned, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","redactedDigests":{"password":"rotated-fingerprint"}}`))
			})
			d := secretUpdateData(t, map[string]interface{}{
				"name": "db", "type": "postgres", "password": planned,
			}, map[string]cty.Value{"password": cty.StringVal("configured")})
			before := d.State().DeepCopy()
			diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client)
			if !diags.HasError() {
				t.Error("accepted planned credential that differs from raw configuration")
			}
			if api.sawPath("/update/") {
				t.Error("planned credential disagreement reached mutation endpoint")
			}
			if !reflect.DeepEqual(before, d.State()) {
				t.Error("rejected update mutated state")
			}
			if diags.HasError() && (!strings.Contains(diags[0].Summary, "password") || !strings.Contains(diags[0].Summary, "ignore_changes")) {
				t.Error("diagnostic does not explain the affected argument and lifecycle conflict")
			}
		})
	}
}

func TestResourceSecretUpdateValidatesSerializedIAMCredential(t *testing.T) {
	enabled, disabled := true, false
	for _, tc := range []struct {
		name  string
		iam   *bool
		value string
		allow bool
	}{
		{"IAM omitted", nil, "configured", false},
		{"IAM disabled", &disabled, "configured", false},
		{"IAM enabled", &enabled, "configured", true},
		{"explicit clear IAM omitted", nil, "", true},
		{"explicit clear IAM disabled", &disabled, "", true},
		{"explicit clear IAM enabled", &enabled, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/update/") {
					_, _ = w.Write([]byte(`{"success":true}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","redactedDigests":{"awsSecretAccessKey":"held"}}`))
			})
			values := map[string]interface{}{"name": "db", "type": "postgres", "aws_secret_access_key": tc.value}
			raw := map[string]cty.Value{"aws_secret_access_key": cty.StringVal(tc.value)}
			if tc.iam != nil {
				values["use_rds_iam"] = *tc.iam
				raw["use_rds_iam"] = cty.BoolVal(*tc.iam)
			}
			d := secretUpdateData(t, values, raw)
			before := d.State().DeepCopy()
			diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client)
			if diags.HasError() == tc.allow {
				t.Error("credential decision ignored conditional serialization")
			}
			if api.sawPath("/update/") != tc.allow {
				t.Error("mutation endpoint did not match payload-integrity decision")
			}
			if !tc.allow && !reflect.DeepEqual(before, d.State()) {
				t.Error("rejected update mutated state")
			}
			if !tc.allow && diags.HasError() && !strings.Contains(diags[0].Summary, "aws_secret_access_key") {
				t.Error("diagnostic does not identify discarded credential argument")
			}
			for i, path := range api.paths {
				if !strings.Contains(path, "/update/") {
					continue
				}
				var cfg map[string]interface{}
				if err := yaml.Unmarshal([]byte(api.body[i]["config"].(string)), &cfg); err != nil {
					t.Fatal(err)
				}
				if tc.value != "" && cfg["awsSecretAccessKey"] != tc.value {
					t.Error("outgoing configuration discarded nonempty held credential")
				}
				if tc.value == "" && cfg["awsSecretAccessKey"] != nil && cfg["awsSecretAccessKey"] != "" {
					t.Error("explicit clear serialized a nonempty credential")
				}
			}
		})
	}
}

func TestResourceSecretUpdateUsesLiveHeldKeys(t *testing.T) {
	for _, body := range []string{
		`{"id":"r-1","name":"db","integrationType":"postgres","redactedDigests":{"password":"newly-added"}}`,
		`{"id":"r-1","name":"db","integrationType":"postgres","redactedKeys":["password"]}`,
		`{"id":"r-1","name":"db","integrationType":"postgres","redactedKeys":["unknownCredential"]}`,
	} {
		api := &fakeAPI{}
		client := api.start(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
		d := secretUpdateData(t, map[string]interface{}{"name": "db", "type": "postgres"}, map[string]cty.Value{})
		if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() {
			t.Error("update ignored live held credentials without a recorded baseline")
		}
		if api.sawPath("/update/") {
			t.Error("unsafe update reached mutation endpoint")
		}
	}
}

func TestResourceSecretUpdateAllowsOmissionAfterExplicitServerClear(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","configuration":{"password":""}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	d := secretUpdateData(t, map[string]interface{}{
		"name": "db", "type": "postgres",
		"applied_digests": map[string]interface{}{"password": "old"},
	}, map[string]cty.Value{})
	if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); diags.HasError() {
		t.Fatal("confirmed cleared credential still requires resupply")
	}
	if !api.sawPath("/update/") {
		t.Error("safe update did not reach mutation endpoint")
	}
}

func TestResourceSecretUpdateReadFailureDoesNotWrite(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		api := &fakeAPI{}
		client := api.start(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		d := secretUpdateData(t, map[string]interface{}{"name": "db", "type": "postgres"}, map[string]cty.Value{})
		if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() {
			t.Error("update accepted an unverifiable remote credential state")
		}
		if api.sawPath("/update/") {
			t.Error("failed preflight reached mutation endpoint")
		}
	}
}

func TestOpaqueResourceUpdateRequiresConfiguredValue(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"r-1","name":"encrypted","integrationType":"adaptiveencrypt"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	d := secretUpdateData(t, map[string]interface{}{
		"name": "encrypted", "type": "adaptiveencrypt",
	}, map[string]cty.Value{})
	if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() {
		t.Error("opaque resource update accepted an omitted value without redaction metadata")
	}
	if api.sawPath("/update/") {
		t.Error("opaque resource omission reached mutation endpoint")
	}
}

func TestResourceSecretUpdateCaptureWithoutDigestsPreservesEvidence(t *testing.T) {
	var reads atomic.Int32
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if reads.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres","redactedDigests":{"password":"last-write"}}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres"}`))
			}
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	d := secretUpdateData(t, map[string]interface{}{
		"name": "db", "type": "postgres", "password": "synthetic-configured",
		"applied_digests": map[string]interface{}{"password": "last-write"},
	}, map[string]cty.Value{"password": cty.StringVal("synthetic-configured")})
	if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); diags.HasError() {
		t.Fatal("successful write reported failure")
	}
	if stateStringMap(d, "applied_digests")["password"] != "last-write" {
		t.Error("missing post-write digests discarded prior evidence")
	}
}

func TestResourceSecretUpdateRetainsBaselineAfterFailedWrite(t *testing.T) {
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "r-1", "name": "db", "integrationType": "postgres",
				"redactedDigests": map[string]string{"password": "rotated"},
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	d := secretUpdateData(t, map[string]interface{}{
		"name": "db", "type": "postgres", "password": "synthetic-configured",
		"applied_digests": map[string]interface{}{"password": "last-write"},
	}, map[string]cty.Value{"password": cty.StringVal("synthetic-configured")})
	if diags := ResourceAdaptiveResourceUpdate(context.Background(), d, client); !diags.HasError() {
		t.Fatal("failed write reported success")
	}
	if got := stateStringMap(d, "applied_digests")["password"]; got != "last-write" {
		t.Error("failed write acknowledged a new fingerprint")
	}
}
