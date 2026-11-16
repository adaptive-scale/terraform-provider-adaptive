package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// optionalBool preserves backend pointer semantics. The raw plan/configuration
// establishes presence only; the value ALWAYS comes from the planned data so
// ignore_changes is never bypassed. Refreshed true/false ownership is retained
// by the scoped omission helper. A false with no ownership evidence is nil.
func optionalBool(d *schema.ResourceData, attr string) *bool {
	raw := d.GetRawConfig()
	if raw.IsKnown() && !raw.IsNull() && raw.Type().IsObjectType() {
		if raw.Type().HasAttribute(attr) && !raw.GetAttr(attr).IsNull() {
			v := attrBool(d, attr)
			return &v
		}
	}
	if v, ok := d.GetOkExists(attr); ok {
		b, ok := v.(bool)
		if ok {
			return &b
		}
	}
	return nil
}

// IAMIdentity is the common camel-case backend RDS/MSK identity vocabulary.
// Callers decide activation; inactive credentials are intentionally not emitted,
// so the serialized held-secret guard rejects unsafe full replacements.
type IAMIdentity struct {
	UseIRSA            *bool  `yaml:"useIrsa,omitempty"`
	AWSRegion          string `yaml:"awsRegion,omitempty"`
	AWSRoleARN         string `yaml:"awsRoleArn,omitempty"`
	AWSAccessKeyID     string `yaml:"awsAccessKeyId,omitempty"`
	AWSSecretAccessKey string `yaml:"awsSecretAccessKey,omitempty"`
	AWSServiceAccount  string `yaml:"awsServiceAccount,omitempty"`
}

func iamIdentity(d *schema.ResourceData, active bool) IAMIdentity {
	cfg := IAMIdentity{
		UseIRSA:           optionalBool(d, "use_irsa"),
		AWSRegion:         attrString(d, "aws_region"),
		AWSRoleARN:        attrString(d, "aws_role_arn"),
		AWSServiceAccount: attrString(d, "aws_service_account"),
	}
	if !active {
		return cfg
	}
	cfg.AWSAccessKeyID = attrString(d, "aws_access_key_id")
	cfg.AWSSecretAccessKey = attrString(d, "aws_secret_access_key")
	return cfg
}
func (a *configApplier) applyIAMIdentity() {
	a.boolean("useIrsa", "use_irsa")
	for _, p := range [][2]string{{"awsRegion", "aws_region"}, {"awsRoleArn", "aws_role_arn"}, {"awsAccessKeyId", "aws_access_key_id"}, {"awsSecretAccessKey", "aws_secret_access_key"}, {"awsServiceAccount", "aws_service_account"}} {
		a.cloudString(p[0], p[1])
	}
}
