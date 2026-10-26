package device

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/filipowm/go-unifi/unifi"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToPortOverrideAggregateTranslation(t *testing.T) {
	t.Run("CountBecomesContiguousMemberRange", func(t *testing.T) {
		po, err := toPortOverride(map[string]interface{}{
			"number":              5,
			"name":                "lag uplink",
			"port_profile_id":     "",
			"op_mode":             "aggregate",
			"poe_mode":            "",
			"aggregate_num_ports": 3,
		})
		require.NoError(t, err)
		assert.Equal(t, []int{5, 6, 7}, po.AggregateMembers)
	})

	t.Run("UnsetLeavesMembersNil", func(t *testing.T) {
		po, err := toPortOverride(map[string]interface{}{
			"number":              1,
			"name":                "plain port",
			"port_profile_id":     "abc123",
			"op_mode":             "switch",
			"poe_mode":            "auto",
			"aggregate_num_ports": 0,
		})
		require.NoError(t, err)
		assert.Nil(t, po.AggregateMembers)
	})
}

func TestFromPortOverrideAggregateTranslation(t *testing.T) {
	t.Run("MemberListLengthBecomesCount", func(t *testing.T) {
		m := fromPortOverride(unifi.DevicePortOverrides{
			PortIDX:          5,
			OpMode:           "aggregate",
			AggregateMembers: []int{5, 6, 7},
		})
		assert.Equal(t, 3, m["aggregate_num_ports"])
	})

	t.Run("NilMembersReadBackAsZero", func(t *testing.T) {
		m := fromPortOverride(unifi.DevicePortOverrides{
			PortIDX: 1,
			OpMode:  "switch",
		})
		assert.Equal(t, 0, m["aggregate_num_ports"])
	})
}

func TestPortOverrideAggregateRoundTrip(t *testing.T) {
	in := map[string]interface{}{
		"number":              2,
		"name":                "lag",
		"port_profile_id":     "",
		"op_mode":             "aggregate",
		"poe_mode":            "",
		"aggregate_num_ports": 4,
	}
	po, err := toPortOverride(in)
	require.NoError(t, err)
	out := fromPortOverride(po)
	assert.Equal(t, in["aggregate_num_ports"], out["aggregate_num_ports"])
}

func stringSet(items ...string) *schema.Set {
	raw := make([]interface{}, len(items))
	for i, s := range items {
		raw[i] = s
	}
	return schema.NewSet(schema.HashString, raw)
}

// portOverrideData builds a complete port_override map (all schema keys present,
// as SDKv2 supplies them) so toPortOverride's type assertions don't panic.
func portOverrideData(overrides map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"number":                    1,
		"name":                      "",
		"port_profile_id":           "",
		"op_mode":                   "switch",
		"poe_mode":                  "",
		"aggregate_num_ports":       0,
		"native_networkconf_id":     "",
		"tagged_vlan_mgmt":          "",
		"forward":                   "",
		"excluded_network_ids":      stringSet(),
		"voice_networkconf_id":      "",
		"setting_preference":        "",
		"port_security_enabled":     false,
		"port_security_mac_address": stringSet(),
	}
	for k, v := range overrides {
		data[k] = v
	}
	return data
}

func TestToPortOverride_VLANFields(t *testing.T) {
	po, err := toPortOverride(portOverrideData(map[string]interface{}{
		"number":                10,
		"native_networkconf_id": "net-native",
		"tagged_vlan_mgmt":      "custom",
		"forward":               "customize",
		"excluded_network_ids":  stringSet("net-a", "net-b"),
		"voice_networkconf_id":  "net-voice",
		"setting_preference":    "manual",
	}))
	require.NoError(t, err)
	assert.Equal(t, "net-native", po.NATiveNetworkID)
	assert.Equal(t, "custom", po.TaggedVLANMgmt)
	assert.Equal(t, "customize", po.Forward)
	assert.Equal(t, "net-voice", po.VoiceNetworkID)
	assert.Equal(t, "manual", po.SettingPreference)
	// TypeSet -> unordered slice: assert membership, not order.
	assert.ElementsMatch(t, []string{"net-a", "net-b"}, po.ExcludedNetworkIDs)
}

// forward has NO default: a block that sets only poe_mode must leave Forward
// empty (so omitempty drops it from the PUT and never black-holes a trunk port).
func TestToPortOverride_ForwardNoDefault(t *testing.T) {
	po, err := toPortOverride(portOverrideData(map[string]interface{}{
		"number":   3,
		"poe_mode": "auto",
	}))
	require.NoError(t, err)
	assert.Equal(t, "", po.Forward)
	assert.Equal(t, "", po.NATiveNetworkID)
	assert.Equal(t, "", po.TaggedVLANMgmt)
	assert.Equal(t, "", po.SettingPreference)
	assert.Empty(t, po.ExcludedNetworkIDs)
}

