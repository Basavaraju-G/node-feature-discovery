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
	"testing"

	"github.com/stretchr/testify/assert"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/source"
)

func TestSingletonCryptoSource(t *testing.T) {
	assert.Equal(t, src.Name(), Name)

	// Check that GetLabels works with empty features
	src.features = nil
	l, err := src.GetLabels()

	assert.Nil(t, err)
	assert.Empty(t, l)
}

func TestCryptoSourcePriority(t *testing.T) {
	assert.Equal(t, 0, src.Priority())
}

func TestGetFeatures(t *testing.T) {
	// Test with nil features
	testSrc := cryptoSource{}
	features := testSrc.GetFeatures()
	assert.NotNil(t, features)
	assert.Empty(t, features.Flags)
	assert.Empty(t, features.Attributes)
	assert.Empty(t, features.Instances)

	// Test with existing features
	testSrc.features = nfdv1alpha1.NewFeatures()
	testSrc.features.Instances[CexCardFeature] = nfdv1alpha1.InstanceFeatureSet{
		Elements: []nfdv1alpha1.InstanceFeature{
			{
				Attributes: map[string]string{
					"name": "card00",
					"type": "CEX8C",
				},
			},
		},
	}
	features = testSrc.GetFeatures()
	assert.NotNil(t, features)
	assert.Len(t, features.Instances[CexCardFeature].Elements, 1)
}

func TestGetLabelsWithVariousCardTypes(t *testing.T) {
	tests := []struct {
		name           string
		cards          []nfdv1alpha1.InstanceFeature
		expectedLabels source.FeatureLabels
	}{
		{
			name:           "no cards",
			cards:          []nfdv1alpha1.InstanceFeature{},
			expectedLabels: source.FeatureLabels{},
		},
		{
			name: "single card type",
			cards: []nfdv1alpha1.InstanceFeature{
				{
					Attributes: map[string]string{
						"name": "card00",
						"type": "CEX8C",
						"mode": "cca",
					},
				},
			},
			expectedLabels: source.FeatureLabels{
				"cex.present":    true,
				"cex.count":      "1",
				"cex.type-CEX8C": true,
				"cex.mode-cca":   true,
			},
		},
		{
			name: "multiple cards same type",
			cards: []nfdv1alpha1.InstanceFeature{
				{
					Attributes: map[string]string{
						"name": "card00",
						"type": "CEX8C",
						"mode": "cca",
					},
				},
				{
					Attributes: map[string]string{
						"name": "card01",
						"type": "CEX8C",
						"mode": "cca",
					},
				},
			},
			expectedLabels: source.FeatureLabels{
				"cex.present":    true,
				"cex.count":      "2",
				"cex.type-CEX8C": true,
				"cex.mode-cca":   true,
			},
		},
		{
			name: "all three modes",
			cards: []nfdv1alpha1.InstanceFeature{
				{
					Attributes: map[string]string{
						"name": "card00",
						"type": "CEX8C",
						"mode": "cca",
					},
				},
				{
					Attributes: map[string]string{
						"name": "card01",
						"type": "CEX8A",
						"mode": "accelerator",
					},
				},
				{
					Attributes: map[string]string{
						"name": "card02",
						"type": "CEX7P",
						"mode": "ep11",
					},
				},
			},
			expectedLabels: source.FeatureLabels{
				"cex.present":          true,
				"cex.count":            "3",
				"cex.type-CEX8C":       true,
				"cex.type-CEX8A":       true,
				"cex.type-CEX7P":       true,
				"cex.mode-cca":         true,
				"cex.mode-accelerator": true,
				"cex.mode-ep11":        true,
			},
		},
		{
			name: "card without type or mode attribute",
			cards: []nfdv1alpha1.InstanceFeature{
				{
					Attributes: map[string]string{
						"name": "card00",
					},
				},
			},
			expectedLabels: source.FeatureLabels{
				"cex.present": true,
				"cex.count":   "1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testSrc := cryptoSource{}
			testSrc.features = nfdv1alpha1.NewFeatures()

			if len(tc.cards) > 0 {
				testSrc.features.Instances[CexCardFeature] = nfdv1alpha1.InstanceFeatureSet{
					Elements: tc.cards,
				}
			}

			labels, err := testSrc.GetLabels()
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedLabels, labels)
		})
	}
}
