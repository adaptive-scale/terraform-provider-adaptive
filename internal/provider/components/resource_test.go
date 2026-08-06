package components

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v2"
)

func TestSchemaToResourceIntegrationConfiguration_TargetsOnlyForAdaptiveRDP(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name":     "win-1",
		"type":     "rdp_windows",
		"hostname": "10.0.1.10",
		"username": "administrator",
		"password": "secret",
		"targets": []interface{}{
			map[string]interface{}{"id": "t1", "host": "10.0.1.11", "username": "admin", "password": "pw"},
		},
	})

	_, err := schemaToResourceIntegrationConfiguration(d, "rdp_windows")
	if err == nil {
		t.Fatal("expected an error when `targets` is set on a non-adaptive_rdp resource")
	}
	if !strings.Contains(err.Error(), "adaptive_rdp") {
		t.Fatalf("expected the error to mention adaptive_rdp, got: %v", err)
	}
}

func TestSchemaToResourceIntegrationConfiguration_RDPWindowsEmptyPassword(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name":     "win-1",
		"type":     "rdp_windows",
		"hostname": "10.0.1.10",
		"username": "administrator",
	})

	_, err := schemaToResourceIntegrationConfiguration(d, "rdp_windows")
	if err == nil {
		t.Fatal("expected an error for an empty rdp_windows password")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Fatalf("expected a password error, got: %v", err)
	}
}

func TestSchemaToResourceIntegrationConfiguration_RDSIAMOnlyForPostgres(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name":        "my-1",
		"type":        "mysql",
		"host":        "db.example.com",
		"username":    "root",
		"password":    "secret",
		"use_rds_iam": true,
	})

	_, err := schemaToResourceIntegrationConfiguration(d, "mysql")
	if err == nil {
		t.Fatal("expected an error when `use_rds_iam` is set on a non-postgres resource")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("expected the error to mention postgres, got: %v", err)
	}
}

func TestSchemaToResourceIntegrationConfiguration_PostgresPasswordModeOmitsRDSIAM(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name":     "pg-1",
		"type":     "postgres",
		"host":     "db.example.com",
		"port":     "5432",
		"username": "postgres",
		"password": "secret",
		"ssl_mode": "require",
	})

	obj, err := schemaToResourceIntegrationConfiguration(d, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"useRdsIam", "useIrsa", "awsRegion", "awsRoleArn", "awsServiceAccount"} {
		if strings.Contains(string(out), key) {
			t.Errorf("password-mode postgres should not emit %q:\n%s", key, out)
		}
	}
}

func TestSchemaToResourceIntegrationConfiguration_PostgresRDSIAM(t *testing.T) {
	tests := []struct {
		name        string
		raw         map[string]interface{}
		wantIRSA    string
		wantPresent []string
	}{
		{
			name: "service account implies irsa",
			raw: map[string]interface{}{
				"aws_service_account": "adaptive-rds-access",
			},
			wantIRSA:    "useIrsa: true",
			wantPresent: []string{"awsServiceAccount: adaptive-rds-access"},
		},
		{
			name: "static keys imply no irsa",
			raw: map[string]interface{}{
				"aws_access_key_id":     "AKID",
				"aws_secret_access_key": "secret",
			},
			wantIRSA:    "useIrsa: false",
			wantPresent: []string{"awsAccessKeyId: AKID"},
		},
		{
			name: "explicit irsa with a role arn",
			raw: map[string]interface{}{
				"use_irsa":     true,
				"aws_role_arn": "arn:aws:iam::123456789012:role/AdaptiveRDSAccess",
				"aws_region":   "us-east-1",
			},
			wantIRSA: "useIrsa: true",
			wantPresent: []string{
				"awsRoleArn: arn:aws:iam::123456789012:role/AdaptiveRDSAccess",
				"awsRegion: us-east-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]interface{}{
				"name":        "pg-iam",
				"type":        "postgres",
				"host":        "mydb.abc123.us-east-1.rds.amazonaws.com",
				"port":        "5432",
				"username":    "iam_user",
				"ssl_mode":    "require",
				"use_rds_iam": true,
			}
			for k, v := range tt.raw {
				raw[k] = v
			}

			d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, raw)
			obj, err := schemaToResourceIntegrationConfiguration(d, "postgres")
			if err != nil {
				t.Fatal(err)
			}
			out, err := yaml.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(string(out), "useRdsIam: true") {
				t.Errorf("expected useRdsIam: true in:\n%s", out)
			}
			if !strings.Contains(string(out), tt.wantIRSA) {
				t.Errorf("expected %q in:\n%s", tt.wantIRSA, out)
			}
			for _, want := range tt.wantPresent {
				if !strings.Contains(string(out), want) {
					t.Errorf("expected %q in:\n%s", want, out)
				}
			}
		})
	}
}

func TestSchemaToResourceIntegrationConfiguration_AdaptiveRDPTargetsAccepted(t *testing.T) {
	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, map[string]interface{}{
		"name": "fleet-1",
		"type": "adaptive_rdp",
		"targets": []interface{}{
			map[string]interface{}{"id": "t1", "host": "10.0.1.11", "username": "admin", "password": "pw"},
		},
	})

	if _, err := schemaToResourceIntegrationConfiguration(d, "adaptive_rdp"); err != nil {
		t.Fatalf("unexpected error for adaptive_rdp with targets: %v", err)
	}
}
