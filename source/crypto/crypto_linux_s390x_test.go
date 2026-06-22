//go:build linux && s390x

/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crypto

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
	"sigs.k8s.io/node-feature-discovery/source"
)

func TestCryptoSourceS390x(t *testing.T) {
	expectedFeatures := map[string]*nfdv1alpha1.Features{
		"no-ap-bus": {
			Flags:      map[string]nfdv1alpha1.FlagFeatureSet{},
			Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{},
			Instances:  map[string]nfdv1alpha1.InstanceFeatureSet{},
		},
		"single-card": {
			Flags:      map[string]nfdv1alpha1.FlagFeatureSet{},
			Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{},
			Instances: map[string]nfdv1alpha1.InstanceFeatureSet{
				"cex-card": {
					Elements: []nfdv1alpha1.InstanceFeature{
						{
							Attributes: map[string]string{
								"name":         "card00",
								"type":         "CEX8C",
								"mode":         "cca",
								"online":       "1",
								"hwtype":       "14",
								"depth":        "7",
								"ap_functions": "0x04000000",
								"config":       "1",
								"queue_count":  "1",
								"queues":       "00.0014",
							},
						},
					},
				},
			},
		},
		"multiple-cards": {
			Flags:      map[string]nfdv1alpha1.FlagFeatureSet{},
			Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{},
			Instances: map[string]nfdv1alpha1.InstanceFeatureSet{
				"cex-card": {
					Elements: []nfdv1alpha1.InstanceFeature{
						{
							Attributes: map[string]string{
								"name":         "card00",
								"type":         "CEX8C",
								"mode":         "cca",
								"online":       "1",
								"hwtype":       "14",
								"depth":        "7",
								"ap_functions": "0x04000000",
								"config":       "1",
								"queue_count":  "2",
								"queues":       "00.0014,00.0015",
							},
						},
						{
							Attributes: map[string]string{
								"name":         "card01",
								"type":         "CEX8A",
								"mode":         "accelerator",
								"online":       "1",
								"hwtype":       "14",
								"depth":        "7",
								"ap_functions": "0x04000000",
								"config":       "1",
								"queue_count":  "1",
								"queues":       "01.0016",
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		rootfs         string
		expectedLabels source.FeatureLabels
	}{
		{
			name:           "no AP bus present",
			rootfs:         "no-ap-bus",
			expectedLabels: source.FeatureLabels{},
		},
		{
			name:   "single CEX card",
			rootfs: "single-card",
			expectedLabels: source.FeatureLabels{
				"cex.present":    true,
				"cex.count":      "1",
				"cex.type-CEX8C": true,
				"cex.mode-cca":   true,
			},
		},
		{
			name:   "multiple CEX cards with different types",
			rootfs: "multiple-cards",
			expectedLabels: source.FeatureLabels{
				"cex.present":          true,
				"cex.count":            "2",
				"cex.type-CEX8C":       true,
				"cex.type-CEX8A":       true,
				"cex.mode-cca":         true,
				"cex.mode-accelerator": true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSysfsPath := filepath.Join("..", "..", "testdata", "source", "crypto", tc.rootfs, "sys")
			hostpath.SysfsDir = hostpath.HostDir(mockSysfsPath)

			testSrc := cryptoSource{}

			err := testSrc.Discover()
			assert.Nil(t, err)

			f := testSrc.GetFeatures()
			assert.Equal(t, expectedFeatures[tc.rootfs], f)

			l, err := testSrc.GetLabels()
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedLabels, l)
		})
	}
}

func TestModeFromType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CEX8C", "cca"},
		{"CEX8A", "accelerator"},
		{"CEX8P", "ep11"},
		{"CEX7C", "cca"},
		{"CEX5A", "accelerator"},
		{"CEX4P", "ep11"},
		{"", ""},
		{"UNKNOWN", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, modeFromType(tc.input))
		})
	}
}