func TestFromPortOverride_VLANFields(t *testing.T) {
	m := fromPortOverride(unifi.DevicePortOverrides{
		PortIDX:            10,
		NATiveNetworkID:    "net-native",
		TaggedVLANMgmt:     "custom",
		Forward:            "customize",
		ExcludedNetworkIDs: []string{"net-a", "net-b"},
		VoiceNetworkID:     "net-voice",
		SettingPreference:  "manual",
	})
	assert.Equal(t, "net-native", m["native_networkconf_id"])
	assert.Equal(t, "custom", m["tagged_vlan_mgmt"])
	assert.Equal(t, "customize", m["forward"])
	assert.Equal(t, "net-voice", m["voice_networkconf_id"])
	assert.Equal(t, "manual", m["setting_preference"])
	excluded, ok := m["excluded_network_ids"].(*schema.Set)
	require.True(t, ok)
	got := make([]string, 0, excluded.Len())
	for _, v := range excluded.List() {
		s, _ := v.(string)
		got = append(got, s)
	}
	assert.ElementsMatch(t, []string{"net-a", "net-b"}, got)
}

func TestPortOverride_VLANRoundTrip(t *testing.T) {
	in := portOverrideData(map[string]interface{}{
		"number":                7,
		"native_networkconf_id": "net-native",
		"tagged_vlan_mgmt":      "custom",
		"forward":               "customize",
		"excluded_network_ids":  stringSet("net-a", "net-b"),
		"voice_networkconf_id":  "net-voice",
		"setting_preference":    "manual",
	})
	po, err := toPortOverride(in)
	require.NoError(t, err)
	out := fromPortOverride(po)
	assert.Equal(t, in["native_networkconf_id"], out["native_networkconf_id"])
	assert.Equal(t, in["tagged_vlan_mgmt"], out["tagged_vlan_mgmt"])
	assert.Equal(t, in["forward"], out["forward"])
	assert.Equal(t, in["voice_networkconf_id"], out["voice_networkconf_id"])
	assert.Equal(t, in["setting_preference"], out["setting_preference"])
	inExcluded, _ := in["excluded_network_ids"].(*schema.Set)
	outExcluded, _ := out["excluded_network_ids"].(*schema.Set)
	assert.True(t, inExcluded.Equal(outExcluded))
}

// The StringInSlice validators are the only client-side guard (SDK validation is
// disabled in base/client.go), so verify they accept valid and reject invalid
// values for the constrained VLAN attributes.
func TestPortOverride_VLANValidators(t *testing.T) {
	elem, _ := ResourceDevice().Schema["port_override"].Elem.(*schema.Resource)

	cases := []struct {
		attr    string
		valid   []string
		invalid []string
	}{
		{"tagged_vlan_mgmt", []string{"auto", "block_all", "custom"}, []string{"bogus", "all"}},
		{"forward", []string{"all", "native", "customize", "disabled"}, []string{"bogus", "trunk"}},
		{"setting_preference", []string{"auto", "manual"}, []string{"bogus", "automatic"}},
	}
	for _, tc := range cases {
		t.Run(tc.attr, func(t *testing.T) {
			vf := elem.Schema[tc.attr].ValidateFunc
			require.NotNil(t, vf, "%s must have a ValidateFunc", tc.attr)
			for _, v := range tc.valid {
				_, errs := vf(v, tc.attr)
				assert.Empty(t, errs, "%q should be valid for %s", v, tc.attr)
			}
			for _, v := range tc.invalid {
				_, errs := vf(v, tc.attr)
				assert.NotEmpty(t, errs, "%q should be rejected for %s", v, tc.attr)
			}
		})
	}
}

// deviceState builds the stored state of a device holding the given overrides.
func deviceState(t *testing.T, overrides []interface{}) *terraform.InstanceState {
	t.Helper()
	d := ResourceDevice().TestResourceData()
	d.SetId("device")
	require.NoError(t, d.Set("mac", "aa:bb:cc:dd:ee:ff"))
	require.NoError(t, d.Set("port_override", overrides))
	return d.State()
}

func deviceConfig(overrides []interface{}) *terraform.ResourceConfig {
	return terraform.NewResourceConfigRaw(map[string]interface{}{
		"mac":           "aa:bb:cc:dd:ee:ff",
		"port_override": overrides,
	})
}

