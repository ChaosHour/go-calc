package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func parseTier(tier string) (int, int, error) {
	re := regexp.MustCompile(`db-custom-(\d+)-(\d+)`)
	matches := re.FindStringSubmatch(tier)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("invalid tier format")
	}
	cpu, err1 := strconv.Atoi(matches[1])
	ram, err2 := strconv.Atoi(matches[2])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("invalid tier numbers")
	}
	return cpu, ram, nil
}

func parseMem(memStr string) (float64, error) {
	if memStr == "" {
		return 0, fmt.Errorf("empty memory string")
	}
	var value float64
	var unit string
	n, err := fmt.Sscanf(memStr, "%f%s", &value, &unit)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid memory format")
	}
	switch unit {
	case "G", "g":
		return value * 1024, nil
	case "M", "m", "":
		return value, nil
	default:
		return 0, fmt.Errorf("invalid unit: %s", unit)
	}
}

func validateTier(cpu, ram int) bool {
	// vCPUs must be 1 or an even number between 2 and 96
	if cpu < 1 || cpu > 96 {
		return false
	}
	if cpu != 1 && cpu%2 != 0 {
		return false
	}
	// Memory must be a multiple of 256 MB and at least 3840 MB
	if ram%256 != 0 || ram < 3840 {
		return false
	}
	// Memory must be 0.9 to 6.5 GB per vCPU
	minRam := int(0.9 * float64(cpu) * 1024)
	maxRam := int(6.5 * float64(cpu) * 1024)
	return ram >= minRam && ram <= maxRam
}

func suggestNextTier(_ int, ram int) (int, int) {
	cpusNeeded := float64(ram) / 1.5 / 1024
	cpusNext := int(math.Ceil(cpusNeeded))
	ramNext := int(float64(cpusNext) * 1.5 * 1024)
	// Ensure multiple of 256
	ramNext = ((ramNext + 255) / 256) * 256
	if ramNext < 3840 {
		ramNext = 3840
	}
	return cpusNext, ramNext
}

var knownTiers = []struct {
	cpu int
	ram int
}{
	{1, 3840},
	{2, 7680},
	{2, 13312},
	{4, 15360},
	{4, 26624},
	{6, 23040},
	{6, 39936},
	{8, 30720},
	{8, 53248},
	{10, 38400},
	{10, 66560},
	{12, 46080},
	{12, 79872},
	{16, 61440},
	{16, 106496},
	{24, 92160},
	{24, 159744},
	{32, 122880},
	{32, 212992},
	{48, 184320},
	{48, 319488},
	{64, 245760},
	{64, 425984},
	{80, 307200},
	{80, 532480},
	{96, 368640},
	{96, 638976},
}

func findNextKnownTier(cpu int, ram int) (int, int, bool) {
	for _, t := range knownTiers {
		if t.cpu > cpu || (t.cpu == cpu && t.ram > ram) {
			return t.cpu, t.ram, true
		}
	}
	return 0, 0, false
}

func findPreviousKnownTier(cpu int, ram int) (int, int, bool) {
	for i := len(knownTiers) - 1; i >= 0; i-- {
		t := knownTiers[i]
		if t.cpu < cpu || (t.cpu == cpu && t.ram < ram) {
			return t.cpu, t.ram, true
		}
	}
	return 0, 0, false
}

func nearestValidTier(cpu, ram int) (int, int) {
	// Fix vCPU: must be 1 or even 2-96
	if cpu < 1 {
		cpu = 1
	} else if cpu > 96 {
		cpu = 96
	} else if cpu != 1 && cpu%2 != 0 {
		cpu = cpu + 1
	}
	// Round RAM up to nearest multiple of 256
	ram = ((ram + 255) / 256) * 256
	if ram < 3840 {
		ram = 3840
	}
	// Clamp to valid range for this CPU count
	minRAM := ((int(0.9*float64(cpu)*1024) + 255) / 256) * 256
	maxRAM := (int(6.5*float64(cpu)*1024) / 256) * 256
	if ram < minRAM {
		ram = minRAM
	}
	if ram > maxRAM {
		ram = maxRAM
	}
	return cpu, ram
}

