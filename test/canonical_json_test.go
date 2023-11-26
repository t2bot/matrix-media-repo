package test

import (
	"testing"

	"github.com/turt2live/matrix-media-repo/util"
)

func TestEncodeCanonicalJson_CaseA(t *testing.T) {
	input := map[string]interface{}{}
	expectedOutput := []byte("{}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func TestEncodeCanonicalJson_CaseB(t *testing.T) {
	input := map[string]interface{}{
		"one": 1,
		"two": "Two",
	}
	expectedOutput := []byte("{\"one\":1,\"two\":\"Two\"}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func TestEncodeCanonicalJson_CaseC(t *testing.T) {
	input := map[string]interface{}{
		"b": "2",
		"a": "1",
	}
	expectedOutput := []byte("{\"a\":\"1\",\"b\":\"2\"}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func TestEncodeCanonicalJson_CaseD(t *testing.T) {
	input := map[string]interface{}{
		"auth": map[string]interface{}{
			"success": true,
			"mxid":    "@john.doe:example.com",
			"profile": map[string]interface{}{
				"display_name": "John Doe",
				"three_pids": []map[string]interface{}{
					{
						"medium":  "email",
						"address": "john.doe@example.org",
					},
					{
						"medium":  "msisdn",
						"address": "123456789",
					},
				},
			},
		},
	}
	expectedOutput := []byte("{\"auth\":{\"mxid\":\"@john.doe:example.com\",\"profile\":{\"display_name\":\"John Doe\",\"three_pids\":[{\"address\":\"john.doe@example.org\",\"medium\":\"email\"},{\"address\":\"123456789\",\"medium\":\"msisdn\"}]},\"success\":true}}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func TestEncodeCanonicalJson_CaseE(t *testing.T) {
	input := map[string]interface{}{
		"a": "日本語",
	}
	expectedOutput := []byte("{\"a\":\"日本語\"}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func TestEncodeCanonicalJson_CaseF(t *testing.T) {
	input := map[string]interface{}{
		"本": 2,
		"日": 1,
	}
	expectedOutput := []byte("{\"日\":1,\"本\":2}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func TestEncodeCanonicalJson_CaseG(t *testing.T) {
	input := map[string]interface{}{
		"a": "\u65E5",
	}
	expectedOutput := []byte("{\"a\":\"日\"}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func TestEncodeCanonicalJson_CaseH(t *testing.T) {
	input := map[string]interface{}{
		"a": nil,
	}
	expectedOutput := []byte("{\"a\":null}")
	actualOutput, _ := util.EncodeCanonicalJson(input)
	compareBytes(expectedOutput, actualOutput, t)
}

func compareBytes(expected []byte, actual []byte, t *testing.T) {
	if len(expected) != len(actual) {
		t.Errorf("Mismatched length: %d != %d", len(actual), len(expected))
		t.Fail()
		return
	}

	for i := range expected {
		e := expected[i]
		a := actual[i]
		if e != a {
			t.Errorf("Expected %b but got %b at index %d", e, a, i)
			t.Fail()
			return
		}
	}
}
