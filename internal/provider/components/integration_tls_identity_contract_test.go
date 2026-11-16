package components

import (
	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"gopkg.in/yaml.v2"
	"reflect"
	"testing"
)

type parityFieldFixture struct {
	typ, yaml string
	names     map[string]string
}

var tlsIdentityFixtures = []parityFieldFixture{
	{"mysql", "clientCert: certificate\nclientKey: private-key\nrootCert: root\noldVersion: true\n", map[string]string{"clientCert": "client_cert", "clientKey": "client_key", "rootCert": "root_cert", "oldVersion": "old_version"}},
	{"mongodb", "clientCertificate: bundle\nuseTLS: true\nversion: '2.0'\n", map[string]string{"clientCertificate": "client_certificate", "useTLS": "use_tls", "version": "version"}},
	{"awsdocumentdb", "tlsEnabled: true\n", map[string]string{"tlsEnabled": "tls_enabled"}},
	{"syslog", "tlsEnabled: true\ncaCertificate: root\nclientCertificate: certificate\nclientKey: private-key\ninsecureSkipVerify: true\n", map[string]string{"tlsEnabled": "tls_enabled", "caCertificate": "tls_ca_cert", "clientCertificate": "client_certificate", "clientKey": "client_key", "insecureSkipVerify": "tls_skip_verify"}},
	{"services", "enableTLS: true\n", map[string]string{"enableTLS": "tls_enabled"}},
	{"sql_server", "sslMode: require\n", map[string]string{"sslMode": "ssl_mode"}},
	{"azuresqlserver", "sslMode: require\n", map[string]string{"sslMode": "ssl_mode"}},
	{"google", "serviceAccount: synthetic-json\nadminEmail: admin@example.invalid\n", map[string]string{"serviceAccount": "service_account", "adminEmail": "admin_email"}},
	{"rdp_windows", "allowFileTransfer: true\n", map[string]string{"allowFileTransfer": "allow_file_transfer"}},
}

func testParityFields(t *testing.T, tc parityFieldFixture) {
	t.Helper()
	var fixture map[string]interface{}
	if err := yaml.UnmarshalStrict([]byte(tc.yaml), &fixture); err != nil {
		t.Fatal(err)
	}
	values := map[string]interface{}{}
	if tc.typ == "azuresqlserver" {
		values["hostname"] = "db"
		values["port"] = "1433"
		values["username"] = "user"
		values["password"] = "synthetic"
		values["database_name"] = "db"
	}
	if tc.typ == "rdp_windows" {
		values["password"] = "synthetic-password"
	}
	for key, value := range fixture {
		values[tc.names[key]] = value
	}
	d := parityData(t, tc.typ, values)
	wire := parityWire(t, d, tc.typ)
	for k, v := range fixture {
		if !reflect.DeepEqual(wire[k], v) {
			t.Errorf("forward %s incorrect (values withheld)", k)
		}
	}
	for k, v := range fixture {
		attr := tc.names[k]
		if err := integrations.ApplyIntegrationConfig(d, tc.typ, map[string]interface{}{k: v}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(d.Get(attr), v) {
			t.Errorf("reverse %s incorrect", attr)
		}
		for _, cfg := range []map[string]interface{}{{}, {k: nil}} {
			if err := integrations.ApplyIntegrationConfig(d, tc.typ, cfg); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(d.Get(attr), v) {
				t.Errorf("absent/null %s changed observation", attr)
			}
		}
		var clear interface{} = ""
		if _, ok := v.(bool); ok {
			clear = false
		}
		if err := integrations.ApplyIntegrationConfig(d, tc.typ, map[string]interface{}{k: clear}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(d.Get(attr), clear) {
			t.Errorf("explicit clear %s lost", attr)
		}
		got, ok := integrations.AttrForConfigKey(tc.typ, k)
		if !ok || got != attr {
			t.Errorf("missing fingerprint alias %s", k)
		}
	}
}
func TestIntegrationGoogleVersionWire(t *testing.T) {
	wire := parityWire(t, parityData(t, "google", map[string]interface{}{}), "google")
	if wire["version"] != "1" {
		t.Error("Google envelope must use backend lowercase version key")
	}
	if _, present := wire["Version"]; present {
		t.Error("unsupported uppercase Version key emitted")
	}
}

func TestIntegrationTLSIdentityBackendContract(t *testing.T) {
	for _, tc := range tlsIdentityFixtures {
		t.Run(tc.typ, func(t *testing.T) { testParityFields(t, tc) })
	}
}
