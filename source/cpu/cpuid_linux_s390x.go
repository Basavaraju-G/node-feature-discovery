/*
Copyright 2019 The Kubernetes Authors.

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

/*
#include <sys/auxv.h>

unsigned long gethwcap() {
	return getauxval(AT_HWCAP);
}
*/
import "C"

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

/*
all special features for s390x should be defined here; canonical list:
https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/arch/s390/include/asm/elf.h
http://sourceware.org/git/?p=glibc.git;a=blob;f=sysdeps/unix/sysv/linux/s390/bits/hwcap.h;hb=HEAD
*/
const (
	/* AT_HWCAP features */
	HWCAP_S390_ESAN3     = 1
	HWCAP_S390_ZARCH     = 2
	HWCAP_S390_STFLE     = 4
	HWCAP_S390_MSA       = 8
	HWCAP_S390_LDISP     = 16
	HWCAP_S390_EIMM      = 32
	HWCAP_S390_DFP       = 64
	HWCAP_S390_HPAGE     = 128
	HWCAP_S390_ETF3EH    = 256
	HWCAP_S390_HIGH_GPRS = 512
	HWCAP_S390_TE        = 1024
	HWCAP_S390_VX        = 2048
	HWCAP_S390_VXD       = 4096
	HWCAP_S390_VXE       = 8192
	HWCAP_S390_GS        = 16384
	HWCAP_S390_VXRS_EXT2 = 32768
	HWCAP_S390_VXRS_PDE  = 65536
	HWCAP_S390_SORT      = 131072
	HWCAP_S390_DFLT      = 262144
	HWCAP_S390_VXRS_PDE2 = 524288
	HWCAP_S390_NNPA      = 1048576
	HWCAP_S390_PCI_MIO   = 2097152
	HWCAP_S390_SIE       = 4194304
)

var flagNames_s390x = map[uint64]string{
	HWCAP_S390_ESAN3:     "ESAN3",
	HWCAP_S390_ZARCH:     "ZARCH",
	HWCAP_S390_STFLE:     "STFLE",
	HWCAP_S390_MSA:       "MSA",
	HWCAP_S390_LDISP:     "LDISP",
	HWCAP_S390_EIMM:      "EIMM",
	HWCAP_S390_DFP:       "DFP",
	HWCAP_S390_HPAGE:     "EDAT",
	HWCAP_S390_ETF3EH:    "ETF3EH",
	HWCAP_S390_HIGH_GPRS: "HIGHGPRS",
	HWCAP_S390_TE:        "TE",
	HWCAP_S390_VX:        "VX",
	HWCAP_S390_VXD:       "VXD",
	HWCAP_S390_VXE:       "VXE",
	HWCAP_S390_GS:        "GS",
	HWCAP_S390_VXRS_EXT2: "VXE2",
	HWCAP_S390_VXRS_PDE:  "VXP",
	HWCAP_S390_SORT:      "SORT",
	HWCAP_S390_DFLT:      "DFLT",
	HWCAP_S390_VXRS_PDE2: "VXP2",
	HWCAP_S390_NNPA:      "NNPA",
	HWCAP_S390_PCI_MIO:   "PCIMIO",
	HWCAP_S390_SIE:       "SIE",
}

