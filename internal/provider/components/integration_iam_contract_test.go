package components

import (
	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
	"reflect"
	"testing"
)

// Backend config.go contracts at inventorize 52bce48e. bootstrapServers is a
// comma-separated STRING, not an array. Fixtures do not use provider wire types.
var iamBackendFixtures = map[string]string{
	"mysql":           "useRdsIam: true\nuseIrsa: false\nauthMode: aws_rds_iam\nawsRegion: us-east-2\nawsRoleArn: role\nawsAccessKeyId: synthetic-id\nawsSecretAccessKey: synthetic-secret\nawsServiceAccount: identity\n",
	"postgres":        "useRdsIam: true\nuseIrsa: false\nauthMode: aws_rds_iam\nawsRegion: us-east-2\nawsRoleArn: role\nawsAccessKeyId: synthetic-id\nawsSecretAccessKey: synthetic-secret\nawsServiceAccount: identity\n",
	"kafka":           "useMskIam: true\nuseIrsa: false\nbootstrapServers: broker-a:9098,broker-b:9098\nawsRegion: us-east-2\nawsRoleArn: role\nawsAccessKeyId: synthetic-id\nawsSecretAccessKey: synthetic-secret\nawsServiceAccount: identity\n",
	"awselasticcache": "useIamAuth: true\nuseIrsa: false\naws_role_arn: role\naws_service_account: identity\ncache_name: cache\ncache_type: serverless\n",
}

func parityData(t *testing.T, typ string, values map[string]interface{}) *schema.ResourceData {
	t.Helper()
	values["type"], values["name"] = typ, "fixture"
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, values)
	raw := map[string]cty.Value{}
	for k, v := range values {
		switch x := v.(type) {
		case string:
			raw[k] = cty.StringVal(x)
		case bool:
			raw[k] = cty.BoolVal(x)
		}
	}
	d.SetId("fixture-id")
	state := d.State()
	state.RawConfig = cty.ObjectVal(raw)
	return ResourceAdaptiveResource().Data(state)
}
func parityWire(t *testing.T, d *schema.ResourceData, typ string) map[string]interface{} {
	t.Helper()
	obj, err := schemaToResourceIntegrationConfiguration(d, typ)
	if err != nil {
		t.Fatal(err)
	}
	b, err := yaml.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]interface{}
	if err = yaml.UnmarshalStrict(b, &wire); err != nil {
		t.Fatal(err)
	}
	return wire
}
func TestIntegrationIAMBackendContract(t *testing.T) {
	for typ, text := range iamBackendFixtures {
		t.Run(typ, func(t *testing.T) {
			var fixture map[string]interface{}
			if err := yaml.UnmarshalStrict([]byte(text), &fixture); err != nil {
				t.Fatal(err)
			}
			values := map[string]interface{}{}
			// Explicit independent public-to-wire map, rather than the production mapper.
			names := map[string]string{"useRdsIam": "use_rds_iam", "useMskIam": "use_msk_iam", "useIamAuth": "use_iam_auth", "useIrsa": "use_irsa", "authMode": "auth_mode", "awsRegion": "aws_region", "awsRoleArn": "aws_role_arn", "awsAccessKeyId": "aws_access_key_id", "awsSecretAccessKey": "aws_secret_access_key", "awsServiceAccount": "aws_service_account", "bootstrapServers": "bootstrap_servers"}
			for k, v := range fixture {
				a := names[k]
				if a == "" {
					a = k
				}
				values[a] = v
			}
			d := parityData(t, typ, values)
			wire := parityWire(t, d, typ)
			for k, v := range fixture {
				if !reflect.DeepEqual(wire[k], v) {
					t.Errorf("wire field %s missing or incorrect (values withheld)", k)
				}
			}
			refreshed := parityData(t, typ, map[string]interface{}{})
			if err := integrations.ApplyIntegrationConfig(refreshed, typ, fixture); err != nil {
				t.Fatal(err)
			}
			for k, v := range fixture {
				a := names[k]
				if a == "" {
					a = k
				}
				if !reflect.DeepEqual(refreshed.Get(a), v) {
					t.Errorf("reverse field %s missing or incorrect", a)
				}
			}
		})
	}
}
func TestIntegrationIAMInactiveIdentityPreservation(t *testing.T) {
	for _, typ := range []string{"mysql", "postgres", "kafka"} {
		t.Run(typ, func(t *testing.T) {
			attr := "use_rds_iam"
			if typ == "kafka" {
				attr = "use_msk_iam"
			}
			wire := parityWire(t, parityData(t, typ, map[string]interface{}{attr: false, "aws_region": "us-east-2", "aws_role_arn": "role", "aws_service_account": "identity", "aws_access_key_id": "synthetic-id", "aws_secret_access_key": "synthetic-secret"}), typ)
			for _, k := range []string{"awsRegion", "awsRoleArn", "awsServiceAccount"} {
				if wire[k] == nil {
					t.Errorf("inactive mode discarded persisted nonsecret identity %s", k)
				}
			}
			for _, k := range []string{"awsAccessKeyId", "awsSecretAccessKey"} {
				if wire[k] != nil {
					t.Errorf("inactive mode unexpectedly emitted conditional credential %s", k)
				}
			}
		})
	}
}

func TestIntegrationIAMReversePresence(t *testing.T) {
	names := map[string]string{"useRdsIam": "use_rds_iam", "useMskIam": "use_msk_iam", "useIamAuth": "use_iam_auth", "useIrsa": "use_irsa", "authMode": "auth_mode", "awsRegion": "aws_region", "awsRoleArn": "aws_role_arn", "awsAccessKeyId": "aws_access_key_id", "awsSecretAccessKey": "aws_secret_access_key", "awsServiceAccount": "aws_service_account", "bootstrapServers": "bootstrap_servers", "cache_name": "cache_name", "cache_type": "cache_type", "aws_role_arn": "aws_role_arn", "aws_service_account": "aws_service_account"}
	for typ, text := range iamBackendFixtures {
		t.Run(typ, func(t *testing.T) { testParityFields(t, parityFieldFixture{typ, text, names}) })
	}
}

func TestIntegrationIAMSwitchPresence(t *testing.T) {
	for _, typ := range []string{"mysql", "postgres", "kafka", "awselasticcache"} {
		attr, key := "use_rds_iam", "useRdsIam"
		if typ == "kafka" {
			attr, key = "use_msk_iam", "useMskIam"
		}
		if typ == "awselasticcache" {
			attr, key = "use_iam_auth", "useIamAuth"
		}
		for _, mode := range []string{"absent", "false", "true", "legacy"} {
			t.Run(typ+"/"+mode, func(t *testing.T) {
				values := map[string]interface{}{}
				if mode == "false" || mode == "true" {
					values[attr] = mode == "true"
					values["use_irsa"] = false
				}
				if mode == "legacy" && (typ == "mysql" || typ == "postgres") {
					values["auth_mode"] = "aws_rds_iam"
				}
				wire := parityWire(t, parityData(t, typ, values), typ)
				got, present := wire[key]
				wantPresent := mode == "false" || mode == "true"
				if present != wantPresent || (present && got != (mode == "true")) {
					t.Error("IAM switch lost nil versus false semantics")
				}
				_, irsa := wire["useIrsa"]
				if irsa != wantPresent {
					t.Error("IRSA omission was flattened or explicit false lost")
				}
			})
		}
	}
}