func main() {
	cpu := flag.Float64("cpu", 0, "Number of vCPUs (e.g., 24, 48, 64)")
	mem := flag.String("mem", "", "Memory (e.g., 6G, 6144M, 6144)")
	tier := flag.String("t", "", "CloudSQL custom tier string (e.g., db-custom-1-3840)")
	bumpMem := flag.String("bump-mem", "", "Bump memory for existing tier (e.g., db-custom-4-3840)")
	checkDowngrade := flag.String("check-downgrade", "", "Check if recommended tier is a valid downgrade from current (format: 'current recommended')")
	downgrade := flag.String("downgrade", "", "Suggest the next valid downgrade tier from current (e.g., db-custom-8-53248)")
	flag.Parse()

	if *bumpMem != "" {
		c, r, err := parseTier(*bumpMem)
		if err != nil {
			fmt.Println("Invalid tier format. Use: db-custom-<cpus>-<ram_mb>")
			os.Exit(1)
		}
		// Keep CPUs, calculate max RAM at 6.5 GB/vCPU
		ramMB := float64(c) * 6.5 * 1024
		ramMB = float64((int(ramMB) / 256) * 256) // round down to stay within 6.5 GB/vCPU
		if ramMB < 3840 {
			ramMB = 3840
		}
		newTier := fmt.Sprintf("db-custom-%d-%d", c, int(ramMB))
		if int(ramMB) == r {
			fmt.Printf("Tier %s is already at the maximum memory level of %.2f GB (6.5 GB/vCPU).\n", *bumpMem, ramMB/1024)
		} else if int(ramMB) < r {
			fmt.Printf("Tier %s already exceeds the maximum standard memory.\n", *bumpMem)
			fmt.Printf("  Current: %d vCPUs, %d MB (%.2f GB) [%.2f GB/vCPU]\n", c, r, float64(r)/1024, float64(r)/1024/float64(c))
			fmt.Printf("  Max at 6.5 GB/vCPU: %d vCPUs, %.0f MB (%.2f GB)\n", c, ramMB, ramMB/1024)
		} else {
			fmt.Printf("Bumping memory for tier %s:\n", *bumpMem)
			fmt.Printf("  Current: %d vCPUs, %d MB (%.2f GB) [%.2f GB/vCPU]\n", c, r, float64(r)/1024, float64(r)/1024/float64(c))
			fmt.Printf("  New: %d vCPUs, %.0f MB (%.2f GB) [%.2f GB/vCPU]\n", c, ramMB, ramMB/1024, ramMB/1024/float64(c))
			fmt.Printf("  New Tier: %s\n", newTier)
		}
		return
	}

	if *checkDowngrade != "" {
		parts := strings.Split(*checkDowngrade, " ")
		if len(parts) != 2 {
			fmt.Println("Usage: -check-downgrade '<current-tier> <recommended-tier>'")
			os.Exit(1)
		}
		currCPU, currRAM, err1 := parseTier(parts[0])
		recCPU, recRAM, err2 := parseTier(parts[1])
		if err1 != nil || err2 != nil {
			fmt.Println("Invalid tier format. Use: db-custom-<cpus>-<ram_mb>")
			os.Exit(1)
		}
		isValidCurr := validateTier(currCPU, currRAM)
		isValidRec := validateTier(recCPU, recRAM)
		isLower := (recCPU < currCPU) || (recCPU == currCPU && recRAM < currRAM)

		fmt.Printf("Checking downgrade from %s to %s:\n", parts[0], parts[1])
		fmt.Printf("  Current: %d vCPUs, %d MB (%.2f GB) - Valid: %t\n", currCPU, currRAM, float64(currRAM)/1024, isValidCurr)
		fmt.Printf("  Recommended: %d vCPUs, %d MB (%.2f GB) - Valid: %t\n", recCPU, recRAM, float64(recRAM)/1024, isValidRec)

		if isValidRec && isLower {
			fmt.Println("  Valid downgrade: Yes")
		} else {
			fmt.Println("  Valid downgrade: No")
			if !isValidRec {
				adjCPU, adjRAM := nearestValidTier(recCPU, recRAM)
				adjLower := (adjCPU < currCPU) || (adjCPU == currCPU && adjRAM < currRAM)
				fmt.Printf("  Nearest valid tier: db-custom-%d-%d (%d vCPUs, %d MB, %.2f GB)\n",
					adjCPU, adjRAM, adjCPU, adjRAM, float64(adjRAM)/1024)
				if adjLower {
					fmt.Println("  This adjusted tier is a valid downgrade.")
				}
			}
			if !isLower {
				fmt.Println("  Recommended tier is not lower than the current tier.")
			}
			if nextCPU, nextRAM, found := findPreviousKnownTier(currCPU, currRAM); found {
				fmt.Printf("  Suggested known lower tier: db-custom-%d-%d (%d vCPUs, %d MB, %.2f GB)\n",
					nextCPU, nextRAM, nextCPU, nextRAM, float64(nextRAM)/1024)
			} else {
				fmt.Println("  No lower tier found in known list.")
			}
		}
		return
	}

	if *downgrade != "" {
		currCPU, currRAM, err := parseTier(*downgrade)
		if err != nil {
			fmt.Println("Invalid tier format. Use: db-custom-<cpus>-<ram_mb>")
			os.Exit(1)
		}
		isValidCurr := validateTier(currCPU, currRAM)
		fmt.Printf("Current tier: %s\n", *downgrade)
		fmt.Printf("  CPUs: %d, RAM: %d MB (%.2f GB) - Valid: %t\n", currCPU, currRAM, float64(currRAM)/1024, isValidCurr)
		fmt.Printf("  Memory per vCPU: %.2f GB (valid range: 0.9-6.5 GB)\n", float64(currRAM)/1024/float64(currCPU))

		if nextCPU, nextRAM, found := findPreviousKnownTier(currCPU, currRAM); found {
			fmt.Printf("Suggested downgrade tier: db-custom-%d-%d\n", nextCPU, nextRAM)
			fmt.Printf("  CPUs: %d, RAM: %d MB (%.2f GB)\n", nextCPU, nextRAM, float64(nextRAM)/1024)
			fmt.Printf("  Memory per vCPU: %.2f GB\n", float64(nextRAM)/1024/float64(nextCPU))
		} else {
			fmt.Println("Already at the lowest known tier.")
		}
		return
	}

	if *tier != "" {
		c, r, err := parseTier(*tier)
		if err != nil {
			fmt.Println("Invalid tier format. Use: db-custom-<cpus>-<ram_mb>")
			os.Exit(1)
		}
		fmt.Printf("Parsed tier: CPUs=%d, RAM=%d MB\n", c, r)
		if nextCPU, nextRAM, found := findNextKnownTier(c, r); found {
			fmt.Printf("Next known working custom tier: db-custom-%d-%d\n", nextCPU, nextRAM)
			fmt.Printf("  CPUs: %d\n  RAM: %d MB (%.2f GB)\n", nextCPU, nextRAM, float64(nextRAM)/1024)
		} else {
			cpusNext, ramNext := suggestNextTier(c, r)
			if cpusNext == c && ramNext == r {
				fmt.Println("This is already a valid custom tier.")
			} else {
				fmt.Printf("Next valid custom tier: db-custom-%d-%d\n", cpusNext, ramNext)
				fmt.Printf("  CPUs: %d\n  RAM: %d MB (%.2f GB)\n", cpusNext, ramNext, float64(ramNext)/1024)
			}
		}
		return
	}

	if (*cpu == 0 && *mem == "") || (*cpu != 0 && *mem != "") {
		fmt.Println("Usage: go-calc -cpu <vCPUs> OR -mem <memory> OR -t <tier> OR -bump-mem <tier> OR -check-downgrade '<current> <recommended>' OR -downgrade <current>")
		fmt.Println("  -mem examples: 6G, 6144M, 6144")
		fmt.Println("  -bump-mem: Increase memory to standard level for the given tier")
		fmt.Println("  -check-downgrade: Validate if recommended is a valid downgrade from current")
		fmt.Println("  -downgrade: Suggest the next valid downgrade tier from current")
		os.Exit(1)
	}

	if *cpu > 0 {
		ramMB := *cpu * 1.5 * 1024
		ramMB = float64(((int(ramMB) + 255) / 256) * 256)
		if ramMB < 3840 {
			ramMB = 3840
		}
		tier := fmt.Sprintf("db-custom-%d-%d", int(*cpu), int(ramMB))
		fmt.Printf("Recommended CloudSQL MySQL tier for %.0f vCPUs:\n", *cpu)
		fmt.Printf("  - Memory: %.0f MB (%.2f GB)\n", ramMB, ramMB/1024)
		fmt.Printf("  - Tier: %s\n", tier)
		fmt.Printf("  - Memory per vCPU: %.2f GB (valid range: 0.9-6.5 GB)\n", ramMB/1024 / *cpu)
	} else {
		memMB, err := parseMem(*mem)
		if err != nil {
			fmt.Println("Invalid mem format:", err)
			os.Exit(1)
		}
		memMB = float64(((int(memMB) + 255) / 256) * 256)
		if memMB < 3840 {
			memMB = 3840
		}
		cpus := memMB / 1.5 / 1024
		cpusRounded := math.Round(cpus)
		if cpusRounded < 1 {
			cpusRounded = 1
		}
		tier := fmt.Sprintf("db-custom-%d-%d", int(cpusRounded), int(memMB))
		if !validateTier(int(cpusRounded), int(memMB)) {
			fmt.Println("Warning: The calculated tier may not be valid. Please check the constraints.")
		}
		fmt.Printf("Recommended CloudSQL MySQL tier for %.0f MB RAM:\n", memMB)
		fmt.Printf("  - vCPUs: %.0f\n", cpusRounded)
		fmt.Printf("  - Memory: %.0f MB (%.2f GB)\n", memMB, memMB/1024)
		fmt.Printf("  - Tier: %s\n", tier)
		fmt.Printf("  - Memory per vCPU: %.2f GB (valid range: 0.9-6.5 GB)\n", memMB/1024/cpusRounded)
	}
}