// facilityNames maps STFLE facility bit numbers to human-readable names.
// Source: IBM z/Architecture Principles of Operation (SA22-7832-14)
// and Linux kernel arch/s390/tools/gen_facilities.c
var facilityNames = map[int]string{
	0:   "N3",
	1:   "ZARCH_INSTALLED",
	2:   "ZARCH_ACTIVE",
	3:   "DAT_ENH_1",
	4:   "IDTE_SEGMENT",
	6:   "AP",
	7:   "STFLE",
	8:   "EDAT_1",
	9:   "SENSE_RUNNING",
	10:  "CONDITIONAL_SSKE",
	11:  "CONFIG_TOPOLOGY",
	12:  "AP_QUERY_CONFIG_INFO",
	13:  "IPTE_RANGE",
	14:  "NONQ_KEY_SETTING",
	15:  "AP_FACILITIES_TEST",
	16:  "EXTENDED_TRANSLATION_2",
	17:  "MSA",
	18:  "LONG_DISPLACEMENT",
	19:  "LONG_DISPLACEMENT_PERF",
	20:  "HFP_MADDSUB",
	21:  "EXTENDED_IMMEDIATE",
	22:  "EXTENDED_TRANSLATION_3",
	23:  "HFP_UNNORM_EXT",
	24:  "ETF2_ENH",
	25:  "STORE_CLOCK_FAST",
	26:  "PARSING_ENH",
	27:  "MVCOS",
	28:  "TOD_CLOCK_STEERING",
	30:  "ETF3_ENH",
	31:  "EXTRACT_CPU_TIME",
	32:  "COMPARE_SWAP_STORE",
	33:  "COMPARE_SWAP_STORE_2",
	34:  "GENERAL_INSTR_EXT",
	35:  "EXECUTE_EXT",
	36:  "ENHANCED_MONITOR",
	37:  "FP_EXT",
	38:  "ORDER_PRESERVING_COMPRESSION",
	40:  "LOAD_PROGRAM_PARAMS",
	41:  "FP_SUPPORT_ENH",
	42:  "DFP",
	43:  "DFP_PERF",
	44:  "PFPO",
	45:  "DISTINCT_OPS",
	46:  "FAST_BCR_SERIAL",
	47:  "CMPSC_ENH",
	48:  "DFP_ZONED_CONV",
	49:  "MISC_INSTR_EXT_1",
	50:  "CONSTRAINED_TX",
	51:  "LOCAL_TLB_CLEARING",
	52:  "INTERLOCKED_ACCESS_2",
	53:  "MISC_INSTR_EXT_2",
	54:  "CMPSC_ENH_2",
	55:  "VECTOR_BCD_ENH_1",
	57:  "MSA_EXT_5",
	58:  "MISC_INSTR_EXT_3",
	59:  "SEMAPHORE_ASSIST",
	60:  "RI",
	61:  "MISC_INSTR_EXT_3B",
	69:  "PROC_ACTIVITY_INSTR",
	70:  "CPU_MEASUREMENT_COUNTER",
	71:  "CPU_MEASUREMENT_SAMPLING",
	72:  "CPU_MEASUREMENT_COUNTER_EXT",
	73:  "TX",
	74:  "STORE_HYPERVISOR_INFO",
	75:  "ACCESS_EXCEPTION_FS",
	76:  "MSA_EXT_3",
	77:  "MSA_EXT_4",
	78:  "EDAT_2",
	80:  "DFP_PACKED_CONV",
	81:  "PPA_IN_ORDER",
	82:  "SPECTRE_MITIGATION",
	84:  "MISC_INSTR_EXT_4",
	129: "VECTOR",
	130: "INSTR_EXEC_PROT",
	131: "ENHANCED_SOP_2",
	133: "GUARDED_STORAGE",
	134: "VECTOR_PACKED_DECIMAL",
	135: "VECTOR_ENH_1",
	139: "MULTIPLE_EPOCH",
	146: "MSA_EXT_8",
	147: "VECTOR_BCD_ENH_2",
	148: "VECTOR_ENH_2",
	150: "EDAT_3",
	151: "DEFLATE_CONV_ENH",
	152: "VECTOR_PACKED_DEC_ENH_2",
	155: "MSA_EXT_9",
	156: "ETOKEN",
	165: "NNPA",
	168: "ESA_COMPAT_MODE",
	192: "VECTOR_PACKED_DEC_ENH_3",
	193: "BEAR_ENH",
	194: "RDP_ENH",
	196: "PROC_ACTIVITY_INSTR_EXT_1",
	197: "PROC_ACTIVITY_INSTR_EXT_2",
	198: "VECTOR_ENH_3",
	199: "VECTOR_PACKED_DEC_ENH_4",
	201: "CONCURRENT_FUNCTIONS",
}

func getCpuidFlags() []string {
	r := make([]string, 0, 20)
	hwcap := uint64(C.gethwcap())
	for i := uint(0); i < 64; i++ {
		key := uint64(1 << i)
		val, ok := flagNames_s390x[key]
		if hwcap&key != 0 && ok {
			r = append(r, val)
		}
	}
	return r
}

func getCpuidAttributes() map[string]string {
	attrs := make(map[string]string)

	facilities := parseFacilities()
	for _, bit := range facilities {
		if name, ok := facilityNames[bit]; ok {
			attrs["facility_"+name] = "true"
		}
	}

	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

// parseFacilities reads the STFLE facility bit list from /proc/cpuinfo.
func parseFacilities() []int {
	path := hostpath.ProcDir.Path("cpuinfo")
	f, err := os.Open(path)
	if err != nil {
		klog.ErrorS(err, "failed to open cpuinfo")
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "facilities") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(parts[1]))
		bits := make([]int, 0, len(fields))
		for _, field := range fields {
			bit, err := strconv.Atoi(field)
			if err != nil {
				continue
			}
			bits = append(bits, bit)
		}
		return bits
	}
	if err := scanner.Err(); err != nil {
		klog.ErrorS(err, "error reading cpuinfo")
	}
	return nil
}
