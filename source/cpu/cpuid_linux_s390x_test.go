//go:build linux && s390x

/*
Copyright 2025 The Kubernetes Authors.

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

package cpu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

func TestParseFacilities(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []int
	}{
		{
			name: "z16 facilities",
			content: `vendor_id       : IBM/S390
# processors    : 8
bogomips per cpu: 26315.00
max thread id   : 0
features	: esan3 zarch stfle msa ldisp eimm dfp edat etf3eh highgprs te vx vxd vxe gs vxe2 vxp sort dflt vxp2 nnpa sie 
facilities      : 0 1 2 3 4 6 7 8 9 10 34 45 129 134 135 148 165 192 193
processor 0: version = FF,  identification = 155D38,  machine = 3931
`,
			expected: []int{0, 1, 2, 3, 4, 6, 7, 8, 9, 10, 34, 45, 129, 134, 135, 148, 165, 192, 193},
		},
		{
			name: "minimal facilities",
			content: `vendor_id       : IBM/S390
facilities      : 0 1 2 17 18 21
`,
			expected: []int{0, 1, 2, 17, 18, 21},
		},
		{
			name:     "no facilities line",
			content:  "vendor_id       : IBM/S390\n",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			procDir := filepath.Join(dir, "proc")
			err := os.MkdirAll(procDir, 0755)
			assert.NoError(t, err)
			err = os.WriteFile(filepath.Join(procDir, "cpuinfo"), []byte(tt.content), 0644)
			assert.NoError(t, err)

			origProcDir := hostpath.ProcDir
			hostpath.ProcDir = hostpath.HostDir(procDir)
			defer func() { hostpath.ProcDir = origProcDir }()

			result := parseFacilities()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCpuidAttributesFacilities(t *testing.T) {
	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")
	err := os.MkdirAll(procDir, 0755)
	assert.NoError(t, err)

	content := `vendor_id       : IBM/S390
facilities      : 0 1 17 34 45 73 129 134 135 148 165 198 199 201
`
	err = os.WriteFile(filepath.Join(procDir, "cpuinfo"), []byte(content), 0644)
	assert.NoError(t, err)

	origProcDir := hostpath.ProcDir
	hostpath.ProcDir = hostpath.HostDir(procDir)
	defer func() { hostpath.ProcDir = origProcDir }()

	attrs := getCpuidAttributes()
	assert.NotNil(t, attrs)

	expectedAttrs := map[string]string{
		"facility_N3":                       "true",
		"facility_ZARCH_INSTALLED":          "true",
		"facility_MSA":                      "true",
		"facility_GENERAL_INSTR_EXT":        "true",
		"facility_DISTINCT_OPS":             "true",
		"facility_TX":                       "true",
		"facility_VECTOR":                   "true",
		"facility_VECTOR_PACKED_DECIMAL":    "true",
		"facility_VECTOR_ENH_1":             "true",
		"facility_VECTOR_ENH_2":             "true",
		"facility_NNPA":                     "true",
		"facility_VECTOR_ENH_3":             "true",
		"facility_VECTOR_PACKED_DEC_ENH_4":  "true",
		"facility_CONCURRENT_FUNCTIONS":     "true",
	}
	assert.Equal(t, expectedAttrs, attrs)
}

func TestFacilityNamesCompleteness(t *testing.T) {
	// Verify key z17 facility bits are mapped
	z17Bits := []int{84, 198, 199, 201}
	for _, bit := range z17Bits {
		_, ok := facilityNames[bit]
		assert.True(t, ok, "z17 facility bit %d should be mapped", bit)
	}

	// Verify key z16 facility bits are mapped
	z16Bits := []int{148, 152, 155, 165}
	for _, bit := range z16Bits {
		_, ok := facilityNames[bit]
		assert.True(t, ok, "z16 facility bit %d should be mapped", bit)
	}

	// Verify key z15 facility bits are mapped
	z15Bits := []int{61, 135, 146}
	for _, bit := range z15Bits {
		_, ok := facilityNames[bit]
		assert.True(t, ok, "z15 facility bit %d should be mapped", bit)
	}
}