// plannedPortOverrides diffs state against config and returns the planned
// port_override set, i.e. what the provider would send on apply.
func plannedPortOverrides(t *testing.T, state *terraform.InstanceState, cfg *terraform.ResourceConfig) map[int]map[string]interface{} {
	t.Helper()
	r := ResourceDevice()
	diff, err := r.Diff(context.Background(), state, cfg, nil)
	require.NoError(t, err)
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	require.NoError(t, err)
	set, _ := d.Get("port_override").(*schema.Set)
	planned := map[int]map[string]interface{}{}
	for _, item := range set.List() {
		m, _ := item.(map[string]interface{})
		number, _ := m["number"].(int)
		planned[number] = m
	}
	return planned
}

// The SDK keeps the existing set element when a planned one hashes the same, so
// with a hash on `number` alone a changed field inside a declared port never
// reached the plan, and nothing was sent.
func TestPortOverride_ChangeInDeclaredPortIsPlanned(t *testing.T) {
	state := deviceState(t, []interface{}{map[string]interface{}{
		"number": 4, "forward": "native", "tagged_vlan_mgmt": "block_all", "setting_preference": "manual",
	}})
	cfg := deviceConfig([]interface{}{map[string]interface{}{
		"number": 4, "forward": "disabled", "tagged_vlan_mgmt": "block_all", "setting_preference": "manual",
	}})

	planned := plannedPortOverrides(t, state, cfg)
	require.Contains(t, planned, 4)
	assert.Equal(t, "disabled", planned[4]["forward"])
}

// portOverrideChanges lists the port_override attributes a plan would change.
func portOverrideChanges(t *testing.T, state *terraform.InstanceState, cfg *terraform.ResourceConfig) []string {
	t.Helper()
	diff, err := ResourceDevice().Diff(context.Background(), state, cfg, nil)
	require.NoError(t, err)
	var changes []string
	if diff == nil {
		return changes
	}
	for k, a := range diff.Attributes {
		if strings.HasPrefix(k, "port_override.") && (a.Old != a.New || a.NewComputed || a.NewRemoved) {
			changes = append(changes, fmt.Sprintf("%s: %q -> %q", k, a.Old, a.New))
		}
	}
	return changes
}

// Once applied, the same config must plan nothing again, even though config
// holds unset fields as nil while state reads them back as zero values, the SDK
// fills in op_mode's default only when hashing config, and the controller fills
// in values on fields a port doesn't declare.
func TestPortOverride_ReapplyPlansNothing(t *testing.T) {
	cfg := deviceConfig([]interface{}{
		map[string]interface{}{
			"number": 1, "name": "ap", "forward": "customize", "native_networkconf_id": "net-lan", "poe_mode": "auto",
			"setting_preference": "manual", "tagged_vlan_mgmt": "custom", "excluded_network_ids": []interface{}{"net-a", "net-b"},
		},
		map[string]interface{}{
			"number": 2, "poe_mode": "off", "setting_preference": "manual", "tagged_vlan_mgmt": "block_all",
		},
		map[string]interface{}{
			"number": 3, "name": "server", "port_profile_id": "profile-server", "setting_preference": "auto",
		},
	})
	controller := &unifi.Device{PortOverrides: []unifi.DevicePortOverrides{
		{
			PortIDX: 1, Name: "ap", Forward: "customize", NATiveNetworkID: "net-lan", PoeMode: "auto",
			SettingPreference: "manual", TaggedVLANMgmt: "custom", ExcludedNetworkIDs: []string{"net-b", "net-a"},
		},
		{PortIDX: 2, PoeMode: "off", SettingPreference: "manual", TaggedVLANMgmt: "block_all", NATiveNetworkID: "net-lan", Forward: "native"},
		{PortIDX: 3, Name: "server", PortProfileID: "profile-server", SettingPreference: "auto"},
	}}

	// apply runs a plan against state and reads the controller back the way
	// Update does, returning the state it leaves behind.
	r := ResourceDevice()
	apply := func(state *terraform.InstanceState) *terraform.InstanceState {
		diff, err := r.Diff(context.Background(), state, cfg, nil)
		require.NoError(t, err)
		d, err := schema.InternalMap(r.Schema).Data(state, diff)
		require.NoError(t, err)
		require.NoError(t, d.Set("port_override", portOverridesFromDevice(controller, d)))
		return d.State()
	}

	state := apply(deviceState(t, nil))
	assert.Empty(t, portOverrideChanges(t, state, cfg))
}

