package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInputAliasesAreExplicitAndQuestionScoped(t *testing.T) {
	a, err := LoadInputAliases("../../translations/input-aliases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ input, expected, want string }{
		{"鑰匙", "KEYS", "KEYS"}, {"钥匙", "KEYS", "KEYS"},
		{"德魯伊", "DRUIDS", "DRUIDS"}, {"德鲁伊", "DRUIDS", "DRUIDS"},
		{"米努", "MEENU", "MEENU"}, {"四十六", "46", "46"},
		{"二十三", "23", "23"}, {"六十四", "64", "64"}, {"三十二", "32", "32"},
		{"４６", "46", "46"}, {"ＫＥＹＳ", "KEYS", "KEYS"},
		{"钥匙", "DRUIDS", "钥匙"}, {"宝剑", "KEYS", "宝剑"},
		{" keys ", "KEYS", "keys"},
	} {
		if got := a.Resolve(tc.input, tc.expected); got != tc.want {
			t.Errorf("%q for %s: %q", tc.input, tc.expected, got)
		}
	}
	if a.Label("KEYS", Simplified) != "钥匙" {
		t.Fatal("simplified answer label")
	}
	for name, blob := range map[string]string{
		"duplicate": "canonical\tlocale\ttext\nKEYS\tzh-Hant\t鑰匙\nKEYS\tzh-Hant\t鑰匙\n",
		"missing":   "canonical\tlocale\ttext\nKEYS\tzh-Hans\t钥匙\n",
		"bad key":   "canonical\tlocale\ttext\nkeys\tzh-Hans\t钥匙\n",
		"bad UTF8":  "canonical\tlocale\ttext\nKEYS\tzh-Hans\t\xff\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "aliases.tsv")
			os.WriteFile(p, []byte(blob), 0600)
			if _, err := LoadInputAliases(p); err == nil {
				t.Fatal("invalid aliases accepted")
			}
		})
	}
}
