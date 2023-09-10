/*
 * Copyright 2019 Travis Ralston <travis@t2bot.io>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