func TestPortOverridesFromDevice(t *testing.T) {
	d := ResourceDevice().TestResourceData()
	require.NoError(t, d.Set("port_override", []interface{}{
		map[string]interface{}{"number": 4, "forward": "disabled", "poe_mode": "off"},
	}))

	got := portOverridesFromDevice(&unifi.Device{
		PortOverrides: []unifi.DevicePortOverrides{
			{PortIDX: 4, Forward: "native", PoeMode: "off", NATiveNetworkID: "net-default", SettingPreference: "manual"},
			{PortIDX: 5, Forward: "all", NATiveNetworkID: "net-default"},
		},
	}, d)

	byNumber := map[int]map[string]interface{}{}
	for _, m := range got {
		number, _ := m["number"].(int)
		byNumber[number] = m
	}
	t.Run("DeclaredFieldsShowDrift", func(t *testing.T) {
		assert.Equal(t, "native", byNumber[4]["forward"])
		assert.Equal(t, "off", byNumber[4]["poe_mode"])
	})
	t.Run("UndeclaredFieldsStayEmpty", func(t *testing.T) {
		assert.Equal(t, "", byNumber[4]["native_networkconf_id"])
		assert.Equal(t, "", byNumber[4]["setting_preference"])
	})
	t.Run("UndeclaredPortsAreReadInFull", func(t *testing.T) {
		assert.Equal(t, "all", byNumber[5]["forward"])
		assert.Equal(t, "net-default", byNumber[5]["native_networkconf_id"])
	})
}

// The VLAN attributes must be Optional+Computed so a field the user leaves unset
// keeps the controller's value instead of planning a change to empty.
func TestPortOverrideVLANFields_AreOptionalComputed(t *testing.T) {
	elem, _ := ResourceDevice().Schema["port_override"].Elem.(*schema.Resource)
	for _, attr := range []string{
		"native_networkconf_id", "tagged_vlan_mgmt", "forward",
		"excluded_network_ids", "voice_networkconf_id", "setting_preference",
		"port_security_enabled", "port_security_mac_address",
	} {
		s := elem.Schema[attr]
		require.NotNil(t, s, "attribute %s must exist", attr)
		assert.True(t, s.Optional, "%s must be Optional", attr)
		assert.True(t, s.Computed, "%s must be Computed (to absorb controller echoes without churn)", attr)
		assert.Nil(t, s.Default, "%s must not have a Default", attr)
	}
}

// A port disabled the way the UniFi UI does it: forward = "disabled" with port
// security enabled and no allowed MAC addresses.
func TestToPortOverride_DisabledWithPortSecurity(t *testing.T) {
	po, err := toPortOverride(portOverrideData(map[string]interface{}{
		"number":                2,
		"forward":               "disabled",
		"port_security_enabled": true,
	}))
	require.NoError(t, err)
	assert.Equal(t, "disabled", po.Forward)
	assert.True(t, po.PortSecurityEnabled)
	assert.Empty(t, po.PortSecurityMACAddress)
}

func TestPortOverride_PortSecurityRoundTrip(t *testing.T) {
	in := portOverrideData(map[string]interface{}{
		"number":                    3,
		"port_security_enabled":     true,
		"port_security_mac_address": stringSet("aa:bb:cc:dd:ee:01", "aa:bb:cc:dd:ee:02"),
	})
	po, err := toPortOverride(in)
	require.NoError(t, err)
	assert.True(t, po.PortSecurityEnabled)
	// TypeSet -> unordered slice: assert membership, not order.
	assert.ElementsMatch(t, []string{"aa:bb:cc:dd:ee:01", "aa:bb:cc:dd:ee:02"}, po.PortSecurityMACAddress)

	out := fromPortOverride(po)
	assert.Equal(t, true, out["port_security_enabled"])
	inMACs, _ := in["port_security_mac_address"].(*schema.Set)
	outMACs, _ := out["port_security_mac_address"].(*schema.Set)
	assert.True(t, inMACs.Equal(outMACs))
}

func radioSet(items ...map[string]interface{}) *schema.Set {
	raw := make([]interface{}, len(items))
	for i, m := range items {
		raw[i] = m
	}
	return schema.NewSet(radioSetHash, raw)
}

func radioByBand(rs []unifi.DeviceRadioTable, band string) (unifi.DeviceRadioTable, bool) {
	for _, r := range rs {
		if r.Radio == band {
			return r, true
		}
	}
	return unifi.DeviceRadioTable{}, false
}

