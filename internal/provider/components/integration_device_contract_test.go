package components

import "testing"

var parityDeviceTypes = []string{"aruba_instant_on", "aruba_sw", "cisco_ngfw", "fortinet_ngfw", "fortinet_analyzer", "fortinet_manager", "hpe_switch", "ivanti", "juniper_sw", "paloalto_ngfw", "sophos_fw"}

func TestIntegrationDeviceBackendContract(t *testing.T) {
	for _, typ := range parityDeviceTypes {
		t.Run(typ, func(t *testing.T) {
			loginAttr := "login_url"
			if typ == "cisco_ngfw" || typ == "fortinet_ngfw" || typ == "hpe_switch" {
				loginAttr = "uri"
			}
			testParityFields(t, parityFieldFixture{typ, "automationMode: script\nfields: synthetic-form\nscript: synthetic-script\nsshKey: synthetic-key\nport: '2222'\nwebui_port: '8443'\nlogin_url: /login\n", map[string]string{"automationMode": "automation_mode", "fields": "fields", "script": "script", "sshKey": "key", "port": "port", "webui_port": "webui_port", "login_url": loginAttr}})
		})
	}
}
