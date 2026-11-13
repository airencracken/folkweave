// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func appCSS(t *testing.T) string {
	t.Helper()
	data, err := assets.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestStylesheetUsesTokensAndOneRulePerLine(t *testing.T) {
	css := appCSS(t)
	for _, gold := range []string{"#ac986a", "#b39c61", "#b9a36e"} {
		if strings.Count(css, gold) > 1 || (gold != "#b39c61" && !strings.Contains(css, "--gold: light-dark(")) {
			t.Fatal("hardcoded gold outside the token", gold)
		}
	}
	if strings.Contains(css, "#b39c61") {
		t.Fatal("focus gold should use --gold")
	}
	sizes := map[int]bool{}
	for _, m := range regexp.MustCompile(`--text-[a-z0-9]+: (\d+)px`).FindAllStringSubmatch(css, -1) {
		n, _ := strconv.Atoi(m[1])
		sizes[n] = true
		if n < 13 {
			t.Fatal("font size below 13px", n)
		}
	}
	if len(sizes) < 4 || len(sizes) > 6 {
		t.Fatal("type scale should have about five steps", sizes)
	}
	if regexp.MustCompile(`font-size:\s*\d`).MatchString(css) || regexp.MustCompile(`font:[^;]*\b\d+px`).MatchString(css) {
		t.Fatal("font sizes must come from the --text scale")
	}
	seen := map[string]int{}
	depth := 0
	for number, line := range strings.Split(css, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "/*") || strings.HasSuffix(trimmed, "*/") {
			continue
		}
		if strings.HasPrefix(trimmed, "@media") {
			depth++
			continue
		}
		if trimmed == "}" {
			depth--
			continue
		}
		if strings.Count(trimmed, "{") != 1 || !strings.HasSuffix(trimmed, "}") {
			t.Fatalf("line %d is not one rule: %s", number+1, trimmed)
		}
		if depth == 0 {
			selector := strings.TrimSpace(trimmed[:strings.Index(trimmed, "{")])
			if previous, ok := seen[selector]; ok {
				t.Fatalf("selector %q on line %d repeats line %d; fold overrides into one rule", selector, number+1, previous)
			}
			seen[selector] = number + 1
		}
	}
	for _, required := range []string{":root[data-theme=\"dark\"]", ":root[data-theme=\"light\"]", "--comfy-radius", "--comfy-ink", "--serif", "min-height: 44px"} {
		if !strings.Contains(css, required) {
			t.Fatal("missing", required)
		}
	}
}