// The core safety property: declaring one band must not wipe the others.
func TestMergeRadios_PreservesUndeclaredBands(t *testing.T) {
	current := []unifi.DeviceRadioTable{
		{Radio: "ng", Channel: "1", Ht: 20, TxPowerMode: "auto"},
		{Radio: "na", Channel: "36", Ht: 80, TxPowerMode: "high"},
		{Radio: "6e", Channel: "37", Ht: 160, TxPowerMode: "auto"},
	}
	got := mergeRadios(current, radioSet(map[string]interface{}{
		"name": "ng", "tx_power_mode": "disabled",
	}))
	if len(got) != 3 {
		t.Fatalf("expected all 3 bands preserved, got %d: %+v", len(got), got)
	}
	ng, _ := radioByBand(got, "ng")
	if ng.TxPowerMode != "disabled" {
		t.Errorf("ng tx_power_mode = %q, want disabled", ng.TxPowerMode)
	}
	if ng.Channel != "1" || ng.Ht != 20 {
		t.Errorf("ng channel/ht clobbered: got channel=%q ht=%d, want 1/20", ng.Channel, ng.Ht)
	}
	if na, _ := radioByBand(got, "na"); na.Channel != "36" || na.Ht != 80 || na.TxPowerMode != "high" {
		t.Errorf("na band modified: %+v", na)
	}
	if sixE, _ := radioByBand(got, "6e"); sixE.Channel != "37" || sixE.Ht != 160 {
		t.Errorf("6e band modified: %+v", sixE)
	}
}

// Only non-zero declared fields overlay; unset fields keep the controller value.
func TestMergeRadios_OverlaysOnlyNonZero(t *testing.T) {
	current := []unifi.DeviceRadioTable{{Radio: "ng", Channel: "6", Ht: 40, TxPowerMode: "medium"}}
	got := mergeRadios(current, radioSet(map[string]interface{}{
		"name": "ng", "channel": "11",
	}))
	ng, _ := radioByBand(got, "ng")
	if ng.Channel != "11" {
		t.Errorf("channel = %q, want 11", ng.Channel)
	}
	if ng.Ht != 40 || ng.TxPowerMode != "medium" {
		t.Errorf("unset fields clobbered: ht=%d tx_power_mode=%q, want 40/medium", ng.Ht, ng.TxPowerMode)
	}
}

// A declared band missing from the device is appended.
func TestMergeRadios_AppendsMissingBand(t *testing.T) {
	current := []unifi.DeviceRadioTable{{Radio: "na", Channel: "36"}}
	got := mergeRadios(current, radioSet(map[string]interface{}{
		"name": "ng", "tx_power_mode": "disabled",
	}))
	if len(got) != 2 {
		t.Fatalf("expected 2 bands, got %d", len(got))
	}
	if ng, ok := radioByBand(got, "ng"); !ok || ng.TxPowerMode != "disabled" {
		t.Errorf("ng not appended correctly: %+v ok=%v", ng, ok)
	}
}

// min_rssi pairs with min_rssi_enabled only when a non-zero threshold is set.
func TestMergeRadios_MinRssiPairing(t *testing.T) {
	current := []unifi.DeviceRadioTable{{Radio: "na"}}
	got := mergeRadios(current, radioSet(map[string]interface{}{
		"name": "na", "min_rssi": -75, "min_rssi_enabled": true,
	}))
	na, _ := radioByBand(got, "na")
	if na.MinRssi != -75 || !na.MinRssiEnabled {
		t.Errorf("min_rssi pairing failed: %+v", na)
	}
}

// Etherlighting overlay: declared fields apply; unset fields keep current values.
func TestMergeEtherLighting_OverlaysOnlyNonZero(t *testing.T) {
	current := unifi.DeviceEtherLighting{Mode: "speed", Brightness: 80, Behavior: "steady", LedMode: "etherlighting"}
	got := mergeEtherLighting(current, map[string]interface{}{"mode": "network"})
	if got.Mode != "network" {
		t.Errorf("mode = %q, want network", got.Mode)
	}
	if got.Brightness != 80 || got.Behavior != "steady" || got.LedMode != "etherlighting" {
		t.Errorf("unset fields clobbered: %+v", got)
	}
}

func TestMergeEtherLighting_FromEmptyCurrent(t *testing.T) {
	got := mergeEtherLighting(unifi.DeviceEtherLighting{}, map[string]interface{}{"mode": "network", "brightness": 60})
	if got.Mode != "network" || got.Brightness != 60 || got.Behavior != "" {
		t.Errorf("unexpected merge from empty: %+v", got)
	}
}
