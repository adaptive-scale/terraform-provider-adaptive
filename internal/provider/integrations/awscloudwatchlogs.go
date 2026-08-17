package integrations

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type AWSCloudWatchLogsIntegrationConfiguration struct {
	Version       string `yaml:"version"`
	Name          string `yaml:"name"`
	AWSRegion     string `yaml:"aws_region"`
	ARN           string `yaml:"arn"`
	LogGroupName  string `yaml:"log_group_name"`
	LogStreamName string `yaml:"log_stream_name"`
}

func SchemaToAWSCloudWatchLogsIntegrationConfiguration(d *schema.ResourceData) AWSCloudWatchLogsIntegrationConfiguration {
	return AWSCloudWatchLogsIntegrationConfiguration{
		Version:       "1.0",
		Name:          attrString(d, "name"),
		AWSRegion:     attrString(d, "aws_region_name"),
		ARN:           attrString(d, "arn"),
		LogGroupName:  attrString(d, "log_group_name"),
		LogStreamName: attrString(d, "log_stream_name"),
	}
}
